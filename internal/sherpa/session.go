package sherpa

import (
	"context"
	"errors"
	"sync"
	"time"

	"cue-go-remix/internal/textnorm"
)

// Metric contains timing/counts only, never audio or recognized words.
type Metric struct {
	Phase, Channel                            string
	AudioMS, ElapsedMS, PendingMS, Characters int64
	Frames, Peak, RMS, LastFrameMS            int64
}

type packet struct {
	channel string
	pcm     []int16
}
type channelState struct {
	handle      uintptr
	last        string
	samples     int64
	firstAudio  time.Time
	seenPartial bool
}

// Session shares one model between independent microphone/system streams.
// The worker alone accesses native handles. Capture only copies PCM into a
// bounded queue, so inference cannot block the other audio channel's callback.
type Session struct {
	ctx      context.Context
	cancel   context.CancelFunc
	decoder  decoder
	channels map[string]*channelState
	queue    chan packet
	done     chan struct{}
	mu       sync.Mutex
	queued   map[string]int
	closed   bool
	err      error
	onText   func(string, string, bool)
	onError  func(error)
	onMetric func(Metric)
	stopOnce sync.Once
}

func Start(ctx context.Context, system bool, onText func(string, string, bool), onError func(error), onMetric func(Metric)) (*Session, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return nil, err
	}
	return StartAt(ctx, dir, system, onText, onError, onMetric)
}

// StartAt permits verification against the exact bundled runtime and model.
func StartAt(ctx context.Context, dir string, system bool, onText func(string, string, bool), onError func(error), onMetric func(Metric)) (*Session, error) {
	started := time.Now()
	d, err := newDecoder(dir)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		d.Close()
		return nil, err
	}
	s, err := startWithDecoder(ctx, d, system, onText, onError, onMetric)
	if err == nil && onMetric != nil {
		onMetric(Metric{Phase: "model-ready", ElapsedMS: time.Since(started).Milliseconds()})
	}
	return s, err
}

func startWithDecoder(ctx context.Context, d decoder, system bool, onText func(string, string, bool), onError func(error), onMetric func(Metric)) (*Session, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{ctx: ctx, cancel: cancel, decoder: d, channels: map[string]*channelState{}, queue: make(chan packet, 128), done: make(chan struct{}), queued: map[string]int{}, onText: onText, onError: onError, onMetric: onMetric}
	names := []string{"you"}
	if system {
		names = append(names, "them")
	}
	for _, name := range names {
		h, err := d.NewStream()
		if err != nil {
			for _, c := range s.channels {
				d.DestroyStream(c.handle)
			}
			d.Close()
			cancel()
			return nil, err
		}
		s.channels[name] = &channelState{handle: h}
	}
	go s.run()
	return s, nil
}

func (s *Session) Send(channel string, pcm []int16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil {
		return errors.New("实时语音会话已停止")
	}
	if _, ok := s.channels[channel]; !ok {
		return errors.New("未知的实时语音通道")
	}
	if len(pcm) == 0 {
		return nil
	}
	if s.queued[channel]+len(pcm) > 48000 {
		return errors.New("实时识别积压超过 3 秒，请停止会话后重试")
	}
	// Preserve callback order, but bound each native decoding step to 100 ms.
	chunks := (len(pcm) + 1599) / 1600
	if cap(s.queue)-len(s.queue) < chunks {
		return errors.New("实时音频队列已满，请停止会话后重试")
	}
	for len(pcm) > 0 {
		n := min(1600, len(pcm))
		s.queue <- packet{channel, append([]int16(nil), pcm[:n]...)}
		s.queued[channel] += n
		pcm = pcm[n:]
	}
	return nil
}

func (s *Session) Stop() error {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		close(s.queue)
		s.mu.Unlock()
		// A healthy streaming decoder drains much faster than real time.
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-s.done:
		case <-timer.C:
			s.mu.Lock()
			if s.err == nil {
				s.err = errors.New("实时识别停止超时，未完成尾句已取消")
			}
			s.mu.Unlock()
			s.cancel()
			<-s.done
		}
		s.cancel()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Session) run() {
	defer close(s.done)
	defer s.decoder.Close()
	defer func() {
		for _, c := range s.channels {
			s.decoder.DestroyStream(c.handle)
		}
	}()
	for {
		select {
		case <-s.ctx.Done():
			return
		case p, ok := <-s.queue:
			if s.ctx.Err() != nil {
				return
			}
			if !ok {
				for _, name := range []string{"you", "them"} {
					c := s.channels[name]
					if c == nil || c.samples == 0 {
						continue
					}
					s.decoder.Accept(c.handle, make([]float32, 9600))
					s.decoder.Finish(c.handle)
					if err := s.decode(name, c); err != nil {
						s.fail(err)
						return
					}
					if c.last != "" {
						s.emit(name, c.last, true)
						s.metric(Metric{Phase: "final-stop", Channel: name, AudioMS: c.samples / 16, ElapsedMS: time.Since(c.firstAudio).Milliseconds(), Characters: int64(len([]rune(c.last)))})
					}
					s.emit(name, "", false)
				}
				return
			}
			s.mu.Lock()
			s.queued[p.channel] -= len(p.pcm)
			s.mu.Unlock()
			c := s.channels[p.channel]
			if c.firstAudio.IsZero() {
				c.firstAudio = time.Now()
			}
			c.samples += int64(len(p.pcm))
			pcm := make([]float32, len(p.pcm))
			for i, v := range p.pcm {
				pcm[i] = float32(v) / 32768
			}
			s.decoder.Accept(c.handle, pcm)
			if err := s.decode(p.channel, c); err != nil {
				s.fail(err)
				return
			}
		}
	}
}

func (s *Session) decode(channel string, c *channelState) error {
	for s.decoder.Ready(c.handle) {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		started := time.Now()
		s.decoder.Decode(c.handle)
		if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
			s.metric(Metric{Phase: "slow-decode", Channel: channel, ElapsedMS: elapsed.Milliseconds()})
		}
		text, err := s.decoder.Result(c.handle)
		if err != nil {
			return err
		}
		text = textnorm.Simplified(text)
		if s.decoder.Endpoint(c.handle) {
			if text != "" {
				s.emit(channel, text, true)
				s.metric(Metric{Phase: "final", Channel: channel, AudioMS: c.samples / 16, ElapsedMS: time.Since(c.firstAudio).Milliseconds(), Characters: int64(len([]rune(text)))})
			}
			s.emit(channel, "", false)
			s.decoder.Reset(c.handle)
			c.last = ""
			c.seenPartial = false
			c.firstAudio = time.Now()
			c.samples = 0
		} else if text != c.last {
			c.last = text
			s.emit(channel, text, false)
			if text != "" && !c.seenPartial {
				c.seenPartial = true
				s.metric(Metric{Phase: "first-partial", Channel: channel, AudioMS: c.samples / 16, ElapsedMS: time.Since(c.firstAudio).Milliseconds(), Characters: int64(len([]rune(text)))})
			}
		}
	}
	return nil
}

func (s *Session) emit(channel, text string, final bool) {
	if s.onText != nil {
		s.onText(channel, text, final)
	}
}
func (s *Session) metric(m Metric) {
	if s.onMetric == nil {
		return
	}
	s.mu.Lock()
	m.PendingMS = int64(s.queued[m.Channel] / 16)
	s.mu.Unlock()
	s.onMetric(m)
}
func (s *Session) fail(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
	if s.onError != nil && s.ctx.Err() == nil {
		s.onError(err)
	}
	s.cancel()
}

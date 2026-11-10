package listening

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"time"

	"cue-go-remix/internal/model"
	"cue-go-remix/internal/platform"
	"cue-go-remix/internal/provider"
	"cue-go-remix/internal/sherpa"
	"cue-go-remix/internal/textnorm"
	"cue-go-remix/internal/whisper"
)

type utterance struct {
	channel string
	pcm     []int16
}
type segment struct {
	pre, pcm, pending []int16
	silent, voiced    int
}
type audioStream interface {
	Send([]int16) error
	Close() error
}

// FatalError indicates that capture has stopped because the active service or
// native audio device failed. The application can observe Done and then Err to
// clear its listening state without stopping synchronously from a callback.
type FatalError struct{ Cause error }

func (e *FatalError) Error() string { return "听写已停止：" + e.Cause.Error() }
func (e *FatalError) Unwrap() error { return e.Cause }

type Session struct {
	mu          sync.Mutex
	segments    map[string]*segment
	queue       chan utterance
	workerDone  chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	stopCapture func()
	closed      bool
	stopping    bool
	failure     error
	streams     map[string]audioStream
	local       *whisper.Session
	realtime    *sherpa.Session
	input       *inputMonitor
	inputStop   chan struct{}
	inputDone   chan struct{}
	transcribe  func(context.Context, []int16) (string, error)
	settings    model.Settings
	onText      func(string, string, bool)
	onError     func(error)
	once        sync.Once
	failOnce    sync.Once
}

func newSession(parent context.Context, settings model.Settings, onText func(string, string, bool), onError func(error)) *Session {
	ctx, cancel := context.WithCancel(parent)
	return &Session{ctx: ctx, cancel: cancel, settings: settings, onText: onText, onError: onError, segments: map[string]*segment{"you": {}, "them": {}}, queue: make(chan utterance, 24), workerDone: make(chan struct{}), streams: map[string]audioStream{}}
}

func Start(parent context.Context, settings model.Settings, models *whisper.Manager, onText func(string, string, bool), onError func(error), metrics ...func(sherpa.Metric)) (*Session, error) {
	if settings.Speech.Provider != "local" && settings.Speech.Provider != "sherpa" && settings.Speech.Provider != "custom" && settings.Speech.APIKey == "" {
		return nil, errors.New("请先在声音与转写设置中填写 API Key")
	}
	s := newSession(parent, settings, onText, onError)
	if len(metrics) > 0 && metrics[0] != nil {
		s.input = newInputMonitor(settings.MeetingAudio, time.Now(), metrics[0])
	}
	if settings.Speech.Provider == "sherpa" {
		var metric func(sherpa.Metric)
		if len(metrics) > 0 {
			metric = metrics[0]
		}
		local, err := sherpa.Start(s.ctx, settings.MeetingAudio, s.text, s.fail, metric)
		if err != nil {
			s.cancel()
			return nil, err
		}
		s.realtime = local
	} else if settings.Speech.Provider == "local" {
		if err := models.Use(settings.WhisperModel); err != nil {
			s.cancel()
			return nil, err
		}
		if err := models.Verify(settings.WhisperModel); err != nil {
			models.Release()
			s.cancel()
			return nil, err
		}
		path, _ := models.Path(settings.WhisperModel)
		local, err := whisper.Start(s.ctx, settings, path)
		if err != nil {
			models.Release()
			s.cancel()
			return nil, err
		}
		s.local = local
		s.transcribe = local.Transcribe
	} else {
		s.transcribe = func(ctx context.Context, pcm []int16) (string, error) {
			return provider.Transcribe(ctx, settings.Speech, pcm, settings.Language)
		}
		if settings.Speech.Stream {
			channels := []string{"you"}
			if settings.MeetingAudio {
				channels = append(channels, "them")
			}
			for _, ch := range channels {
				stream, err := provider.OpenStream(s.ctx, settings.Speech, settings.Language, func(text string, final bool) { s.text(ch, text, final) }, s.fail)
				if err != nil {
					for _, v := range s.streams {
						_ = v.Close()
					}
					s.cancel()
					return nil, err
				}
				s.streams[ch] = stream
			}
		}
	}
	go s.worker()
	stop, err := platform.StartAudio(s.ctx, settings.MeetingAudio, s.feed, s.fail, func(state string) {
		if s.input != nil {
			s.input.emit(sherpa.Metric{Phase: state, Channel: "you"})
		}
	})
	s.mu.Lock()
	s.stopCapture = stop
	s.mu.Unlock()
	if err == nil && s.ctx.Err() != nil {
		err = s.Err()
		if err == nil {
			err = s.ctx.Err()
		}
	}
	if err != nil {
		s.Stop()
		if s.local != nil {
			models.Release()
		}
		return nil, err
	}
	s.startInputMonitor()
	return s, nil
}

// Done closes immediately after a fatal failure, or after normal Stop drains
// all accepted audio. Err distinguishes these two cases.
func (s *Session) Done() <-chan struct{} { return s.ctx.Done() }
func (s *Session) Err() error            { s.mu.Lock(); defer s.mu.Unlock(); return s.failure }

func (s *Session) report(err error) {
	if err != nil && s.onError != nil {
		s.onError(err)
	}
}
func (s *Session) text(channel, text string, final bool) {
	if s.settings.Speech.Provider != "sherpa" && (s.settings.Language == "zh" || strings.HasPrefix(s.settings.Language, "zh-")) {
		text = textnorm.Simplified(text)
	}
	if s.onText != nil {
		s.onText(channel, text, final)
	}
}
func (s *Session) fail(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	stopping := s.stopping
	s.mu.Unlock()
	if stopping || s.ctx.Err() != nil {
		return
	}
	s.failOnce.Do(func() {
		failure := &FatalError{Cause: err}
		s.mu.Lock()
		if s.stopping || s.ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		s.failure = failure
		s.mu.Unlock()
		s.cancel() // Also stops native capture, without waiting from its callback.
		s.report(failure)
	})
}

func (s *Session) feed(channel string, pcm []int16) {
	s.mu.Lock()
	if s.closed || s.ctx.Err() != nil || len(pcm) == 0 {
		s.mu.Unlock()
		return
	}
	if s.input != nil {
		s.input.observe(channel, pcm, time.Now())
	}
	if local := s.realtime; local != nil {
		s.mu.Unlock()
		if err := local.Send(channel, pcm); err != nil {
			s.fail(err)
		}
		return
	}
	if stream := s.streams[channel]; stream != nil {
		s.mu.Unlock() // A slow socket must not hold the other channel's VAD lock.
		if err := stream.Send(pcm); err != nil {
			s.fail(err)
		}
		return
	}
	v := s.segments[channel]
	if v == nil {
		s.mu.Unlock()
		return
	}
	v.pending = append(v.pending, pcm...)
	dropped := false
	// Keep frame boundaries stable even when device callbacks split samples at
	// arbitrary positions. Short final fragments are processed during Stop.
	for len(v.pending) >= 320 {
		if s.frameLocked(channel, v, v.pending[:320]) {
			dropped = true
		}
		v.pending = v.pending[320:]
	}
	if len(v.pending) == 0 {
		v.pending = nil
	} else {
		v.pending = append([]int16(nil), v.pending...)
	}
	s.mu.Unlock()
	if dropped {
		s.report(errors.New("转写速度跟不上录音，已跳过一段音频；请换用更快的模型"))
	}
}

// frameLocked and enqueueLocked require mu; they never invoke user callbacks.
func (s *Session) frameLocked(channel string, v *segment, frame []int16) bool {
	var sum float64
	for _, x := range frame {
		f := float64(x) / 32768
		sum += f * f
	}
	voice := math.Sqrt(sum/float64(len(frame))) > .009
	if len(v.pcm) == 0 {
		v.pre = append(v.pre, frame...)
		if len(v.pre) > 4800 {
			v.pre = v.pre[len(v.pre)-4800:]
		}
		if !voice {
			return false
		}
		v.pcm = append([]int16(nil), v.pre...)
		v.pre = nil
		v.voiced = len(frame)
		v.silent = 0
		return false
	}
	v.pcm = append(v.pcm, frame...)
	if voice {
		v.silent = 0
		v.voiced += len(frame)
	} else {
		v.silent += len(frame)
	}
	if v.silent >= 11200 || len(v.pcm) >= 240000 {
		return s.enqueueLocked(channel, v)
	}
	return false
}
func (s *Session) enqueueLocked(channel string, v *segment) bool {
	dropped := false
	if v.voiced >= 1600 && s.ctx.Err() == nil {
		u := utterance{channel: channel, pcm: v.pcm}
		select {
		case s.queue <- u:
		default:
			dropped = true
		}
	}
	v.pcm = nil
	v.voiced = 0
	v.silent = 0
	return dropped
}

func (s *Session) worker() {
	defer close(s.workerDone)
	for {
		select {
		case <-s.ctx.Done():
			return
		case u, ok := <-s.queue:
			if !ok || s.ctx.Err() != nil {
				return
			}
			text, err := s.transcribe(s.ctx, u.pcm)
			if err != nil {
				if s.ctx.Err() != nil {
					return
				}
				var apiErr *provider.APIError
				if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 402 || apiErr.Status == 403 || apiErr.Status == 404) {
					s.mu.Lock()
					stopping := s.stopping
					s.mu.Unlock()
					if stopping {
						s.report(err)
					} else {
						s.fail(err)
					}
					return
				}
				s.report(err)
				continue
			}
			if strings.TrimSpace(text) != "" {
				s.text(u.channel, text, true)
			}
		}
	}
}

func (s *Session) Stop() {
	s.once.Do(func() {
		s.mu.Lock()
		s.stopping = true
		stop := s.stopCapture
		s.mu.Unlock()
		if stop != nil {
			stop()
		}
		if s.inputStop != nil {
			close(s.inputStop)
			<-s.inputDone
		}
		s.mu.Lock()
		s.closed = true
		dropped := false
		for ch, v := range s.segments {
			if len(v.pending) > 0 {
				if s.frameLocked(ch, v, v.pending) {
					dropped = true
				}
				v.pending = nil
			}
			if s.enqueueLocked(ch, v) {
				dropped = true
			}
		}
		close(s.queue)
		s.mu.Unlock()
		if dropped {
			s.report(errors.New("停止时转写队列已满，最后一段音频未能加入队列"))
		}
		var wg sync.WaitGroup
		for _, stream := range s.streams {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := stream.Close(); err != nil && s.ctx.Err() == nil {
					s.report(err)
				}
			}()
		}
		wg.Wait()
		if s.realtime != nil {
			if err := s.realtime.Stop(); err != nil && s.ctx.Err() == nil {
				s.report(err)
			}
		}
		timer := time.NewTimer(15 * time.Second)
		select {
		case <-s.workerDone:
		case <-timer.C:
			s.report(errors.New("停止听写时转写队列超时，尚未完成的尾句已取消"))
			s.cancel()
			<-s.workerDone
		}
		timer.Stop()
		if s.local != nil {
			s.local.Close()
		}
		s.cancel()
	})
}

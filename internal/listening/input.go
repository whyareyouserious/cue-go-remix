package listening

import (
	"errors"
	"math"
	"sync"
	"time"

	"cue-go-remix/internal/sherpa"
)

type inputLevel struct {
	frames, samples, peak int64
	energy                float64
	last                  time.Time
}
type inputMonitor struct {
	mu                sync.Mutex
	channels          map[string]*inputLevel
	started, previous time.Time
	emit              func(sherpa.Metric)
	warned            bool
}

func newInputMonitor(system bool, now time.Time, emit func(sherpa.Metric)) *inputMonitor {
	m := &inputMonitor{channels: map[string]*inputLevel{"you": {}}, started: now, previous: now, emit: emit}
	if system {
		m.channels["them"] = &inputLevel{}
	}
	return m
}
func (m *inputMonitor) observe(channel string, pcm []int16, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.channels[channel]
	if v == nil {
		return
	}
	v.frames++
	v.samples += int64(len(pcm))
	v.last = now
	for _, sample := range pcm {
		x := int64(sample)
		if x < 0 {
			x = -x
		}
		if x > v.peak {
			v.peak = x
		}
		v.energy += float64(x * x)
	}
}
func (m *inputMonitor) snapshot(now time.Time, final bool) ([]sherpa.Metric, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []sherpa.Metric
	var warning error
	phase := "audio-input"
	if final {
		phase = "audio-input-stop"
	}
	for _, ch := range []string{"you", "them"} {
		v := m.channels[ch]
		if v == nil {
			continue
		}
		gap := int64(-1)
		if !v.last.IsZero() {
			gap = now.Sub(v.last).Milliseconds()
		}
		rms := int64(0)
		if v.samples > 0 {
			rms = int64(math.Sqrt(v.energy / float64(v.samples)))
		}
		out = append(out, sherpa.Metric{Phase: phase, Channel: ch, Frames: v.frames, AudioMS: v.samples / 16, ElapsedMS: now.Sub(m.previous).Milliseconds(), Peak: v.peak, RMS: rms, LastFrameMS: gap})
		if ch == "you" && !final && !m.warned && now.Sub(m.started) >= 6*time.Second {
			if v.last.IsZero() || gap >= 4000 {
				warning = errors.New("未收到麦克风音频，请检查系统输入设备，结束会话后重新开始")
			} else if v.samples > 0 && v.peak == 0 {
				warning = errors.New("麦克风送来的音频为静音，请检查耳机静音和系统输入音量")
			}
			if warning != nil {
				m.warned = true
			}
		}
		v.frames = 0
		v.samples = 0
		v.peak = 0
		v.energy = 0
	}
	m.previous = now
	return out, warning
}
func (s *Session) startInputMonitor() {
	if s.input == nil {
		return
	}
	s.input.mu.Lock()
	s.input.started = time.Now()
	s.input.previous = s.input.started
	s.input.mu.Unlock()
	s.inputStop = make(chan struct{})
	s.inputDone = make(chan struct{})
	go func() {
		defer close(s.inputDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		write := func(final bool) {
			metrics, warning := s.input.snapshot(time.Now(), final)
			for _, m := range metrics {
				s.input.emit(m)
			}
			if warning != nil {
				s.report(warning)
			}
		}
		for {
			select {
			case <-s.ctx.Done():
				write(true)
				return
			case <-s.inputStop:
				write(true)
				return
			case <-ticker.C:
				write(false)
			}
		}
	}()
}

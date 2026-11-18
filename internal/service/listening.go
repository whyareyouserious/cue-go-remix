package service

import (
	"context"
	"errors"
	"fmt"

	"cue-go-remix/internal/listening"
	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/sherpa"
)

// StartListening 启动音频采集和转写；重复调用保持当前会话，退出中或设备、模型初始化失败时返回错误。
func (a *Runtime) StartListening() error {
	a.trace("event=listening-start")
	a.listenMu.Lock()
	defer a.listenMu.Unlock()
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return errors.New("应用正在退出")
	}
	if a.listening {
		a.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.startCancel = cancel
	a.mu.Unlock()
	s := a.store.Settings()
	session, e := listening.Start(ctx, s, a.models, func(channel, text string, final bool) {
		if !final {
			a.emit(model.Event{Type: "interim", Channel: channel, Text: text})
			return
		}
		t, err := a.store.Append(channel, text)
		if err != nil {
			a.report(fmt.Errorf("转录保存失败：%w", err))
		}
		a.emit(model.Event{Type: "transcript", Data: t})
	}, a.report, func(m sherpa.Metric) {
		a.trace("event=speech-metric phase=%s channel=%s audio_ms=%d elapsed_ms=%d pending_ms=%d chars=%d frames=%d peak=%d rms=%d last_frame_ms=%d", m.Phase, m.Channel, m.AudioMS, m.ElapsedMS, m.PendingMS, m.Characters, m.Frames, m.Peak, m.RMS, m.LastFrameMS)
	})
	a.mu.Lock()
	a.startCancel = nil
	if e == nil {
		a.session = session
		a.listening = true
		a.sessionCancel = cancel
		a.slides = []model.Slide{}
	}
	a.mu.Unlock()
	if e != nil {
		a.trace("event=listening-failed stage=start")
		cancel()
		return e
	}
	a.trace("event=listening-ready provider=%s system_audio=%t", logging.Provider(s.Speech.Provider), s.MeetingAudio)
	a.status()
	a.startSlides(ctx, s)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		<-session.Done()
		if session.Err() == nil {
			return
		}
		a.listenMu.Lock()
		defer a.listenMu.Unlock()
		a.mu.Lock()
		current := a.session == session
		a.mu.Unlock()
		if current {
			a.trace("event=listening-failed stage=capture-or-transcription")
			a.stopListeningLocked()
		}
	}()
	return nil
}

// StopListening 取消正在初始化或运行的音频会话，释放模型，并按原流程尝试生成当前会议纪要。
func (a *Runtime) StopListening() error {
	a.trace("event=listening-stop")
	a.mu.Lock()
	if a.startCancel != nil {
		a.startCancel()
	}
	a.mu.Unlock()
	a.listenMu.Lock()
	defer a.listenMu.Unlock()
	a.stopListeningLocked()
	return nil
}

// stopListeningLocked 在持有 listenMu 时停止会话和页面任务并释放模型；纪要使用应用上下文，不复用已取消的音频上下文。
func (a *Runtime) stopListeningLocked() {
	a.mu.Lock()
	s := a.session
	a.session = nil
	stopSlides := a.slideCancel
	a.slideCancel = nil
	a.slideGeneration++
	cancel := a.sessionCancel
	a.sessionCancel = nil
	a.mu.Unlock()
	if stopSlides != nil {
		stopSlides()
	}
	if s != nil {
		s.Stop()
	}
	if cancel != nil {
		cancel()
	}
	a.models.Release()
	a.mu.Lock()
	a.listening = false
	a.mu.Unlock()
	a.status()
	if m := a.store.Current(); m != nil {
		a.makeNotes(*m)
	}
}

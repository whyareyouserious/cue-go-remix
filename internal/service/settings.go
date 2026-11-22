package service

import (
	"errors"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
)

func (a *Runtime) GetSettings() model.Settings { return repository.Redact(a.store.Settings()) }

func (a *Runtime) SaveSettings(s model.Settings) (model.Settings, error) {
	a.trace("event=settings-save-start chat=%s speech=%s", logging.Provider(s.Chat.Provider), logging.Provider(s.Speech.Provider))
	a.listenMu.Lock()
	defer a.listenMu.Unlock()
	// 先解析账号池中的实际语音端点，防止仅改变激活 ID 绕过录音期间的配置保护。
	resolved, err := a.store.PrepareSettings(s)
	if err != nil {
		a.trace("event=settings-save-failed reason=validation-or-storage")
		return model.Settings{}, err
	}
	a.mu.Lock()
	live := a.listening
	a.mu.Unlock()
	if live {
		old := a.store.Settings()
		if resolved.Speech != old.Speech || resolved.MeetingAudio != old.MeetingAudio || resolved.WhisperModel != old.WhisperModel || resolved.WhisperPath != old.WhisperPath || resolved.WhisperThreads != old.WhisperThreads || resolved.Language != old.Language {
			a.trace("event=settings-save-failed reason=listening")
			return model.Settings{}, errors.New("请先结束会话再修改音频服务、语言或模型")
		}
	}
	v, e := a.store.SaveSettings(s)
	if e != nil {
		a.trace("event=settings-save-failed reason=validation-or-storage")
		return v, e
	}
	a.trace("event=settings-save-done chat=%s speech=%s stream=%t", logging.Provider(v.Chat.Provider), logging.Provider(v.Speech.Provider), v.Chat.Stream)
	if a.settingsChanged != nil {
		a.settingsChanged(v)
	}
	return v, nil
}

package service

import (
	"errors"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
)

// GetSettings 返回面向前端的设置副本，隐藏 API Key 并通过 HasKey 标记是否已配置。
func (a *Runtime) GetSettings() model.Settings { return repository.Redact(a.store.Settings()) }

// SaveSettings 校验并持久化 s，返回隐藏密钥后的设置；录音期间拒绝切换音频配置，写入失败返回错误。
func (a *Runtime) SaveSettings(s model.Settings) (model.Settings, error) {
	a.trace("event=settings-save-start chat=%s speech=%s", logging.Provider(s.Chat.Provider), logging.Provider(s.Speech.Provider))
	a.listenMu.Lock()
	defer a.listenMu.Unlock()
	a.mu.Lock()
	live := a.listening
	a.mu.Unlock()
	if live {
		old := a.store.Settings()
		if s.Speech != repository.Redact(old).Speech || s.MeetingAudio != old.MeetingAudio || s.WhisperModel != old.WhisperModel || s.WhisperPath != old.WhisperPath || s.Language != old.Language {
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

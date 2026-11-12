package repository

import (
	"errors"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"cue-go-remix/internal/model"
)

func DefaultSettings() model.Settings {
	return model.Settings{
		Chat:     model.Endpoint{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", SmartModel: "gpt-4o", Stream: true},
		Speech:   model.Endpoint{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "whisper-1"},
		Language: "zh", Opacity: 0.94, ContentProtection: true, MeetingAudio: runtime.GOOS == "windows",
		SlidesInterval: 3000, WhisperModel: "base", WhisperThreads: 4,
	}
}

func (s *Store) Settings() model.Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.settings }

func Redact(v model.Settings) model.Settings {
	v.Chat.HasKey = v.Chat.APIKey != ""
	v.Chat.APIKey = ""
	v.Chat.ClearKey = false
	v.Speech.HasKey = v.Speech.APIKey != ""
	v.Speech.APIKey = ""
	v.Speech.ClearKey = false
	return v
}

func endpoint(ep, old model.Endpoint) (model.Endpoint, error) {
	ep.Provider = strings.ToLower(strings.TrimSpace(ep.Provider))
	ep.BaseURL = strings.TrimSpace(ep.BaseURL)
	if ep.BaseURL != "" {
		u, err := url.Parse(ep.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return ep, errors.New("服务地址必须为有效的 http/https 地址，不能包含用户名或密码")
		}
	}
	if ep.ClearKey {
		ep.APIKey = ""
	} else if ep.APIKey == "" && ep.Provider == old.Provider && ep.BaseURL == old.BaseURL {
		ep.APIKey = old.APIKey
	}
	ep.APIKey = strings.TrimSpace(ep.APIKey)
	ep.HasKey = ep.APIKey != ""
	ep.ClearKey = false
	if strings.ContainsAny(ep.APIKey, "\r\n") {
		return ep, errors.New("API Key 不能包含换行")
	}
	return ep, nil
}

func (s *Store) SaveSettings(v model.Settings) (model.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	v.Chat, err = endpoint(v.Chat, s.settings.Chat)
	if err != nil {
		return model.Settings{}, err
	}
	v.Speech, err = endpoint(v.Speech, s.settings.Speech)
	if err != nil {
		return model.Settings{}, err
	}
	if v.Opacity < .2 || v.Opacity > 1 {
		return model.Settings{}, errors.New("透明度须在 20% 到 100% 之间")
	}
	if len([]rune(v.AIRules)) > 2000 {
		return model.Settings{}, errors.New("回答风格最多 2000 字")
	}
	if v.SlidesInterval < 1500 {
		v.SlidesInterval = 3000
	}
	if v.SlidesInterval > 15000 {
		v.SlidesInterval = 15000
	}
	if v.WhisperThreads < 0 || v.WhisperThreads > 64 {
		return model.Settings{}, errors.New("Whisper 线程数须在 0 到 64 之间")
	}
	if v.Language == "" {
		v.Language = "zh"
	}
	// Window coordinates are maintained by the desktop process, not settings forms.
	v.WindowX = s.settings.WindowX
	v.WindowY = s.settings.WindowY
	v.WindowPositioned = s.settings.WindowPositioned
	if err = WriteJSON(filepath.Join(s.dir, "settings.json"), v); err != nil {
		return model.Settings{}, err
	}
	s.settings = v
	return Redact(v), nil
}

func (s *Store) SavePosition(x, y int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.settings
	s.settings.WindowX = x
	s.settings.WindowY = y
	s.settings.WindowPositioned = true
	if err := WriteJSON(filepath.Join(s.dir, "settings.json"), s.settings); err != nil {
		s.settings = previous
		return err
	}
	return nil
}

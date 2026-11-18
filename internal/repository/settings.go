package repository

import (
	"errors"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"cue-go-remix/internal/model"
)

// DefaultSettings 返回默认模型、音频和界面设置；系统音频默认值按操作系统决定。
func DefaultSettings() model.Settings {
	return model.Settings{
		Chat:     model.Endpoint{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", SmartModel: "gpt-4o", Stream: true},
		Speech:   model.Endpoint{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "whisper-1"},
		Language: "zh", Opacity: 0.94, ContentProtection: true, MeetingAudio: runtime.GOOS == "windows",
		SlidesInterval: 3000, WhisperModel: "base", WhisperThreads: 4,
	}
}

// Settings 在锁内返回内部设置副本，包含完整密钥；面向前端必须先调用 Redact。
func (s *Store) Settings() model.Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.settings }

// Redact 清除 v 中的聊天和语音密钥，保留 HasKey 标志，返回可供前端使用的设置副本。
func Redact(v model.Settings) model.Settings {
	v.Chat.HasKey = v.Chat.APIKey != ""
	v.Chat.APIKey = ""
	v.Chat.ClearKey = false
	v.Speech.HasKey = v.Speech.APIKey != ""
	v.Speech.APIKey = ""
	v.Speech.ClearKey = false
	return v
}

// endpoint 校验 ep 的服务地址与密钥；仅在提供商和地址未变化时继承 old 的密钥，失败返回错误。
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

// SaveSettings 校验并持久化 v，保留桌面维护的窗口坐标；成功返回脱敏设置，失败不替换内存设置。
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
	// 窗口位置由桌面进程维护，设置表单不能覆盖移动后的位置。
	v.WindowX = s.settings.WindowX
	v.WindowY = s.settings.WindowY
	v.WindowPositioned = s.settings.WindowPositioned
	if err = WriteJSON(filepath.Join(s.dir, "settings.json"), v); err != nil {
		return model.Settings{}, err
	}
	s.settings = v
	return Redact(v), nil
}

// SavePosition 将 x、y 作为窗口逻辑像素坐标持久化；写入失败回滚内存中的位置设置。
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

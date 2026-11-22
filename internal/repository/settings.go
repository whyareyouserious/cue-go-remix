package repository

import (
	"errors"
	"fmt"
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
		ActiveScenarioID: "default",
		Scenarios: []model.Scenario{
			{
				ID:   "default",
				Name: "默认场景",
				Profile: model.Profile{
					Role:      "",
					Industry:  "",
					Years:     "",
					Skills:    "",
					Strengths: "",
					Goals:     "",
					Projects:  []model.ProjectExperience{},
				},
				Preferences: model.Preferences{
					Length:    "",
					Tone:      "",
					Structure: "",
					Examples:  "",
					Custom:    "",
				},
			},
		},
		Shortcuts: map[string]string{
			"say":         "CmdOrCtrl+Enter",
			"assist":      "CmdOrCtrl+Shift+Enter",
			"leetcode":    "CmdOrCtrl+H",
			"toggle":      "CmdOrCtrl+Shift+/",
			"quit":        "CmdOrCtrl+Shift+X",
			"passthrough": "CmdOrCtrl+Shift+I",
		},
	}
}

// Settings 在锁内返回内部设置深拷贝副本，包含完整密钥；面向前端必须先调用 Redact。
func (s *Store) Settings() model.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Clone()
}

// Redact 清除 v 中的聊天和语音密钥，保留 HasKey 标志，返回可供前端使用的深拷贝设置副本。
func Redact(v model.Settings) model.Settings {
	v = v.Clone()
	v.Chat = redactEndpoint(v.Chat)
	v.Speech = redactEndpoint(v.Speech)
	for i := range v.ChatChannels {
		v.ChatChannels[i].MaskedAPIKey = maskAPIKey(v.ChatChannels[i].APIKey)
		v.ChatChannels[i].Endpoint = redactEndpoint(v.ChatChannels[i].Endpoint)
	}
	for i := range v.SpeechChannels {
		v.SpeechChannels[i].MaskedAPIKey = maskAPIKey(v.SpeechChannels[i].APIKey)
		v.SpeechChannels[i].Endpoint = redactEndpoint(v.SpeechChannels[i].Endpoint)
	}
	return v
}

// maskAPIKey 生成只读展示预览：长密钥保留首尾各四个字符，八字符及以下仅显示星号，空值不生成预览。
func maskAPIKey(key string) string {
	characters := []rune(key)
	if len(characters) == 0 {
		return ""
	}
	if len(characters) <= 8 {
		return "*****"
	}
	return string(characters[:4]) + "*****" + string(characters[len(characters)-4:])
}

// redactEndpoint 隐藏单个端点的完整凭据，HasKey 仅从真实密钥计算。
func redactEndpoint(ep model.Endpoint) model.Endpoint {
	ep.HasKey = ep.APIKey != ""
	ep.APIKey = ""
	ep.ClearKey = false
	return ep
}

// endpoint 校验 ep 的服务地址与密钥；仅在提供商和地址未变化时继承 old 的密钥，失败返回错误。
func endpoint(ep, old model.Endpoint) (model.Endpoint, error) {
	ep.Provider = strings.ToLower(strings.TrimSpace(ep.Provider))
	ep.ChatProtocol = strings.ToLower(strings.TrimSpace(ep.ChatProtocol))
	if err := ep.ValidateChatProtocol(); err != nil {
		return ep, err
	}
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
	v, err := s.prepareSettingsLocked(v)
	if err != nil {
		return model.Settings{}, err
	}
	if err = WriteJSON(filepath.Join(s.dir, "settings.json"), v); err != nil {
		return model.Settings{}, err
	}
	s.settings = v.Clone()
	return Redact(v), nil
}

// PrepareSettings 返回完整校验、密钥合并和激活渠道解析后的独立配置，仅供内部业务检查；不写盘、不修改当前设置，结果包含密钥不得直接返回前端。
func (s *Store) PrepareSettings(v model.Settings) (model.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepareSettingsLocked(v)
}

// prepareSettingsLocked 在调用方持有存储锁时校验候选设置；返回完整内部副本或校验错误，不产生持久化副作用。
func (s *Store) prepareSettingsLocked(v model.Settings) (model.Settings, error) {
	v = v.Clone()
	if err := prepareChannels(&v, s.settings); err != nil {
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
	if len(v.Scenarios) == 0 {
		v.Scenarios = DefaultSettings().Scenarios
	}

	// 场景 ID 唯一性与各字段长度严密校验（场景偏好优先于全局规则）
	scenarioIDs := make(map[string]bool)
	for i, sc := range v.Scenarios {
		scID := strings.TrimSpace(sc.ID)
		if scID == "" {
			return model.Settings{}, fmt.Errorf("第 %d 个场景的 ID 不能为空", i+1)
		}
		if scenarioIDs[scID] {
			return model.Settings{}, fmt.Errorf("场景 ID 重复：%s", scID)
		}
		scenarioIDs[scID] = true

		if len([]rune(sc.Name)) > 50 {
			return model.Settings{}, fmt.Errorf("场景名称最多 50 字: %s", sc.Name)
		}
		// 个人定位分项长度限制
		p := sc.Profile
		if len([]rune(p.Role)) > 100 || len([]rune(p.Industry)) > 100 || len([]rune(p.Years)) > 50 {
			return model.Settings{}, errors.New("个人岗位、行业或年限超出字符上限")
		}
		if len([]rune(p.Skills)) > 500 || len([]rune(p.Strengths)) > 500 || len([]rune(p.Goals)) > 500 {
			return model.Settings{}, errors.New("技能专长、核心擅长或目标超出 500 字限制")
		}
		for _, proj := range p.Projects {
			if len([]rune(proj.Name)) > 100 || len([]rune(proj.Role)) > 100 {
				return model.Settings{}, errors.New("项目名称或担任角色超出 100 字限制")
			}
			if len([]rune(proj.Achievements)) > 2000 {
				return model.Settings{}, errors.New("项目职责与成果描述超出 2000 字限制")
			}
		}
		// 回答偏好自定义补充长度限制
		if len([]rune(sc.Preferences.Custom)) > 1000 {
			return model.Settings{}, errors.New("自定义回答偏好超出 1000 字限制")
		}
	}

	if v.ActiveScenarioID == "" || !scenarioIDs[v.ActiveScenarioID] {
		v.ActiveScenarioID = v.Scenarios[0].ID
	}
	// 窗口位置与全局快捷键由专用路径独立维护，普通设置保存不能覆盖移动后的位置或当前生效的系统快捷键。
	v.WindowX = s.settings.WindowX
	v.WindowY = s.settings.WindowY
	v.WindowPositioned = s.settings.WindowPositioned
	if len(s.settings.Shortcuts) > 0 {
		v.Shortcuts = s.settings.Clone().Shortcuts
	} else if len(v.Shortcuts) == 0 {
		v.Shortcuts = DefaultSettings().Shortcuts
	}
	return v, nil
}

// SaveShortcuts 持久化自定义全局快捷键映射；写入失败回滚内存设置。
func (s *Store) SaveShortcuts(shortcuts map[string]string) (model.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.settings.Clone()
	s.settings.Shortcuts = make(map[string]string, len(shortcuts))
	for k, v := range shortcuts {
		s.settings.Shortcuts[k] = v
	}
	if err := WriteJSON(filepath.Join(s.dir, "settings.json"), s.settings); err != nil {
		s.settings = previous
		return model.Settings{}, err
	}
	return Redact(s.settings), nil
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

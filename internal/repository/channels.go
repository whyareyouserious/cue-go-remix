package repository

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"cue-go-remix/internal/model"
)

var channelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,95}$`)

// migrateChannels 在读取旧配置时补充稳定渠道标识，仅修改内存副本，不改写文件或切换原服务。
func migrateChannels(v *model.Settings) error {
	// 预览属于响应字段，读取文件时同样丢弃，避免历史或外部写入值进入后续持久化。
	for i := range v.ChatChannels {
		v.ChatChannels[i].MaskedAPIKey = ""
	}
	for i := range v.SpeechChannels {
		v.SpeechChannels[i].MaskedAPIKey = ""
	}
	if v.ChatChannels == nil {
		v.ChatChannels, v.ActiveChatChannelID = legacyChannel("chat", v.Chat)
	} else {
		ep, err := selectedChannel("chat", v.ChatChannels, v.ActiveChatChannelID, *v)
		if err != nil {
			return err
		}
		v.Chat = ep
	}
	if v.SpeechChannels == nil {
		v.SpeechChannels, v.ActiveSpeechChannelID = legacyChannel("speech", v.Speech)
	} else {
		ep, err := selectedChannel("speech", v.SpeechChannels, v.ActiveSpeechChannelID, *v)
		if err != nil {
			return err
		}
		v.Speech = ep
	}
	return nil
}

// legacyChannel 为已配置的旧端点建立卡片；空白默认云端配置不制造已配置渠道，本地引擎不进入 API 池。
func legacyChannel(kind string, ep model.Endpoint) ([]model.APIChannel, string) {
	if kind == "speech" {
		if ep.Provider == "sherpa" {
			return nil, "local_sherpa"
		}
		if ep.Provider == "local" {
			return nil, "local_whisper"
		}
	}
	if ep.Provider == "" || (ep.APIKey == "" && ep.Provider != "custom" && ep.Provider != "ollama") {
		return nil, ""
	}
	name := "原有聊天服务"
	if kind == "speech" {
		name = "原有语音服务"
	}
	id := "legacy_" + kind
	return []model.APIChannel{{ID: id, Name: name, Endpoint: ep, Status: "untested"}}, id
}

// prepareChannels 校验两个独立渠道池并解析生效端点；nil 池兼容旧客户端，显式空池不会复活旧端点。
func prepareChannels(v *model.Settings, old model.Settings) error {
	var err error
	v.ChatChannels, v.ActiveChatChannelID, v.Chat, err = prepareChannelKind("chat", v.ChatChannels, v.ActiveChatChannelID, v.Chat, old.ChatChannels, old.ActiveChatChannelID, old.Chat, *v)
	if err != nil {
		return err
	}
	v.SpeechChannels, v.ActiveSpeechChannelID, v.Speech, err = prepareChannelKind("speech", v.SpeechChannels, v.ActiveSpeechChannelID, v.Speech, old.SpeechChannels, old.ActiveSpeechChannelID, old.Speech, *v)
	return err
}

// prepareChannelKind 归一化单类渠道；返回独立池、激活 ID 和完整有效端点，所有校验失败均不修改持久状态。
func prepareChannelKind(kind string, channels []model.APIChannel, activeID string, legacy model.Endpoint, oldChannels []model.APIChannel, oldActiveID string, oldEndpoint model.Endpoint, settings model.Settings) ([]model.APIChannel, string, model.Endpoint, error) {
	if channels == nil {
		if kind == "speech" && legacy.ChatProtocol != "" {
			return nil, "", model.Endpoint{}, errors.New("语音渠道不能设置聊天协议")
		}
		// 旧客户端只提交单端点：仅更新原激活渠道，保留其他已存渠道和凭据。
		ep, err := endpoint(legacy, oldEndpoint)
		if err != nil {
			return nil, "", model.Endpoint{}, err
		}
		channels = append([]model.APIChannel(nil), oldChannels...)
		if kind == "speech" && (ep.Provider == "sherpa" || ep.Provider == "local") {
			activeID = "local_sherpa"
			if ep.Provider == "local" {
				activeID = "local_whisper"
			}
			resolved, err := selectedChannel(kind, channels, activeID, settings)
			return channels, activeID, resolved, err
		}
		if ep.Provider != "" && !validChannelProvider(kind, ep.Provider) {
			return nil, "", model.Endpoint{}, errors.New("渠道提供商不支持所选的聊天或语音协议")
		}
		if oldActiveID != "" && oldActiveID != "local_sherpa" && oldActiveID != "local_whisper" {
			for i := range channels {
				if channels[i].ID == oldActiveID {
					channels[i].Endpoint = ep
					channels[i].Status, channels[i].Latency = "untested", 0
					activeID = oldActiveID
					// 旧端点已按原激活端点完成密钥合并，不再借用池中其他身份。
					validated, err := normalizeChannels(kind, channels, nil)
					if err != nil {
						return nil, "", model.Endpoint{}, err
					}
					resolved, err := selectedChannel(kind, validated, activeID, settings)
					return validated, activeID, resolved, err
				}
			}
		}
		migrated, id := legacyChannel(kind, ep)
		if len(migrated) == 0 {
			return channels, "", ep, nil
		}
		for _, previous := range channels {
			if previous.ID == id {
				id += "_" + ID()
				break
			}
		}
		migrated[0].ID = id
		channels = append(channels, migrated[0])
		validated, err := normalizeChannels(kind, channels, nil)
		if err != nil {
			return nil, "", model.Endpoint{}, err
		}
		resolved, err := selectedChannel(kind, validated, id, settings)
		return validated, id, resolved, err
	}

	validated, err := normalizeChannels(kind, channels, oldChannels)
	if err != nil {
		return nil, "", model.Endpoint{}, err
	}
	ep, err := selectedChannel(kind, validated, activeID, settings)
	return validated, activeID, ep, err
}

// normalizeChannels 逐条校验渠道并按同类同 ID 合并凭据；未知 ID、改提供商或改 URL 均不能继承其他账号密钥。
func normalizeChannels(kind string, channels, oldChannels []model.APIChannel) ([]model.APIChannel, error) {
	result := make([]model.APIChannel, len(channels))
	previous := make(map[string]model.APIChannel, len(oldChannels))
	for _, channel := range oldChannels {
		previous[channel.ID] = channel
	}
	seen := make(map[string]bool, len(channels))
	for i, channel := range channels {
		// 仅由 Redact 根据实际凭据生成展示值，永不信任或保存前端提交的预览。
		channel.MaskedAPIKey = ""
		if !channelIDPattern.MatchString(channel.ID) || channel.ID == "local_sherpa" || channel.ID == "local_whisper" {
			return nil, errors.New("渠道 ID 格式不正确或使用了内置引擎保留标识")
		}
		if seen[channel.ID] {
			return nil, errors.New("同类渠道 ID 不能重复")
		}
		seen[channel.ID] = true
		channel.Name = strings.TrimSpace(channel.Name)
		if channel.Name == "" || len([]rune(channel.Name)) > 100 {
			return nil, errors.New("渠道名称须为 1 到 100 个字符")
		}
		if len([]rune(channel.CreatedAt)) > 100 {
			return nil, errors.New("渠道创建时间文本过长")
		}
		old := previous[channel.ID]
		ep, err := endpoint(channel.Endpoint, old.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个渠道地址或凭据无效：%w", i+1, err)
		}
		if kind == "speech" && ep.ChatProtocol != "" {
			return nil, errors.New("语音渠道不能设置聊天协议")
		}
		if !validChannelProvider(kind, ep.Provider) {
			return nil, errors.New("渠道提供商不支持所选的聊天或语音协议")
		}
		// 官方协议沿用 provider 层已有默认地址；自定义和 Azure 没有统一可推断的端点。
		if ep.BaseURL == "" && (ep.Provider == "custom" || ep.Provider == "azure") {
			return nil, errors.New("自定义或 Azure 渠道必须填写 HTTP/HTTPS 服务地址")
		}
		ep.Model = strings.TrimSpace(ep.Model)
		ep.SmartModel = strings.TrimSpace(ep.SmartModel)
		if ep.Model == "" || len([]rune(ep.Model)) > 200 || len([]rune(ep.SmartModel)) > 200 {
			return nil, errors.New("渠道模型名称不能为空且最多 200 个字符")
		}
		if kind == "speech" && ep.Stream && ep.Provider != "openai" && ep.Provider != "deepgram" && ep.Provider != "gemini" {
			return nil, errors.New("当前语音渠道不支持实时转写，请使用分段模式")
		}
		if kind == "chat" && ep.SmartModel == "" {
			ep.SmartModel = ep.Model
		}
		if channel.Status == "" {
			channel.Status = "untested"
		}
		if channel.Status != "untested" && channel.Status != "success" && channel.Status != "failed" {
			return nil, errors.New("渠道检测状态无效")
		}
		if channel.Latency < 0 {
			return nil, errors.New("渠道检测耗时不能为负数")
		}
		channel.Endpoint = ep
		result[i] = channel
	}
	return result, nil
}

// selectedChannel 将已校验的激活渠道投影到旧业务端点；未知或缺失 ID 返回错误，不擅自选择其他服务。
func selectedChannel(kind string, channels []model.APIChannel, activeID string, settings model.Settings) (model.Endpoint, error) {
	if kind == "speech" {
		switch activeID {
		case "local_sherpa":
			return model.Endpoint{Provider: "sherpa", Model: "zipformer", Stream: true}, nil
		case "local_whisper":
			return model.Endpoint{Provider: "local", Model: settings.WhisperModel}, nil
		}
	}
	if activeID == "" && len(channels) == 0 {
		return model.Endpoint{}, nil
	}
	for _, channel := range channels {
		if channel.ID == activeID {
			return channel.Endpoint, nil
		}
	}
	return model.Endpoint{}, errors.New("激活的渠道不存在，请选择有效渠道后保存")
}

// validChannelProvider 限定已有实现支持的协议，展示品牌应映射到 custom 等实际协议而非扩展任意标识。
func validChannelProvider(kind, provider string) bool {
	if kind == "speech" {
		switch provider {
		case "openai", "custom", "groq", "gemini", "deepgram":
			return true
		}
		return false
	}
	switch provider {
	case "openai", "custom", "azure", "cerebras", "deepseek", "groq", "minimax", "ollama", "anthropic", "gemini":
		return true
	}
	return false
}

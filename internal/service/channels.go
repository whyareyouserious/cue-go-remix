package service

import (
	"errors"
	"strings"

	"cue-go-remix/internal/model"
)

// channelPool 按 kind 返回同类账号副本；未知类型返回错误，禁止跨聊天和语音账号查找凭据。
func (a *Runtime) channelPool(kind string) ([]model.APIChannel, error) {
	s := a.store.Settings()
	switch kind {
	case "chat":
		return s.ChatChannels, nil
	case "speech":
		return s.SpeechChannels, nil
	default:
		return nil, errors.New("无效的服务类型，仅支持 chat 或 speech")
	}
}

// CheckChannelEndpoint 检测账号草稿，只按同类型、同 ID、同提供商和同地址继承密钥。
// 新账号使用显式输入的凭据；失败返回探测原因，不修改配置，不触碰录音设备。
func (a *Runtime) CheckChannelEndpoint(kind string, channel model.APIChannel) (model.EndpointCheckResult, error) {
	channels, err := a.channelPool(kind)
	if err != nil {
		return model.EndpointCheckResult{}, err
	}
	ep := channel.Endpoint
	ep.Provider = strings.ToLower(strings.TrimSpace(ep.Provider))
	ep.BaseURL = strings.TrimSpace(ep.BaseURL)
	if ep.ClearKey {
		ep.APIKey = ""
	} else if ep.APIKey == "" {
		for _, saved := range channels {
			if saved.ID == channel.ID && saved.Provider == ep.Provider && saved.BaseURL == ep.BaseURL {
				ep.APIKey = saved.APIKey
				break
			}
		}
	}
	return a.checkResolvedEndpoint(kind, ep)
}

// GetChannelKey 返回用户明确选择复制或显示的已存账号密钥；普通设置读取保持脱敏，缺失账号或密钥返回错误。
func (a *Runtime) GetChannelKey(kind, id string) (string, error) {
	channels, err := a.channelPool(kind)
	if err != nil {
		return "", err
	}
	for _, channel := range channels {
		if channel.ID == id {
			if channel.APIKey == "" {
				return "", errors.New("该账号尚未保存 API Key")
			}
			return channel.APIKey, nil
		}
	}
	return "", errors.New("账号尚未保存或已删除")
}

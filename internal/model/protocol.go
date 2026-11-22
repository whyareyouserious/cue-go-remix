package model

import "errors"

const (
	// ChatProtocolCompletions 使用 OpenAI 兼容的 Chat Completions 接口。
	ChatProtocolCompletions = "chat-completions"
	// ChatProtocolResponses 使用 Responses 接口，独立编码请求和响应。
	ChatProtocolResponses = "responses"
)

// ValidateChatProtocol 校验显式聊天协议；空值保留旧配置，原生 Anthropic/Gemini 不接受兼容协议覆盖。
func (ep Endpoint) ValidateChatProtocol() error {
	if ep.ChatProtocol == "" {
		return nil
	}
	if ep.ChatProtocol != ChatProtocolCompletions && ep.ChatProtocol != ChatProtocolResponses {
		return errors.New("聊天协议仅支持 Chat Completions 或 Responses")
	}
	switch ep.Provider {
	case "openai", "custom", "azure", "cerebras", "deepseek", "groq", "minimax", "ollama":
		return nil
	default:
		return errors.New("当前服务商使用原生协议，请清除兼容聊天协议设置")
	}
}

package logging

import (
	"context"
	"errors"
	"fmt"
	"net"

	"cue-go-remix/internal/model"
	"cue-go-remix/internal/provider"
)

func Provider(provider string) string {
	switch provider {
	case "custom", "openai", "anthropic", "gemini", "azure", "deepseek", "groq", "cerebras", "minimax", "ollama", "deepgram", "local", "sherpa":
		return provider
	default:
		return "unknown"
	}
}

// Error categories carry no endpoint, credentials or response body.
func Error(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var api *provider.APIError
	if errors.As(err, &api) {
		if api.Status < 100 || api.Status > 599 {
			return "other"
		}
		return fmt.Sprintf("http-%d", api.Status)
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return "timeout"
		}
		return "network"
	}
	return "other"
}

// ChatProtocol 返回可写入日志的固定协议标识；原生和语音端点不输出客户端传入的任意文本。
func ChatProtocol(ep model.Endpoint) string {
	if ep.ChatProtocol == model.ChatProtocolResponses {
		return model.ChatProtocolResponses
	}
	if ep.ChatProtocol != "" && ep.ChatProtocol != model.ChatProtocolCompletions {
		return "unknown"
	}
	switch ep.Provider {
	case "openai", "custom", "azure", "cerebras", "deepseek", "groq", "minimax", "ollama":
		return model.ChatProtocolCompletions
	default:
		return "native"
	}
}

// ErrorCategory 返回固定业务错误分类；协议不匹配单独标识，其他错误沿用状态或网络分类。
func ErrorCategory(err error) string {
	var api *provider.APIError
	if errors.As(err, &api) && api.Category == "protocol-mismatch" {
		return "protocol-mismatch"
	}
	return Error(err)
}

package logging

import (
	"context"
	"errors"
	"fmt"
	"net"

	"cue-go-remix/internal/provider"
)

// Provider 将 provider 归一到允许记录的固定名称，未识别值返回 unknown。
func Provider(provider string) string {
	switch provider {
	case "custom", "openai", "anthropic", "gemini", "azure", "deepseek", "groq", "cerebras", "minimax", "ollama", "deepgram", "local", "sherpa":
		return provider
	default:
		return "unknown"
	}
}

// Error 将 err 分类为固定诊断标记，不返回服务地址、凭据或响应正文。
func Error(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var api *provider.APIError
	if errors.As(err, &api) {
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

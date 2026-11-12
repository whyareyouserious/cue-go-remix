package logging

import (
	"context"
	"errors"
	"fmt"
	"net"

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

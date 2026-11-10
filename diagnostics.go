package main

import (
	"context"
	"cue-go-remix/internal/provider"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
)

// 只写固定事件名、枚举值和数值，禁止写入设置、URL、密钥和对话原文。
type diagnostics struct {
	file   *os.File
	logger *log.Logger
}

func openDiagnostics(dir string) (*diagnostics, error) {
	dir = filepath.Join(dir, "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "cue.log")
	if info, err := os.Stat(path); err == nil && info.Size() >= 1024*1024 {
		previous := filepath.Join(dir, "cue.previous.log")
		_ = os.Remove(previous)
		if err := os.Rename(path, previous); err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &diagnostics{file: file, logger: log.New(io.MultiWriter(file, os.Stderr), fmt.Sprintf("pid=%d ", os.Getpid()), log.LstdFlags|log.Lmicroseconds)}, nil
}

func (a *App) trace(format string, args ...any) {
	if a.diagnostics != nil {
		a.diagnostics.logger.Printf(format, args...)
	}
}

func diagnosticProvider(provider string) string {
	switch provider {
	case "custom", "openai", "anthropic", "gemini", "azure", "deepseek", "groq", "cerebras", "minimax", "ollama", "deepgram", "local", "sherpa":
		return provider
	default:
		return "unknown"
	}
}

// Error categories carry no endpoint, credentials or response body.
func diagnosticError(err error) string {
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

// RecordUIEvent 的所有参数均受白名单约束，不能用来写入任意用户输入。
func (a *App) RecordUIEvent(event, value string) {
	allowed := false
	switch event {
	case "settings-open", "settings-save":
		switch value {
		case "chat", "speech", "style", "appearance", "models", "access":
			allowed = true
		}
	case "provider-select":
		allowed = diagnosticProvider(value) != "unknown"
	case "key-focus":
		allowed = value == "chat" || value == "speech"
	case "panel-toggle":
		allowed = value == "collapsed" || value == "expanded"
	case "sidebar-toggle":
		allowed = value == "open" || value == "closed"
	}
	if allowed {
		a.trace("event=ui action=%s value=%s", event, value)
	}
}

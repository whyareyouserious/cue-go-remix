package service

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
)

func (a *Runtime) emit(e model.Event) {
	if a.emitEvent != nil {
		a.emitEvent(e)
	}
}
func (a *Runtime) Emit(e model.Event) { a.emit(e) }

// Report 在 err 非空时记录错误类别并通知前端，日志不写入原始异常文本。
func (a *Runtime) Report(err error) { a.report(err) }

// Warn 保存 text 为运行警告并通知前端。
func (a *Runtime) Warn(text string) { a.warn(text) }

// AddRecentErrorCategory 在锁内添加一条固定脱敏分类记录（最多保留 50 条）。
func (a *Runtime) AddRecentErrorCategory(level, category string) {
	if category == "" {
		return
	}
	a.errorMu.Lock()
	defer a.errorMu.Unlock()
	entry := model.RecentError{
		ID:       fmt.Sprintf("err-%d-%d", time.Now().UnixMilli(), len(a.recentErrors)+1),
		Time:     time.Now().UnixMilli(),
		Level:    level,
		Category: category,
	}
	a.recentErrors = append(a.recentErrors, entry)
	if len(a.recentErrors) > 50 {
		a.recentErrors = a.recentErrors[len(a.recentErrors)-50:]
	}
}

// GetRecentErrors 返回近期错误和告警固定分类快照。
func (a *Runtime) GetRecentErrors() []model.RecentError {
	a.errorMu.Lock()
	defer a.errorMu.Unlock()
	res := make([]model.RecentError, len(a.recentErrors))
	copy(res, a.recentErrors)
	return res
}

// ClearRecentErrors 清空近期错误和告警列表。
func (a *Runtime) ClearRecentErrors() {
	a.errorMu.Lock()
	defer a.errorMu.Unlock()
	a.recentErrors = []model.RecentError{}
}

// classifyError 将原始 err 映射为固定脱敏分类代码，禁止包含明文或用户数据。
func classifyError(err error) string {
	if err == nil {
		return "none"
	}
	if logging.ErrorCategory(err) == "protocol-mismatch" {
		return "protocol-mismatch"
	}
	s := strings.ToLower(err.Error())
	if errors.Is(err, context.Canceled) || strings.Contains(s, "cancel") {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(s, "timeout") {
		return "timeout"
	}
	if strings.Contains(s, "401") {
		return "http-401"
	}
	if strings.Contains(s, "403") {
		return "http-403"
	}
	if strings.Contains(s, "404") {
		return "http-404"
	}
	if strings.Contains(s, "429") {
		return "http-429"
	}
	if strings.Contains(s, "500") || strings.Contains(s, "502") || strings.Contains(s, "503") || strings.Contains(s, "504") {
		return "http-5xx"
	}
	if strings.Contains(s, "connection refused") || strings.Contains(s, "network") || strings.Contains(s, "dial") {
		return "network-error"
	}
	if strings.Contains(s, "permission") || strings.Contains(s, "not permitted") {
		return "permission-denied"
	}
	if strings.Contains(s, "shortcut") || strings.Contains(s, "快捷键") {
		return "shortcut-conflict"
	}
	kind := logging.Error(err)
	if kind != "other" {
		return kind
	}
	return "general-error"
}

// classifyWarning 将运行警告 text 映射为固定脱敏分类代码。
func classifyWarning(text string) string {
	s := strings.ToLower(text)
	if strings.Contains(s, "快捷键") || strings.Contains(s, "shortcut") {
		return "shortcut-conflict"
	}
	if strings.Contains(s, "录屏") || strings.Contains(s, "protection") {
		return "screen-protection-unavailable"
	}
	if strings.Contains(s, "sherpa") || strings.Contains(s, "whisper") || strings.Contains(s, "语音") || strings.Contains(s, "音频") {
		return "audio-runtime-warning"
	}
	if strings.Contains(s, "助手") || strings.Contains(s, "applink") {
		return "applink-warning"
	}
	if strings.Contains(s, "日志") || strings.Contains(s, "log") {
		return "log-warning"
	}
	return "general-warning"
}

// report 将非空 err 的类别写入诊断日志，并把可显示的错误发送到前端。
func (a *Runtime) report(err error) {
	if err != nil {
		a.trace("event=operation-error kind=%s", logging.Error(err))
		a.AddRecentErrorCategory("error", classifyError(err))
		a.emit(model.Event{Type: "error", Text: err.Error()})
	}
}

func (a *Runtime) warn(text string) {
	a.mu.Lock()
	a.warnings = append(a.warnings, text)
	a.mu.Unlock()
	a.AddRecentErrorCategory("warn", classifyWarning(text))
	a.emit(model.Event{Type: "error", Text: text})
}

func (a *Runtime) GetStatus() model.Status {
	a.mu.Lock()
	v := model.Status{Platform: runtime.GOOS, Listening: a.listening, Busy: a.busy, DataDir: a.store.Dir(), Slides: append([]model.Slide{}, a.slides...), Warnings: append([]string{}, a.warnings...), Transcript: []model.Turn{}}
	a.mu.Unlock()
	if m := a.store.Current(); m != nil {
		v.Transcript = m.Turns
		if len(v.Transcript) > 200 {
			v.Transcript = v.Transcript[len(v.Transcript)-200:]
		}
	}
	return v
}

func (a *Runtime) status() { a.emit(model.Event{Type: "status", Data: a.GetStatus()}) }

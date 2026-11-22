package service

import (
	"context"
	"errors"
	"time"

	"cue-go-remix/internal/assistant"
	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/platform"
	"cue-go-remix/internal/provider"
	"cue-go-remix/internal/repository"
)

// Ask 使用 ask 指定的模式和问题异步生成回答；参数或模型配置无效、已有任务或正在退出时返回错误。
func (a *Runtime) Ask(ask model.AskRequest) error {
	if len([]rune(ask.Text)) > 30000 {
		return errors.New("问题过长，最多 30000 字")
	}
	s := a.store.Settings()
	status := a.GetStatus()
	if s.Chat.APIKey == "" && s.Chat.Provider != "custom" && s.Chat.Provider != "ollama" {
		return errors.New("请先在模型与中转设置中填写 API Key")
	}
	current := ""
	if m := a.store.Current(); m != nil {
		current = m.ID
	}
	req, screen, e := assistant.Build(s, ask, status.Transcript, a.store.Meetings(), current)
	if e != nil {
		return e
	}
	a.mu.Lock()
	if a.busy || a.closing {
		a.mu.Unlock()
		return errors.New("正在生成回答，请等待或停止当前回答")
	}
	a.busy = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.requestCancel = cancel
	a.wg.Add(1)
	a.mu.Unlock()
	id := repository.ID()
	a.trace("event=answer-start id=%s provider=%s protocol=%s stream=%t", id, logging.Provider(s.Chat.Provider), logging.ChatProtocol(s.Chat), s.Chat.Stream)
	labels := map[string]string{"assist": "智能辅助", "say": "我该怎么说？", "recap": "回顾当前会话", "leetcode": "解答屏幕上的代码题"}
	text := ask.Text
	if text == "" {
		text = labels[ask.Mode]
	}
	a.emit(model.Event{Type: "answer-start", ID: id, Text: text})
	a.status()
	go func() {
		defer a.wg.Done()
		defer cancel()
		started := time.Now()
		defer func() { a.mu.Lock(); a.busy = false; a.requestCancel = nil; a.mu.Unlock(); a.status() }()
		if screen {
			a.captureMu.Lock()
			req.Image, e = platform.Screenshot(ctx)
			a.captureMu.Unlock()
			if e != nil {
				a.trace("event=answer-failed id=%s stage=screenshot", id)
				a.AddRecentErrorCategory("error", "permission-denied")
				a.emit(model.Event{Type: "answer-error", ID: id, Text: "无法截取屏幕：" + e.Error()})
				return
			}
		}
		ep := s.Chat
		if s.Smart && ep.SmartModel != "" {
			ep.Model = ep.SmartModel
		}
		answer, err := provider.Chat(ctx, ep, req, func(token string) { a.emit(model.Event{Type: "answer-token", ID: id, Text: token}) }, func() { a.emit(model.Event{Type: "thinking", ID: id}) })
		if err != nil {
			a.trace("event=answer-failed id=%s stage=provider protocol=%s cancelled=%t kind=%s category=%s elapsed_ms=%d", id, logging.ChatProtocol(ep), ctx.Err() != nil, logging.Error(err), logging.ErrorCategory(err), time.Since(started).Milliseconds())
			if !errors.Is(ctx.Err(), context.Canceled) {
				a.AddRecentErrorCategory("error", classifyError(err))
			}
			msg := err.Error()
			if errors.Is(ctx.Err(), context.Canceled) {
				msg = "已停止生成"
			}
			a.emit(model.Event{Type: "answer-error", ID: id, Text: msg})
			return
		}
		a.trace("event=answer-done id=%s elapsed_ms=%d", id, time.Since(started).Milliseconds())
		a.emit(model.Event{Type: "answer-done", ID: id, Text: answer})
	}()
	return nil
}

// Cancel 取消当前回答任务；没有进行中的回答时不执行操作。
func (a *Runtime) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.requestCancel != nil {
		a.requestCancel()
	}
}

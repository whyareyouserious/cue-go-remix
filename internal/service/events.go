package service

import (
	"runtime"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
)

// emit 通过组装时传入的回调发布 e；回调为空时不发送事件。
func (a *Runtime) emit(e model.Event) {
	if a.emitEvent != nil {
		a.emitEvent(e)
	}
}

// Emit 向桌面层转发 e，业务层无需依赖 Wails。
func (a *Runtime) Emit(e model.Event) { a.emit(e) }

// Report 在 err 非空时记录错误类别并通知前端，日志不写入原始异常文本。
func (a *Runtime) Report(err error) { a.report(err) }

// Warn 保存 text 为运行警告并通知前端。
func (a *Runtime) Warn(text string) { a.warn(text) }

// report 将非空 err 的类别写入诊断日志，并把可显示的错误发送到前端。
func (a *Runtime) report(err error) {
	if err != nil {
		a.trace("event=operation-error kind=%s", logging.Error(err))
		a.emit(model.Event{Type: "error", Text: err.Error()})
	}
}

// warn 在锁内追加 text 警告，并通知前端刷新提示。
func (a *Runtime) warn(text string) {
	a.mu.Lock()
	a.warnings = append(a.warnings, text)
	a.mu.Unlock()
	a.emit(model.Event{Type: "error", Text: text})
}

// GetStatus 返回平台、任务状态及最近最多 200 条转录的快照，不包含 API Key。
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

// status 发布当前业务状态快照。
func (a *Runtime) status() { a.emit(model.Event{Type: "status", Data: a.GetStatus()}) }

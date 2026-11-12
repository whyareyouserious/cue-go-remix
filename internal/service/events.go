package service

import (
	"runtime"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
)

func (a *Runtime) emit(e model.Event) {
	if a.emitEvent != nil {
		a.emitEvent(e)
	}
}
func (a *Runtime) Emit(e model.Event) { a.emit(e) }
func (a *Runtime) Report(err error)   { a.report(err) }
func (a *Runtime) Warn(text string)   { a.warn(text) }
func (a *Runtime) report(err error) {
	if err != nil {
		a.trace("event=operation-error kind=%s", logging.Error(err))
		a.emit(model.Event{Type: "error", Text: err.Error()})
	}
}

func (a *Runtime) warn(text string) {
	a.mu.Lock()
	a.warnings = append(a.warnings, text)
	a.mu.Unlock()
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

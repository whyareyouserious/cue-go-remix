package controller

import (
	"cue-go-remix/internal/model"
)

func (a *App) RecordUIEvent(event, value string) { a.runtime.RecordUIEvent(event, value) }
func (a *App) GetStatus() model.Status           { return a.runtime.GetStatus() }

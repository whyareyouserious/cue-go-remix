package controller

import (
	"cue-go-remix/internal/model"
)

func (a *App) Ask(ask model.AskRequest) error { return a.runtime.Ask(ask) }
func (a *App) Cancel()                        { a.runtime.Cancel() }

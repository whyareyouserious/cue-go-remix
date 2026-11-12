package controller

import (
	"cue-go-remix/internal/applink"
)

func (a *App) Consent(id string, allowed bool) { a.runtime.Consent(id, allowed) }
func (a *App) ListCallers() []applink.Caller   { return a.runtime.ListCallers() }
func (a *App) RevokeCaller(id string) error    { return a.runtime.RevokeCaller(id) }

package controller

import (
	"cue-go-remix/internal/model"
)

func (a *App) SetCollapsed(value bool) { a.desktop.SetCollapsed(value) }
func (a *App) SetOverlayLayout(width, height int, regions []model.OverlayRegion) model.OverlayViewport {
	return a.desktop.SetOverlayLayout(width, height, regions)
}
func (a *App) SetPassthrough(value bool) error { return a.desktop.SetPassthrough(value) }
func (a *App) Quit()                           { a.desktop.Quit() }

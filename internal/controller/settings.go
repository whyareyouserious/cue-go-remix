package controller

import (
	"cue-go-remix/internal/model"
)

func (a *App) GetSettings() model.Settings { return a.runtime.GetSettings() }
func (a *App) SaveSettings(s model.Settings) (model.Settings, error) {
	return a.runtime.SaveSettings(s)
}

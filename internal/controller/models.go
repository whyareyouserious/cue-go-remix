package controller

import (
	"cue-go-remix/internal/model"
)

func (a *App) ListModels() []model.ModelInfo { return a.runtime.ListModels() }
func (a *App) DownloadModel(id string) error { return a.runtime.DownloadModel(id) }
func (a *App) CancelDownload(id string)      { a.runtime.CancelDownload(id) }
func (a *App) DeleteModel(id string) error   { return a.runtime.DeleteModel(id) }
func (a *App) ImportModel(id string) error {
	path, err := a.desktop.ImportModelPath()
	if err != nil || path == "" {
		return err
	}
	return a.runtime.ImportModel(id, path)
}

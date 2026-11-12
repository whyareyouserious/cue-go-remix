package service

import (
	"cue-go-remix/internal/model"
)

func (a *Runtime) ListModels() []model.ModelInfo { return a.models.List() }

func (a *Runtime) DownloadModel(id string) error { return a.models.Download(a.ctx, id) }

func (a *Runtime) CancelDownload(id string) { a.models.Cancel(id) }

func (a *Runtime) DeleteModel(id string) error       { return a.models.Delete(id) }
func (a *Runtime) ImportModel(id, path string) error { return a.models.Import(id, path) }

package service

import (
	"cue-go-remix/internal/model"
)

// ListModels 返回可用 Whisper 模型及其安装、下载进度信息。
func (a *Runtime) ListModels() []model.ModelInfo { return a.models.List() }

// DownloadModel 按 id 开始下载 Whisper 模型；未知模型或下载初始化失败时返回错误。
func (a *Runtime) DownloadModel(id string) error { return a.models.Download(a.ctx, id) }

// CancelDownload 按 id 取消正在进行的模型下载，已下载部分按模型管理模块规则保留。
func (a *Runtime) CancelDownload(id string) { a.models.Cancel(id) }

// DeleteModel 按 id 删除本机模型文件，失败时返回模型管理模块的错误。
func (a *Runtime) DeleteModel(id string) error { return a.models.Delete(id) }

// ImportModel 按 id 校验并导入 path 指定的模型文件；文件不符合模型校验或写入失败时返回错误。
func (a *Runtime) ImportModel(id, path string) error { return a.models.Import(id, path) }

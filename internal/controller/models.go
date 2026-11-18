package controller

import (
	"cue-go-remix/internal/model"
)

// ListModels 返回可用 Whisper 模型及其安装、下载进度信息。
func (a *App) ListModels() []model.ModelInfo { return a.runtime.ListModels() }

// DownloadModel 按 id 开始下载 Whisper 模型；未知模型或下载初始化失败时返回错误。
func (a *App) DownloadModel(id string) error { return a.runtime.DownloadModel(id) }

// CancelDownload 按 id 取消正在进行的模型下载，已下载部分按模型管理模块规则保留。
func (a *App) CancelDownload(id string) { a.runtime.CancelDownload(id) }

// DeleteModel 按 id 删除本机模型文件，失败时返回模型管理模块的错误。
func (a *App) DeleteModel(id string) error { return a.runtime.DeleteModel(id) }

// ImportModel 将用户选定的本地模型导入 id 对应位置；选择取消视为正常结束，校验或写入失败返回错误。
func (a *App) ImportModel(id string) error {
	path, err := a.desktop.ImportModelPath()
	if err != nil || path == "" {
		return err
	}
	return a.runtime.ImportModel(id, path)
}

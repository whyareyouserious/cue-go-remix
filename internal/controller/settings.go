package controller

import (
	"cue-go-remix/internal/model"
)

// GetSettings 返回面向前端的设置副本，隐藏 API Key 并通过 HasKey 标记是否已配置。
func (a *App) GetSettings() model.Settings { return a.runtime.GetSettings() }

// SaveSettings 校验并持久化 s，返回隐藏密钥后的设置；录音期间拒绝切换音频配置，写入失败返回错误。
func (a *App) SaveSettings(s model.Settings) (model.Settings, error) {
	return a.runtime.SaveSettings(s)
}

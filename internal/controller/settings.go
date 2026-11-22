package controller

import (
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
)

func (a *App) GetSettings() model.Settings { return a.runtime.GetSettings() }
func (a *App) SaveSettings(s model.Settings) (model.Settings, error) {
	return a.runtime.SaveSettings(s)
}

// TestEndpoint 在授权有效时探测聊天或语音服务；不持久化设置，不自动录音，授权失效取消检测。
func (a *App) TestEndpoint(kind string, ep model.Endpoint) (model.EndpointCheckResult, error) {
	return a.runtime.CheckEndpoint(kind, ep)
}

// TestChannelEndpoint 在授权有效时检测账号草稿，按同类账号 ID 查找已存密钥；不保存配置或录音，授权失效取消检测。
func (a *App) TestChannelEndpoint(kind string, channel model.APIChannel) (model.EndpointCheckResult, error) {
	return a.runtime.CheckChannelEndpoint(kind, channel)
}

// GetChannelKey 供用户点击复制或显示时读取指定已存账号凭据；未知类型、账号或缺少密钥时返回错误，禁止记录返回值。
func (a *App) GetChannelKey(kind, id string) (string, error) {
	return a.runtime.GetChannelKey(kind, id)
}

// UpdateShortcuts 校验并重注册全局快捷键；注册或持久化失败均回滚旧配置，回滚失败绝不谎称已恢复。
func (a *App) UpdateShortcuts(shortcuts map[string]string) (model.Settings, error) {
	var latest model.Settings
	err := a.desktop.UpdateShortcuts(shortcuts, func() error {
		var saveErr error
		latest, saveErr = a.runtime.SaveShortcuts(shortcuts)
		return saveErr
	})
	if err != nil {
		return a.runtime.GetSettings(), err
	}
	return latest, nil
}

// ResetShortcuts 恢复为默认快捷键映射；注册或持久化失败均执行回滚。
func (a *App) ResetShortcuts() (model.Settings, error) {
	defaults := repository.DefaultSettings().Shortcuts
	var latest model.Settings
	err := a.desktop.UpdateShortcuts(defaults, func() error {
		var saveErr error
		latest, saveErr = a.runtime.SaveShortcuts(defaults)
		return saveErr
	})
	if err != nil {
		return a.runtime.GetSettings(), err
	}
	return latest, nil
}

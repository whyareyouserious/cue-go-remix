package controller

import (
	"cue-go-remix/internal/model"
)

// SetCollapsed 按 value 切换宿主窗口的收起状态，同时更新默认命中区域。
func (a *App) SetCollapsed(value bool) { a.desktop.SetCollapsed(value) }

// SetOverlayLayout 按逻辑像素尺寸及 regions 更新窗口与命中区域，返回屏幕可用尺寸；无效尺寸返回空视口。
func (a *App) SetOverlayLayout(width, height int, regions []model.OverlayRegion) model.OverlayViewport {
	return a.desktop.SetOverlayLayout(width, height, regions)
}

// SetPassthrough 按 value 设置鼠标穿透；恢复交互的全局快捷键未就绪时拒绝开启并返回错误。
func (a *App) SetPassthrough(value bool) error { return a.desktop.SetPassthrough(value) }

// Quit 请求桌面应用正常退出，后台任务由关闭流程取消。
func (a *App) Quit() { a.desktop.Quit() }

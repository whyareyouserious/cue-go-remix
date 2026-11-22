package controller

import (
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/service"
)

// App 是 Wails 绑定入口；业务状态由 Runtime 持有，桌面操作由 Desktop 实现。
type App struct {
	runtime *service.Runtime
	desktop Desktop
}
type Desktop interface {
	SetCollapsed(bool)
	SetOverlayLayout(int, int, []model.OverlayRegion) model.OverlayViewport
	SetPassthrough(bool) error
	Quit()
	ExportMeetingPath(string) (string, error)
	// ExportDiagnosticPath 选择诊断报告文件保存路径；取消为空路径，失败返回错误。
	ExportDiagnosticPath(string) (string, error)
	// ImportModelPath 选择模型文件；取消为空路径，失败返回错误。
	ImportModelPath() (string, error)
	// UpdateShortcuts 更新并重新注册全局快捷键，并在 persistFn 失败时回滚；回滚失败绝不误报已恢复。
	UpdateShortcuts(map[string]string, func() error) error
}

func New(runtime *service.Runtime, desktop Desktop) *App {
	return &App{runtime: runtime, desktop: desktop}
}

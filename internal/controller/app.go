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
	ImportModelPath() (string, error)
}

func New(runtime *service.Runtime, desktop Desktop) *App {
	return &App{runtime: runtime, desktop: desktop}
}

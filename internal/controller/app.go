package controller

import (
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/service"
)

// App 是 Wails 绑定入口；业务状态由 Runtime 持有，桌面操作由 Desktop 实现。
type App struct {
	// runtime 共享业务运行时；使用命名字段避免暴露其内部方法。
	runtime *service.Runtime
	// desktop 窗口和文件对话框能力，由 desktop.Host 实现。
	desktop Desktop
}

// Desktop 定义控制层需要的桌面能力；生产实现为 desktop.Host，测试替换对话框。
type Desktop interface {
	// SetCollapsed 切换窗口收起状态。
	SetCollapsed(bool)
	// SetOverlayLayout 更新逻辑像素布局并返回屏幕工作区范围。
	SetOverlayLayout(int, int, []model.OverlayRegion) model.OverlayViewport
	// SetPassthrough 设置鼠标穿透；无法保证恢复交互时返回错误。
	SetPassthrough(bool) error
	// Quit 请求桌面应用正常退出。
	Quit()
	// ExportMeetingPath 选择默认名称对应的导出路径；取消为空路径，失败返回错误。
	ExportMeetingPath(string) (string, error)
	// ImportModelPath 选择模型文件；取消为空路径，失败返回错误。
	ImportModelPath() (string, error)
}

// New 使用 runtime 处理业务，以 desktop 完成窗口和文件选择；返回只显式暴露前端方法的控制器。
func New(runtime *service.Runtime, desktop Desktop) *App {
	return &App{runtime: runtime, desktop: desktop}
}

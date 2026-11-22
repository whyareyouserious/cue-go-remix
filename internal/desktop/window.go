package desktop

import (
	"errors"

	"cue-go-remix/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// createWindow 根据内部设置创建透明悬浮窗口，窗口保护由平台能力决定。
func (a *Host) createWindow() *application.WebviewWindow {
	s := a.runtime.Settings()
	return a.desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "cue", Title: "Cue", URL: "/",
		Width: 700, Height: 68, MinWidth: 480, MinHeight: 64,
		DisableResize: true, Frameless: true, AlwaysOnTop: true,
		BackgroundType:           application.BackgroundTypeTransparent,
		ContentProtectionEnabled: s.ContentProtection && protectionAvailable(),
		Mac: application.MacWindow{
			Backdrop:           application.MacBackdropTransparent,
			DisableShadow:      true,
			WindowClass:        application.MacWindowClassPanel,
			PanelPreferences:   application.MacPanelPreferences{FloatingPanel: true, NonActivating: true},
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces | application.MacWindowCollectionBehaviorFullScreenAuxiliary,
		},
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
	})
}

// SetCollapsed 按 value 切换宿主窗口的收起状态，同时更新默认命中区域。
func (a *Host) SetCollapsed(value bool) {
	a.mu.Lock()
	a.collapsed = value
	a.mu.Unlock()
	if value {
		a.SetOverlayLayout(700, 68, []model.OverlayRegion{{Width: 700, Height: 68}})
	} else {
		a.SetOverlayLayout(700, 320, []model.OverlayRegion{{Width: 700, Height: 320}})
	}
}

// SetPassthrough 按 value 设置鼠标穿透；恢复交互的全局快捷键未就绪时拒绝开启并返回错误。
func (a *Host) SetPassthrough(value bool) error {
	a.mu.Lock()
	ready := a.passthroughReady
	a.mu.Unlock()
	if value && !ready {
		return errors.New("恢复鼠标交互的全局快捷键注册失败，无法开启穿透")
	}
	a.mu.Lock()
	a.manualPassthrough = value
	a.mu.Unlock()
	a.window.SetIgnoreMouseEvents(value)
	return nil
}

// Quit 请求桌面应用正常退出，后台任务由关闭流程取消。
func (a *Host) Quit() {
	a.trace("event=quit-request")
	a.desktop.Quit()
}

// settingsChanged 将 settings 中的录屏保护设置派发到 UI 线程，应用关闭后忽略窗口更新。
func (a *Host) settingsChanged(settings model.Settings) {
	if a.window == nil {
		return
	}
	application.InvokeAsync(func() {
		a.mu.Lock()
		closing := a.closing
		a.mu.Unlock()
		if !closing {
			a.window.SetContentProtection(settings.ContentProtection && protectionAvailable())
		}
	})
}

// ExportMeetingPath 打开默认文件名为 name 的保存对话框；取消返回空路径，系统对话框失败返回错误。
func (a *Host) ExportMeetingPath(name string) (string, error) {
	return a.desktop.Dialog.SaveFile().SetFilename(name).AddFilter("Markdown", "*.md").PromptForSingleSelection()
}

// ExportDiagnosticPath 打开默认文件名为 name 的诊断文件保存对话框；取消返回空路径，失败返回错误。
func (a *Host) ExportDiagnosticPath(name string) (string, error) {
	return a.desktop.Dialog.SaveFile().SetFilename(name).AddFilter("JSON 诊断文件", "*.json").PromptForSingleSelection()
}

// ImportModelPath 选择 Whisper GGML 模型路径；取消返回空路径，系统对话框失败返回错误。
func (a *Host) ImportModelPath() (string, error) {
	return a.desktop.Dialog.OpenFile().AddFilter("Whisper GGML 模型", "*.bin").PromptForSingleSelection()
}

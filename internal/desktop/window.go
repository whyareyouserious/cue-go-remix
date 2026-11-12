package desktop

import (
	"errors"

	"cue-go-remix/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

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
func (a *Host) Quit() {
	a.trace("event=quit-request")
	a.desktop.Quit()
}

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
func (a *Host) ExportMeetingPath(name string) (string, error) {
	return a.desktop.Dialog.SaveFile().SetFilename(name).AddFilter("Markdown", "*.md").PromptForSingleSelection()
}
func (a *Host) ImportModelPath() (string, error) {
	return a.desktop.Dialog.OpenFile().AddFilter("Whisper GGML 模型", "*.bin").PromptForSingleSelection()
}

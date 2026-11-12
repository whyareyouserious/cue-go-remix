package desktop

import (
	"context"
	"io/fs"
	"sync"

	"cue-go-remix/internal/config"
	"cue-go-remix/internal/controller"
	"cue-go-remix/internal/entitlement"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Host 仅管理桌面窗口、快捷键与主线程事件。
type Host struct {
	runtime           *service.Runtime
	desktop           *application.App
	window            *application.WebviewWindow
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	wg                sync.WaitGroup
	closing           bool
	collapsed         bool
	passthroughReady  bool
	manualPassthrough bool
	overlayRegions    []model.OverlayRegion
}

func Run(assets fs.FS) error {
	dir, err := config.DataDir()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Host{ctx: ctx, cancel: cancel}
	url, allowHTTP := config.LicenseEndpoint()
	a.runtime, err = service.New(service.Options{DataDir: dir, LicenseURL: url, AllowHTTP: allowHTTP, Emit: a.emit, SettingsChanged: a.settingsChanged})
	if err != nil {
		cancel()
		return err
	}
	desktop := application.New(application.Options{
		Name: "Cue", Description: "屏幕与会议 AI 助手",
		Services: []application.Service{application.NewService(controller.New(a.runtime, a))},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		// NSPanel 不属于主窗口。WebKit 的临时选择菜单关闭时，不能连带退出悬浮助手。
		Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory, ApplicationShouldTerminateAfterLastWindowClosed: false},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.cue.remix", OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
			if a.window != nil {
				_ = a.SetPassthrough(false)
				a.window.Show()
			}
		}},
		OnShutdown: a.shutdown, ErrorHandler: func(err error) { a.runtime.Warn(err.Error()) },
	})
	a.desktop = desktop
	a.window = a.createWindow()
	desktop.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		a.positionOverlay()
		a.startup()
	})
	desktop.Event.OnApplicationEvent(events.Common.SystemWillSleep, func(_ *application.ApplicationEvent) { a.runtime.RevalidateLicense() })
	desktop.Event.OnApplicationEvent(events.Common.SystemDidWake, func(_ *application.ApplicationEvent) { a.runtime.RevalidateLicense() })
	a.window.OnWindowEvent(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		x, y := a.window.Position()
		a.runtime.Report(a.runtime.SavePosition(x, y))
	})
	defer a.shutdown()
	return desktop.Run()
}
func (a *Host) shutdown() {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return
	}
	a.closing = true
	a.mu.Unlock()
	a.cancel()
	a.runtime.Shutdown()
	a.wg.Wait()
}
func (a *Host) startup() {
	a.runtime.Start()
	a.registerShortcuts()
	if !protectionAvailable() {
		a.runtime.Warn("当前系统或远程桌面会话无法启用录屏排除")
	}
	a.startMouseTracking()
}
func (a *Host) trace(format string, args ...any) { a.runtime.Trace(format, args...) }
func (a *Host) emit(event model.Event) {
	reveal := event.Type == "consent"
	if status, ok := event.Data.(entitlement.Status); event.Type == "license-status" && ok && !status.Allowed {
		reveal = true
	}
	if reveal && a.window != nil {
		application.InvokeAsync(func() {
			a.mu.Lock()
			closing := a.closing
			a.manualPassthrough = false
			a.mu.Unlock()
			if !closing {
				a.window.SetIgnoreMouseEvents(false)
				a.window.Show()
			}
		})
	}
	if a.desktop != nil {
		a.desktop.Event.Emit("cue:event", event)
	}
}

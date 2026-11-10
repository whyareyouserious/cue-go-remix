package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var frontend embed.FS

func main() {
	a, err := newApp()
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(frontend, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	desktop := application.New(application.Options{
		Name: "Cue", Description: "屏幕与会议 AI 助手",
		Services: []application.Service{application.NewService(a)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		// NSPanel 不属于主窗口。WebKit 的临时选择菜单关闭时，不能连带退出悬浮助手。
		Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory, ApplicationShouldTerminateAfterLastWindowClosed: false},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.cue.remix", OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
			if a.window != nil {
				_ = a.SetPassthrough(false)
				a.window.Show()
			}
		}},
		OnShutdown: a.shutdown, ErrorHandler: func(err error) { a.warn(err.Error()) },
	})
	a.desktop = desktop
	s := a.store.Settings()
	a.window = desktop.Window.NewWithOptions(application.WebviewWindowOptions{
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
	desktop.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		a.positionOverlay()
		a.startup()
	})
	a.window.OnWindowEvent(events.Common.WindowDidMove, func(_ *application.WindowEvent) {
		x, y := a.window.Position()
		a.report(a.store.SavePosition(x, y))
	})
	if err = desktop.Run(); err != nil {
		log.Fatal(err)
	}
}

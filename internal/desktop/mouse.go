package desktop

import (
	"runtime"
	"time"

	"cue-go-remix/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// startMouseTracking 每 75 毫秒向 UI 线程派发命中检查；透明区域穿透，关闭时通过上下文结束后台循环。
func (a *Host) startMouseTracking() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(75 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				application.InvokeAsync(func() {
					a.mu.Lock()
					closing, manual := a.closing, a.manualPassthrough
					a.mu.Unlock()
					if closing || manual {
						return
					}
					cx, cy, err := platform.CursorPosition()
					if err != nil {
						return
					}
					p := application.Point{X: cx, Y: cy}
					if runtime.GOOS == "windows" {
						p = a.desktop.Screen.PhysicalToDipPoint(p)
					}
					x, y := a.window.Position()
					a.mu.Lock()
					inside := overlayContains(a.overlayRegions, p.X-x, p.Y-y)
					a.mu.Unlock()
					a.window.SetIgnoreMouseEvents(!inside)
				})
			}
		}
	}()
}

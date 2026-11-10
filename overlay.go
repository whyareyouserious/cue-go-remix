package main

import "github.com/wailsapp/wails/v3/pkg/application"

// OverlayRegion 是前端实际可见的控件区域，坐标相对于透明窗口左上角。
// 窗口中控件之外的透明部分不应阻挡其他应用。
type OverlayRegion struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type OverlayViewport struct {
	MaxWidth  int `json:"maxWidth"`
	MaxHeight int `json:"maxHeight"`
}

func overlayContains(regions []OverlayRegion, x, y int) bool {
	for _, r := range regions {
		if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
			return true
		}
	}
	return false
}

func fitOverlay(bounds, area application.Rect) application.Rect {
	bounds.Width = min(bounds.Width, area.Width)
	bounds.Height = min(bounds.Height, area.Height)
	bounds.X = max(area.X, min(bounds.X, area.X+area.Width-bounds.Width))
	bounds.Y = max(area.Y, min(bounds.Y, area.Y+area.Height-bounds.Height))
	return bounds
}

// SetOverlayLayout 随可见内容调整宿主窗口，主列左侧保持不动，历史在右侧展开。
// 窗口操作统一在主线程中执行，以免与鼠标命中检查并发访问 Wails 窗口。
func (a *App) SetOverlayLayout(width, height int, regions []OverlayRegion) OverlayViewport {
	if a.window == nil || width <= 0 || height <= 0 {
		return OverlayViewport{}
	}
	var viewport OverlayViewport
	application.InvokeSync(func() {
		a.mu.Lock()
		if a.closing {
			a.mu.Unlock()
			return
		}
		a.overlayRegions = append([]OverlayRegion(nil), regions...)
		a.mu.Unlock()
		x, y := a.window.Position()
		bounds := application.Rect{X: x, Y: y, Width: width, Height: height}
		if screen, err := a.window.GetScreen(); err == nil && screen != nil {
			bounds = fitOverlay(bounds, screen.WorkArea)
			viewport = OverlayViewport{MaxWidth: screen.WorkArea.Width, MaxHeight: screen.WorkArea.Height}
		}
		w, h := a.window.Size()
		if w != bounds.Width || h != bounds.Height {
			a.trace("event=window-size width=%d height=%d regions=%d", bounds.Width, bounds.Height, len(regions))
			a.window.SetSize(bounds.Width, bounds.Height)
		}
		// macOS 调整高度时也可能移动窗口，显式保持原来的顶部位置。
		if px, py := a.window.Position(); px != bounds.X || py != bounds.Y {
			a.window.SetPosition(bounds.X, bounds.Y)
		}
	})
	return viewport
}

func (a *App) positionOverlay() {
	s := a.store.Settings()
	screen := a.desktop.Screen.GetPrimary()
	if screen == nil {
		return
	}
	w, h := a.window.Size()
	bounds := application.Rect{X: screen.WorkArea.X + (screen.WorkArea.Width-w)/2, Y: screen.WorkArea.Y + 6, Width: w, Height: h}
	if s.WindowPositioned {
		for _, candidate := range a.desktop.Screen.GetAll() {
			r := candidate.WorkArea
			if s.WindowX >= r.X && s.WindowX < r.X+r.Width && s.WindowY >= r.Y && s.WindowY < r.Y+r.Height {
				screen = candidate
				bounds.X, bounds.Y = s.WindowX, s.WindowY
				break
			}
		}
	}
	bounds = fitOverlay(bounds, screen.WorkArea)
	a.window.SetPosition(bounds.X, bounds.Y)
}

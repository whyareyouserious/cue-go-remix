package desktop

import (
	"cue-go-remix/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// overlayContains 判断逻辑像素坐标 x、y 是否位于 regions 中任一可见区域，右下边界不包含在内。
func overlayContains(regions []model.OverlayRegion, x, y int) bool {
	for _, r := range regions {
		if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
			return true
		}
	}
	return false
}

// fitOverlay 将 bounds 的尺寸和位置限制在屏幕工作区 area 内，返回调整后的矩形。
func fitOverlay(bounds, area application.Rect) application.Rect {
	bounds.Width = min(bounds.Width, area.Width)
	bounds.Height = min(bounds.Height, area.Height)
	bounds.X = max(area.X, min(bounds.X, area.X+area.Width-bounds.Width))
	bounds.Y = max(area.Y, min(bounds.Y, area.Y+area.Height-bounds.Height))
	return bounds
}

// currentNormalizedScreen 获取当前窗口所在屏幕的归一化逻辑点屏幕信息。
// 直接调用 a.window.GetScreen() 在 macOS 上会返回物理像素尺寸（绕过了 ScreenManager 的归一化），
// 因此优先通过 screen.ID 从 a.desktop.Screen.GetByID 取回已归一化屏幕；若不存在则合理回退至 a.desktop.Screen.GetPrimary()。
func (a *Host) currentNormalizedScreen() *application.Screen {
	if a.desktop == nil || a.desktop.Screen == nil {
		return nil
	}
	if a.window != nil {
		raw, err := a.window.GetScreen()
		if err == nil && raw != nil && raw.ID != "" {
			if s := a.desktop.Screen.GetByID(raw.ID); s != nil {
				return s
			}
		}
	}
	return a.desktop.Screen.GetPrimary()
}

// SetOverlayLayout 按逻辑像素尺寸及 regions 更新窗口与命中区域，返回屏幕可用尺寸；无效尺寸返回空视口。
// 当 width == -1 && height == -1 时，表示进入全屏工作区透明画布模式，窗口扩展至整块屏幕工作区（包括负坐标支持）；
// 恢复紧凑模式时，窗口精确还原到用户原本放置的屏幕物理位置。
func (a *Host) SetOverlayLayout(width, height int, regions []model.OverlayRegion) model.OverlayViewport {
	if a.window == nil {
		return model.OverlayViewport{}
	}
	var viewport model.OverlayViewport
	application.InvokeSync(func() {
		a.mu.Lock()
		if a.closing {
			a.mu.Unlock()
			return
		}
		a.overlayRegions = append([]model.OverlayRegion(nil), regions...)
		a.mu.Unlock()

		screen := a.currentNormalizedScreen()

		// 检查是否请求全屏工作区透明画布（活动弹窗模式）
		if width == -1 && height == -1 {
			if screen != nil {
				a.mu.Lock()
				if !a.temporaryFullscreen {
					curX, curY := a.window.Position()
					a.savedWindowPos = application.Point{X: curX, Y: curY}
					a.temporaryFullscreen = true
				}
				origPos := a.savedWindowPos
				a.mu.Unlock()

				// 移动至真实 WorkArea 原点（含副屏负坐标）并覆盖整个工作区尺寸
				wa := screen.WorkArea
				curW, curH := a.window.Size()
				if curW != wa.Width || curH != wa.Height {
					a.window.SetSize(wa.Width, wa.Height)
				}
				curX, curY := a.window.Position()
				if curX != wa.X || curY != wa.Y {
					a.window.SetPosition(wa.X, wa.Y)
				}

				viewport = model.OverlayViewport{
					MaxWidth:  wa.Width,
					MaxHeight: wa.Height,
					WorkAreaX: wa.X,
					WorkAreaY: wa.Y,
					WindowX:   origPos.X,
					WindowY:   origPos.Y,
				}
			}
			return
		}

		// 退出全屏 / 紧凑模式
		if width <= 0 || height <= 0 {
			return
		}

		a.mu.Lock()
		wasFullscreen := a.temporaryFullscreen
		origPos := a.savedWindowPos
		// 严禁在此处过早清空 temporaryFullscreen！必须在 SetSize 与 SetPosition 完全复位后再解除标记，
		// 以免复位过程派发的 WindowDidMove 事件误将过渡期位置写入持久化配置。
		a.mu.Unlock()

		var targetX, targetY int
		if wasFullscreen {
			targetX, targetY = origPos.X, origPos.Y
		} else {
			targetX, targetY = a.window.Position()
		}

		bounds := application.Rect{X: targetX, Y: targetY, Width: width, Height: height}
		if screen != nil {
			bounds = fitOverlay(bounds, screen.WorkArea)
			viewport = model.OverlayViewport{
				MaxWidth:  screen.WorkArea.Width,
				MaxHeight: screen.WorkArea.Height,
				WorkAreaX: screen.WorkArea.X,
				WorkAreaY: screen.WorkArea.Y,
				WindowX:   bounds.X,
				WindowY:   bounds.Y,
			}
		}

		w, h := a.window.Size()
		if w != bounds.Width || h != bounds.Height {
			a.trace("event=window-size width=%d height=%d regions=%d", bounds.Width, bounds.Height, len(regions))
			a.window.SetSize(bounds.Width, bounds.Height)
		}
		if px, py := a.window.Position(); px != bounds.X || py != bounds.Y {
			a.window.SetPosition(bounds.X, bounds.Y)
		}

		// 尺寸与物理位置在窗口系统层完全还原后，在主线程安全解除幕布标记
		if wasFullscreen {
			a.mu.Lock()
			a.temporaryFullscreen = false
			a.mu.Unlock()
		}
	})
	return viewport
}

// positionOverlay 恢复已保存的窗口位置；显示器不存在时使用主屏幕并限制在工作区内。
func (a *Host) positionOverlay() {
	s := a.runtime.Settings()
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

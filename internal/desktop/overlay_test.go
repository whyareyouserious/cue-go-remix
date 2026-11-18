package desktop

import (
	"testing"

	"cue-go-remix/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestOverlayTransparentSpaceDoesNotBlockOtherApps(t *testing.T) {
	regions := []model.OverlayRegion{
		{X: 140, Y: 14, Width: 420, Height: 40},
		{X: 38, Y: 66, Width: 624, Height: 230},
		{X: 674, Y: 66, Width: 300, Height: 230},
	}
	for _, p := range []application.Point{{X: 150, Y: 20}, {X: 40, Y: 100}, {X: 700, Y: 100}} {
		if !overlayContains(regions, p.X, p.Y) {
			t.Errorf("可见控件无法点击：%+v", p)
		}
	}
	for _, p := range []application.Point{{X: 20, Y: 20}, {X: 400, Y: 60}, {X: 670, Y: 120}, {X: 400, Y: 320}} {
		if overlayContains(regions, p.X, p.Y) {
			t.Errorf("透明区域阻挡其他窗口：%+v", p)
		}
	}
	if overlayContains(regions[:1], 400, 100) {
		t.Fatal("折叠之后原回答区仍然拦截鼠标")
	}
}

func TestOverlayExpandsWithinMonitorWorkArea(t *testing.T) {
	area := application.Rect{X: -1440, Y: 25, Width: 1440, Height: 875}
	got := fitOverlay(application.Rect{X: -700, Y: 800, Width: 1000, Height: 608}, area)
	want := application.Rect{X: -1000, Y: 292, Width: 1000, Height: 608}
	if got != want {
		t.Fatalf("展开后设置或侧栏超出副屏：得到 %+v，期望 %+v", got, want)
	}
	compact := fitOverlay(application.Rect{X: -1100, Y: 31, Width: 700, Height: 68}, area)
	if compact.X != -1100 || compact.Y != 31 {
		t.Fatalf("屏内悬浮条不应改变位置：%+v", compact)
	}
}

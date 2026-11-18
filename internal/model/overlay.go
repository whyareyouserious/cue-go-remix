package model

// OverlayRegion 是前端实际可见的控件区域，坐标相对于透明窗口左上角。
// 窗口中控件之外的透明部分不应阻挡其他应用。
type OverlayRegion struct {
	// X 相对于窗口左上角的横坐标，逻辑像素。
	X int `json:"x"`
	// Y 相对于窗口左上角的纵坐标，逻辑像素。
	Y int `json:"y"`
	// Width 命中区域宽度，逻辑像素。
	Width int `json:"width"`
	// Height 命中区域高度，逻辑像素。
	Height int `json:"height"`
}

// OverlayViewport 向前端返回当前屏幕工作区的可用尺寸。
type OverlayViewport struct {
	// MaxWidth 最大可用宽度，逻辑像素。
	MaxWidth int `json:"maxWidth"`
	// MaxHeight 最大可用高度，逻辑像素。
	MaxHeight int `json:"maxHeight"`
}

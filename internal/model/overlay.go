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

// OverlayViewport 向前端返回当前屏幕工作区的可用尺寸及物理位置坐标。
type OverlayViewport struct {
	// MaxWidth 最大可用宽度，逻辑像素。
	MaxWidth int `json:"maxWidth"`
	// MaxHeight 最大可用高度，逻辑像素。
	MaxHeight int `json:"maxHeight"`
	// WorkAreaX 当前屏幕工作区的横坐标原点（逻辑像素，支持副屏负坐标）。
	WorkAreaX int `json:"workAreaX"`
	// WorkAreaY 当前屏幕工作区的纵坐标原点（逻辑像素，支持副屏负坐标）。
	WorkAreaY int `json:"workAreaY"`
	// WindowX 悬浮菜单条窗口在屏幕上的原本物理横坐标。
	WindowX int `json:"windowX"`
	// WindowY 悬浮菜单条窗口在屏幕上的原本物理纵坐标。
	WindowY int `json:"windowY"`
}

package model

// OverlayRegion 是前端实际可见的控件区域，坐标相对于透明窗口左上角。
// 窗口中控件之外的透明部分不应阻挡其他应用。
type OverlayRegion struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// OverlayViewport 向前端返回当前屏幕工作区的可用尺寸及物理位置坐标。
type OverlayViewport struct {
	MaxWidth  int `json:"maxWidth"`
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

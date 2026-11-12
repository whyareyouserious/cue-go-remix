package model

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

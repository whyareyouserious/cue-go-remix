package model

// Event 定义 cue:event 的统一封装，按 Type 决定可选字段的含义。
type Event struct {
	// Type 种类，如 status、transcript、answer-token、error、shortcut 或 consent。
	Type string `json:"type"`
	// ID 回答或权限请求的关联标识；无关联对象时省略。
	ID string `json:"id,omitempty"`
	// Text 事件正文或固定操作名，含义由 Type 决定。
	Text string `json:"text,omitempty"`
	// Channel 音频事件来源；不适用时省略。
	Channel string `json:"channel,omitempty"`
	// Data 对应事件类型的结构化载荷，如 Status、Turn 或下载进度。
	Data any `json:"data,omitempty"`
}

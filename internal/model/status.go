package model

// Status 是面向前端的运行态快照，不包含密钥或原始设置。
type Status struct {
	// Platform 操作系统标识，如 darwin 或 windows。
	Platform string `json:"platform"`
	// Listening 音频会话是否运行中。
	Listening bool `json:"listening"`
	// Busy 是否有正在生成的回答。
	Busy bool `json:"busy"`
	// DataDir 配置、会议、模型及日志所在的本机目录。
	DataDir string `json:"dataDir"`
	// Transcript 最近最多 200 条最终转录。
	Transcript []Turn `json:"transcript"`
	// Slides 当前会话最多 50 条页面摘要。
	Slides []Slide `json:"slides"`
	// Warnings 待展示的运行警告。
	Warnings []string `json:"warnings"`
}

// RecentError 包含运行时脱敏分类快照，绝不包含原始异常正文、密钥、转录正文或个人资料。
type RecentError struct {
	ID       string `json:"id"`
	Time     int64  `json:"time"`
	Level    string `json:"level"`    // "error" | "warn"
	Category string `json:"category"` // 固定脱敏分类代码
}

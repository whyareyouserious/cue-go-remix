package model

type Status struct {
	Platform   string   `json:"platform"`
	Listening  bool     `json:"listening"`
	Busy       bool     `json:"busy"`
	DataDir    string   `json:"dataDir"`
	Transcript []Turn   `json:"transcript"`
	Slides     []Slide  `json:"slides"`
	Warnings   []string `json:"warnings"`
}

// RecentError 包含运行时脱敏分类快照，绝不包含原始异常正文、密钥、转录正文或个人资料。
type RecentError struct {
	ID       string `json:"id"`
	Time     int64  `json:"time"`
	Level    string `json:"level"`    // "error" | "warn"
	Category string `json:"category"` // 固定脱敏分类代码
}

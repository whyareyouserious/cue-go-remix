package model

// Turn 表示持久化的最终转录，暂定字幕不写入此模型。
type Turn struct {
	// ID 转录业务标识。
	ID string `json:"id"`
	// Channel 来源：you 为麦克风，them 为系统声音。
	Channel string `json:"channel"`
	// Text 转录正文，禁止写入诊断日志。
	Text string `json:"text"`
	// Time 生成时间，Unix 毫秒时间戳，不附带本地时区。
	Time int64 `json:"time"`
}

// Meeting 保存会议生命周期、转录与纪要；时间采用 Unix 毫秒时间戳。
type Meeting struct {
	// ID 会议业务标识，供纪要写入和导出关联。
	ID string `json:"id"`
	// Title 会议标题，默认包含创建时的本地时间。
	Title string `json:"title"`
	// StartedAt 会议开始时间，Unix 毫秒时间戳。
	StartedAt int64 `json:"startedAt"`
	// EndedAt 会议结束时间，Unix 毫秒时间戳；0 表示未结束。
	EndedAt int64 `json:"endedAt"`
	// Turns 按发生顺序保存的最终转录。
	Turns []Turn `json:"turns"`
	// Notes 纪要正文，空值表示尚无纪要。
	Notes string `json:"notes"`
	// NotesTurns 已有纪要覆盖的转录条数，用于判断是否需要补齐。
	NotesTurns int `json:"notesTurns"`
}

// Slide 表示当前会话的页面摘要，仅保存在内存中。
type Slide struct {
	// ID 页面记录业务标识。
	ID string `json:"id"`
	// Time 摘要生成时间，Unix 毫秒时间戳。
	Time int64 `json:"time"`
	// Text 页面摘要正文，禁止写入诊断日志。
	Text string `json:"text"`
}

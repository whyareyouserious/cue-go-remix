package model

// Endpoint 保存独立的聊天或语音服务配置；完整密钥只供业务和存储层使用。
type Endpoint struct {
	// Provider 提供商标识，如 openai、custom、ollama 或 sherpa。
	Provider string `json:"provider"`
	// BaseURL 聊天或语音服务根地址，支持 HTTP/HTTPS。
	BaseURL string `json:"baseUrl"`
	// APIKey 内部凭据；返回前端时清空，禁止写入日志。
	APIKey string `json:"apiKey"`
	// HasKey 表示是否已配置密钥，不承载实际凭据。
	HasKey bool `json:"hasKey"`
	// ClearKey 保存时显式删除旧密钥；省略不代表删除。
	ClearKey bool `json:"clearKey,omitempty"`
	// Model 常规模型名称，具体值由提供商定义。
	Model string `json:"model"`
	// SmartModel 深度思考模型名称，空值时沿用常规模型。
	SmartModel string `json:"smartModel"`
	// Stream 是否请求流式聊天响应。
	Stream bool `json:"stream"`
}

// Settings 保存用户配置；JSON 字段兼容旧版，持久化和脱敏见 repository.Store。
type Settings struct {
	// Chat 聊天服务配置，与语音服务独立。
	Chat Endpoint `json:"chat"`
	// Speech 语音服务配置，与聊天服务独立。
	Speech Endpoint `json:"speech"`
	// Smart 是否优先使用 Chat.SmartModel 回答。
	Smart bool `json:"smart"`
	// Language 语言标识，如 zh、en，具体支持由提供商决定。
	Language string `json:"language"`
	// AIRules 用户回答风格，最多 2000 个字符。
	AIRules string `json:"aiRules"`
	// Background 用户提供的个人背景，以事实参考说明注入提示词。
	Background string `json:"background"`
	// MeetingAudio 是否采集系统声音，仍需满足平台权限。
	MeetingAudio bool `json:"meetingAudio"`
	// Opacity 界面不透明度，范围 0.2 到 1。
	Opacity float64 `json:"opacity"`
	// ContentProtection 是否请求平台支持的录屏排除。
	ContentProtection bool `json:"contentProtection"`
	// SlidesEnabled 是否在音频会话中启用页面记录。
	SlidesEnabled bool `json:"slidesEnabled"`
	// SlidesInterval 页面检查间隔，单位毫秒，范围 1500 到 15000。
	SlidesInterval int `json:"slidesInterval"`
	// WhisperModel 本地 Whisper 模型目录标识。
	WhisperModel string `json:"whisperModel"`
	// WhisperPath 自定义 Whisper 可执行文件路径，空值使用默认查找规则。
	WhisperPath string `json:"whisperPath"`
	// WhisperThreads 工作线程数，最大 64，0 交由运行时决定。
	WhisperThreads int `json:"whisperThreads"`
	// Onboarded 用户是否完成引导的历史标记。
	Onboarded bool `json:"onboarded"`
	// WindowX 窗口左上角横坐标，单位为桌面逻辑像素。
	WindowX int `json:"windowX"`
	// WindowY 窗口左上角纵坐标，单位为桌面逻辑像素。
	WindowY int `json:"windowY"`
	// WindowPositioned 是否已经保存窗口位置，用于区分初始零值。
	WindowPositioned bool `json:"windowPositioned"`
}

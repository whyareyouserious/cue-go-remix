package model

// Endpoint 保存独立的聊天或语音服务配置；完整密钥仅供业务和存储层使用。
type Endpoint struct {
	// Provider 提供商标识，如 openai、custom、ollama 或 sherpa。
	Provider string `json:"provider"`
	// ChatProtocol 聊天接口协议：chat-completions 或 responses；空值沿用服务商原有协议，不用于语音。
	ChatProtocol string `json:"chatProtocol,omitempty"`
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
	// WhisperModel 本地测试时优先使用的草稿模型标识。
	WhisperModel string `json:"whisperModel,omitempty"`
	// WhisperPath 本地测试时优先使用的草稿运行时路径。
	WhisperPath string `json:"whisperPath,omitempty"`
}

// APIChannel 保存一条独立的聊天或云端语音渠道；内嵌端点以保持前端 JSON 字段扁平。
type APIChannel struct {
	// ID 渠道在所属聊天或语音池中的稳定唯一标识，不得使用内置语音引擎的保留 ID。
	ID string `json:"id"`
	// Name 用户设置的展示名称，不能为空，最多 100 个字符。
	Name string `json:"name"`
	// Endpoint 渠道连接参数；密钥仅在后端完整配置中保留，对前端统一脱敏。
	Endpoint
	// MaskedAPIKey 仅用于响应展示的密钥预览，不参与鉴权、日志或持久化；保存时忽略客户端提交值。
	MaskedAPIKey string `json:"maskedApiKey,omitempty"`
	// CreatedAt 历史界面提供的创建时间文本，允许为空，不参与排序或业务时间计算。
	CreatedAt string `json:"createdAt,omitempty"`
	// Status 最近一次显式检测结果：success、failed 或 untested；不代表持续可用。
	Status string `json:"status,omitempty"`
	// Latency 最近一次检测的响应耗时，单位毫秒，零表示未记录。
	Latency int64 `json:"latency,omitempty"`
}

// ProjectExperience 项目职责与成果
type ProjectExperience struct {
	Name         string `json:"name"`         // 项目名称
	Role         string `json:"role"`         // 担任角色/职责
	Achievements string `json:"achievements"` // 成果与亮点
}

// Profile 个人定位分项
type Profile struct {
	Role      string              `json:"role"`      // 岗位
	Industry  string              `json:"industry"`  // 行业
	Years     string              `json:"years"`     // 年限
	Skills    string              `json:"skills"`    // 技术栈
	Strengths string              `json:"strengths"` // 擅长领域
	Goals     string              `json:"goals"`     // 目标
	Projects  []ProjectExperience `json:"projects"`  // 多条项目职责与成果
}

// Preferences 回答偏好；各字段空值表示“未指定”，沿用旧版全局规则或模型默认风格，绝不捏造资料或擅自改变原风格。
type Preferences struct {
	// Length 篇幅长短偏好，空值沿用默认，可选 concise(极简扼要)、balanced(适中平衡)、detailed(详尽充分)。
	Length string `json:"length"`
	// Tone 表达语气风格，空值沿用默认，可选 professional(专业严谨)、direct(坦诚直接)、inspiring(启发引导)、casual(轻松自然)。
	Tone string `json:"tone"`
	// Structure 行文逻辑结构，空值沿用默认，可选 conclusion-first(结论先行)、bullet-points(条理分点)、flexible(自然流式)。
	Structure string `json:"structure"`
	// Examples 举例说明偏好，空值沿用默认，可选 always(务必举例)、moderate(适度举例)、never(避免举例)。
	Examples string `json:"examples"`
	// Custom 自定义补充风格要求，空值表示无额外定制要求。
	Custom string `json:"custom"`
}

// Scenario 场景定义，封装该场景独立的个人定位事实与回答风格偏好。
type Scenario struct {
	// ID 场景唯一标识符，用于持久化映射与场景激活索引。
	ID string `json:"id"`
	// Name 场景展示名称，供界面列表与切换展示。
	Name string `json:"name"`
	// Profile 该场景下的个人定位事实分项（岗位、行业、技能、擅长、目标与项目成果）。
	Profile Profile `json:"profile"`
	// Preferences 该场景下的模型回答偏好（长短、语气、结构、举例与自定义要求）。
	Preferences Preferences `json:"preferences"`
}

type Settings struct {
	// Chat 聊天服务配置，与语音服务独立。
	Chat Endpoint `json:"chat"`
	// Speech 语音服务配置，与聊天服务独立。
	Speech Endpoint `json:"speech"`
	// ChatChannels 聊天渠道池；nil 兼容旧单端点输入，空数组表示明确删除所有渠道。
	ChatChannels []APIChannel `json:"chatChannels"`
	// SpeechChannels 云端语音渠道池，不包含内置 Sherpa 和 Whisper 引擎。
	SpeechChannels []APIChannel `json:"speechChannels"`
	// ActiveChatChannelID 当前生效的聊天渠道 ID，空值表示尚未选择聊天渠道。
	ActiveChatChannelID string `json:"activeChatChannelId"`
	// ActiveSpeechChannelID 当前语音渠道 ID，也可为 local_sherpa 或 local_whisper。
	ActiveSpeechChannelID string `json:"activeSpeechChannelId"`
	// Smart 是否优先使用 Chat.SmartModel 回答。
	Smart bool `json:"smart"`
	// Language 语言标识，如 zh、en，具体支持由提供商决定。
	Language string `json:"language"`
	// AIRules 用户回答风格，最多 2000 个字符；完整保留并兼容。
	AIRules string `json:"aiRules"`
	// Background 用户提供的个人背景，以事实参考说明注入提示词；完整保留并兼容。
	Background string `json:"background"`
	// Scenarios 多套场景列表。
	Scenarios []Scenario `json:"scenarios"`
	// ActiveScenarioID 当前激活的场景 ID。
	ActiveScenarioID string `json:"activeScenarioId"`
	// Shortcuts 自定义全局快捷键映射（操作名 -> 键位组合）。
	Shortcuts map[string]string `json:"shortcuts,omitempty"`
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

// Clone 实现完整的深拷贝，切片和映射独立分配，杜绝浅拷贝引用共享。
func (s Settings) Clone() Settings {
	cp := s
	if s.ChatChannels != nil {
		cp.ChatChannels = make([]APIChannel, len(s.ChatChannels))
		copy(cp.ChatChannels, s.ChatChannels)
	}
	if s.SpeechChannels != nil {
		cp.SpeechChannels = make([]APIChannel, len(s.SpeechChannels))
		copy(cp.SpeechChannels, s.SpeechChannels)
	}
	if s.Scenarios != nil {
		cp.Scenarios = make([]Scenario, len(s.Scenarios))
		for i, sc := range s.Scenarios {
			scCopy := sc
			if sc.Profile.Projects != nil {
				scCopy.Profile.Projects = make([]ProjectExperience, len(sc.Profile.Projects))
				copy(scCopy.Profile.Projects, sc.Profile.Projects)
			}
			cp.Scenarios[i] = scCopy
		}
	}
	if s.Shortcuts != nil {
		cp.Shortcuts = make(map[string]string, len(s.Shortcuts))
		for k, v := range s.Shortcuts {
			cp.Shortcuts[k] = v
		}
	}
	return cp
}

// EndpointCheckResult 保存服务连接探测结果；不承载密钥或用户正文。
type EndpointCheckResult struct {
	// Success 表示服务连通性与鉴权探测是否完全成功；为 true 时链路正常，为 false 时由 Message 说明原因。
	Success bool   `json:"success"`
	Latency int64  `json:"latency"` // 响应延迟（毫秒）
	Message string `json:"message"` // 说明或失败原因
}

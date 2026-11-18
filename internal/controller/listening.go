package controller

// StartListening 启动音频采集和转写；重复调用保持当前会话，退出中或设备、模型初始化失败时返回错误。
func (a *App) StartListening() error { return a.runtime.StartListening() }

// StopListening 取消正在初始化或运行的音频会话，释放模型，并按原流程尝试生成当前会议纪要。
func (a *App) StopListening() error { return a.runtime.StopListening() }

// ClearTranscript 结束当前会议并清空运行态转录和页面记录；持久化失败返回错误，已保存历史保留。
func (a *App) ClearTranscript() error { return a.runtime.ClearTranscript() }

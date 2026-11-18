package model

// AskRequest 接收前端或快捷键问答参数，由 service.Runtime 校验并编排任务。
type AskRequest struct {
	// Mode 模式：ask、answerThis、say、assist、recap 或 leetcode。
	Mode string `json:"mode"`
	// Text 问题正文；ask 和 answerThis 必填，最多 30000 个字符。
	Text string `json:"text"`
}

package controller

import (
	"cue-go-remix/internal/model"
)

// RecordUIEvent 按白名单记录 event 和 value；忽略任意用户输入，避免密钥或正文进入日志。
func (a *App) RecordUIEvent(event, value string) { a.runtime.RecordUIEvent(event, value) }

// GetStatus 返回平台、任务状态及最近最多 200 条转录的快照，不包含 API Key。
func (a *App) GetStatus() model.Status { return a.runtime.GetStatus() }

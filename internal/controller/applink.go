package controller

import (
	"cue-go-remix/internal/applink"
)

// Consent 处理 id 对应的本地调用方权限申请；allowed 只决定本次请求的授权结果。
func (a *App) Consent(id string, allowed bool) { a.runtime.Consent(id, allowed) }

// ListCallers 返回本地助手调用方记录；连接模块尚未启动时返回空列表。
func (a *App) ListCallers() []applink.Caller { return a.runtime.ListCallers() }

// RevokeCaller 撤销 id 对应调用方权限并终止相关连接；保存失败返回错误。
func (a *App) RevokeCaller(id string) error { return a.runtime.RevokeCaller(id) }

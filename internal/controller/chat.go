package controller

import (
	"cue-go-remix/internal/model"
)

// Ask 使用 ask 指定的模式和问题异步生成回答；参数或模型配置无效、已有任务或正在退出时返回错误。
func (a *App) Ask(ask model.AskRequest) error { return a.runtime.Ask(ask) }

// Cancel 取消当前回答任务；没有进行中的回答时不执行操作。
func (a *App) Cancel() { a.runtime.Cancel() }

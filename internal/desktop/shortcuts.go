package desktop

import (
	"cue-go-remix/internal/model"
)

func (a *Host) registerShortcuts() {
	for key, mode := range map[string]string{"CmdOrCtrl+Enter": "say", "CmdOrCtrl+Shift+Enter": "assist", "CmdOrCtrl+H": "leetcode", "CmdOrCtrl+Shift+/": "toggle", "CmdOrCtrl+Shift+X": "quit", "CmdOrCtrl+Shift+I": "passthrough"} {
		err := a.desktop.GlobalShortcut.Register(key, func() {
			switch mode {
			case "quit":
				a.Quit()
			case "toggle":
				_ = a.SetPassthrough(false)
				a.runtime.Emit(model.Event{Type: "shortcut", Text: "toggle"})
			case "passthrough":
				_ = a.SetPassthrough(false)
				a.runtime.Emit(model.Event{Type: "error", Text: "已恢复窗口鼠标交互"})
			default:
				a.runtime.Report(a.runtime.Ask(model.AskRequest{Mode: mode}))
			}
		})
		if err != nil {
			a.runtime.Warn("快捷键 " + key + " 注册失败：" + err.Error())
		} else if mode == "passthrough" {
			a.mu.Lock()
			a.passthroughReady = true
			a.mu.Unlock()
		}
	}
}

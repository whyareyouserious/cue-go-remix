package service

import (
	"cue-go-remix/internal/logging"
)

// RecordUIEvent 的所有参数均受白名单约束，不能用来写入任意用户输入。
func (a *Runtime) RecordUIEvent(event, value string) {
	allowed := false
	switch event {
	case "settings-open", "settings-save":
		switch value {
		case "chat", "speech", "style", "appearance", "models", "access":
			allowed = true
		}
	case "provider-select":
		allowed = logging.Provider(value) != "unknown"
	case "key-focus":
		allowed = value == "chat" || value == "speech"
	case "panel-toggle":
		allowed = value == "collapsed" || value == "expanded"
	case "sidebar-toggle":
		allowed = value == "open" || value == "closed"
	}
	if allowed {
		a.trace("event=ui action=%s value=%s", event, value)
	}
}

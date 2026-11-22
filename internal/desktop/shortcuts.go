package desktop

import (
	"fmt"
	"strings"

	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
)

// defaultShortcuts 返回默认快捷键映射。
func defaultShortcuts() map[string]string {
	return repository.DefaultSettings().Shortcuts
}

// normalizeKey 归一化快捷键字符串，以便比对冲突。
func normalizeKey(key string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), " ", ""))
}

// validateShortcuts 校验快捷键是否有冲突或缺失关键项。
func validateShortcuts(shortcuts map[string]string) error {
	required := []string{"say", "assist", "leetcode", "toggle", "quit", "passthrough"}
	seen := make(map[string]string) // normalizedKey -> action

	for _, action := range required {
		key, ok := shortcuts[action]
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("快捷键 [%s] 不能为空", action)
		}
		norm := normalizeKey(key)
		if existingAction, exists := seen[norm]; exists {
			return fmt.Errorf("快捷键冲突：[%s] 与 [%s] 不能使用相同按键 (%s)", existingAction, action, key)
		}
		seen[norm] = action
	}
	return nil
}

// copyShortcuts 创建深拷贝映射。
func copyShortcuts(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// registerShortcuts 启动时注册配置中的全局快捷键。
func (a *Host) registerShortcuts() {
	a.shortcutMu.Lock()
	defer a.shortcutMu.Unlock()
	settings := a.runtime.Settings()
	shortcuts := settings.Shortcuts
	if len(shortcuts) == 0 {
		shortcuts = defaultShortcuts()
	}
	ptOk, err := a.applyShortcuts(shortcuts)
	a.mu.Lock()
	a.passthroughReady = ptOk
	a.mu.Unlock()
	if err != nil {
		a.runtime.Warn("快捷键启动注册存在失败项: " + err.Error())
	}
}

// applyShortcuts 在系统级应用快捷键映射；返回 passthrough 键是否注册成功及可能出现的错误。
func (a *Host) applyShortcuts(shortcuts map[string]string) (bool, error) {
	a.desktop.GlobalShortcut.UnregisterAll()

	var err error
	var ptOk bool

	for action, key := range shortcuts {
		act := action
		k := key
		regErr := a.desktop.GlobalShortcut.Register(k, func() {
			switch act {
			case "quit":
				a.Quit()
			case "toggle":
				_ = a.SetPassthrough(false)
				a.runtime.Emit(model.Event{Type: "shortcut", Text: "toggle"})
			case "passthrough":
				_ = a.SetPassthrough(false)
				a.runtime.Emit(model.Event{Type: "error", Text: "已恢复窗口鼠标交互"})
			default:
				a.runtime.Report(a.runtime.Ask(model.AskRequest{Mode: act}))
			}
		})
		if regErr != nil {
			if err == nil {
				err = fmt.Errorf("[%s=%s: %w]", act, k, regErr)
			}
		} else if act == "passthrough" {
			ptOk = true
		}
	}

	return ptOk, err
}

// UpdateShortcuts 校验并重注册全局快捷键，并在持久化失败时同样执行回滚；
// 回滚失败绝不虚假声称已恢复，失败时强制恢复鼠标交互并准确维护 passthroughReady。
func (a *Host) UpdateShortcuts(shortcuts map[string]string, persistFn func() error) error {
	a.shortcutMu.Lock()
	defer a.shortcutMu.Unlock()

	if err := validateShortcuts(shortcuts); err != nil {
		return err
	}

	oldShortcuts := a.runtime.Settings().Shortcuts
	if len(oldShortcuts) == 0 {
		oldShortcuts = defaultShortcuts()
	}

	// 1. 尝试应用新快捷键
	newPtOk, err := a.applyShortcuts(shortcuts)
	if err != nil {
		// 注册失败，执行回滚
		rbPtOk, rbErr := a.applyShortcuts(oldShortcuts)
		_ = a.SetPassthrough(false) // 发生失败强制恢复鼠标交互，防穿透卡死
		a.mu.Lock()
		a.passthroughReady = rbPtOk
		a.mu.Unlock()

		if rbErr != nil {
			return fmt.Errorf("快捷键注册失败 (%v)，回滚原快捷键亦失败 (%v)，已恢复鼠标交互", err, rbErr)
		}
		return fmt.Errorf("快捷键注册失败：%w，已恢复原快捷键", err)
	}

	// 2. 注册成功，执行持久化
	if persistFn != nil {
		if saveErr := persistFn(); saveErr != nil {
			// 持久化失败，必须将已注册的新快捷键回滚为旧快捷键
			rbPtOk, rbErr := a.applyShortcuts(oldShortcuts)
			_ = a.SetPassthrough(false) // 恢复鼠标交互
			a.mu.Lock()
			a.passthroughReady = rbPtOk
			a.mu.Unlock()

			if rbErr != nil {
				return fmt.Errorf("快捷键保存配置失败 (%v)，回滚原快捷键亦失败 (%v)，已恢复鼠标交互", saveErr, rbErr)
			}
			return fmt.Errorf("快捷键保存配置失败：%w，已恢复原快捷键", saveErr)
		}
	}

	// 3. 注册与持久化均成功，准确维护 passthrough 状态
	a.mu.Lock()
	a.passthroughReady = newPtOk
	a.mu.Unlock()
	return nil
}

package entitlement

import (
	"context"
	"errors"
	"time"

	secure "cue-go-remix/internal/entitlement/platform"
)

func (m *Manager) initialize(restore bool) error {
	if m.initialized {
		return nil
	}
	id, err := m.opts.MachineID()
	if err != nil {
		return errors.New("无法获取本机设备标识，不能验证授权")
	}
	if id == "" {
		return errors.New("本机设备标识为空")
	}
	m.device = id
	saved, err := m.opts.Load()
	if err != nil && !errors.Is(err, secure.ErrNotFound) && restore {
		return errors.New("无法读取系统凭据，请检查本机凭据访问权限")
	}
	if err == nil {
		m.credential = saved
		m.mu.Lock()
		m.state.Saved = true
		m.state.Mode = saved.Mode
		m.state.MaskedKey = mask(saved.LicenseKey)
		m.mu.Unlock()
	}
	m.initialized = true
	return nil
}

func mask(key string) string {
	if len(key) < 4 {
		return ""
	}
	return "••••" + key[len(key)-4:]
}

func (m *Manager) Forget() error {
	m.op.Lock()
	defer m.op.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return context.Canceled
	}
	token, mode := m.token, m.credential.Mode
	m.lockLocked("本机授权已清除，请重新激活")
	m.retry = false
	m.token = ""
	m.credential = secure.Credential{}
	m.state = Status{State: "inactive", Message: "输入 Cue 卡密或领取一次 24 小时试用"}
	m.mu.Unlock()
	m.emit()
	err := m.opts.Delete()
	if err != nil && !errors.Is(err, secure.ErrNotFound) {
		m.fail(errors.New("系统凭据删除失败，请重试清除授权"))
		return errors.New("系统凭据删除失败，请重试清除授权")
	}
	if mode == "license" && token != "" {
		ctx, cancel := context.WithTimeout(m.root, 2*time.Second)
		defer cancel()
		_, _ = m.client.request(ctx, "license/deactivate", token, map[string]string{"machineId": m.device})
	}
	return nil
}

package entitlement

import (
	"context"
	"errors"
	"strings"
	"time"

	secure "cue-go-remix/internal/entitlement/platform"
)

func (m *Manager) Retry() error { m.op.Lock(); defer m.op.Unlock(); return m.retryLocked() }

func (m *Manager) retryLocked() error {
	if m.root.Err() != nil {
		return m.root.Err()
	}
	if err := m.initialize(true); err != nil {
		m.fail(err)
		return err
	}
	if m.credential.Mode == "" {
		m.mu.Lock()
		m.state = Status{State: "inactive", Message: "输入 Cue 卡密或领取一次 24 小时试用"}
		m.retry = false
		m.mu.Unlock()
		m.emit()
		return nil
	}
	return m.activate(m.credential, false)
}

func (m *Manager) Activate(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 128 {
		return errors.New("请输入有效的 Cue 卡密")
	}
	m.op.Lock()
	defer m.op.Unlock()
	if m.root.Err() != nil {
		return m.root.Err()
	}
	if err := m.initialize(false); err != nil {
		m.fail(err)
		return err
	}
	return m.activate(secure.Credential{Mode: "license", LicenseKey: key}, true)
}

func (m *Manager) Trial() error {
	m.op.Lock()
	defer m.op.Unlock()
	if m.root.Err() != nil {
		return m.root.Err()
	}
	if err := m.initialize(false); err != nil {
		m.fail(err)
		return err
	}
	m.mu.Lock()
	paid := m.state.Allowed && m.state.Mode == "license"
	m.mu.Unlock()
	if paid {
		return errors.New("当前卡密已激活，无需领取试用")
	}
	return m.activate(secure.Credential{Mode: "trial"}, true)
}

func (m *Manager) activate(credential secure.Credential, save bool) error {
	if credential.Mode != "license" && credential.Mode != "trial" {
		err := errors.New("本机授权记录无效，请重新输入卡密")
		m.fail(err)
		return err
	}
	m.mu.Lock()
	epoch := m.epoch
	wasAllowed := m.state.Allowed
	oldCred, oldToken := m.credential, m.token
	if !wasAllowed {
		m.state.State = "checking"
		m.state.Message = "正在联网验证授权"
	}
	m.mu.Unlock()
	m.emit()
	verifiedAt := m.opts.Now()
	reply, err := m.client.request(m.root, credential.Mode+"/activate", "", map[string]string{"licenseKey": credential.LicenseKey, "machineId": m.device, "clientVersion": "0.1.0"})
	if err == nil {
		err = validate(reply, credential.Mode, true)
	}
	m.mu.Lock()
	stale := epoch != m.epoch || m.closed
	m.mu.Unlock()
	if stale {
		return context.Canceled
	}
	if err != nil {
		if wasAllowed && save && credential != oldCred {
			m.emit()
			return err
		}
		m.failRequest(err, epoch)
		return err
	}
	warning := ""
	m.mu.Lock()
	saved := m.state.Saved
	m.mu.Unlock()
	if save {
		saved = true
		if err = m.opts.Save(credential); err != nil {
			saved = false
			warning = "本次已激活，但系统凭据保存失败；下次启动需重新输入卡密"
		}
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return context.Canceled
	}
	if m.expiredLocked(m.opts.Now()) {
		m.lockLocked("授权验证已超时，请联网重新验证")
	}
	m.credential = credential
	m.token = reply.Token
	if epoch != m.epoch {
		m.token = ""
		m.state.Saved = saved
		m.mu.Unlock()
		return context.Canceled
	}
	m.state = Status{State: credential.Mode, Allowed: true, Mode: credential.Mode, ExpiresAt: reply.ExpiredAt, Message: warning, Saved: saved, MaskedKey: mask(credential.LicenseKey)}
	err = m.acceptLocked(reply, verifiedAt)
	m.mu.Unlock()
	m.emit()
	if oldCred.Mode == "license" && oldToken != "" && oldCred.LicenseKey != credential.LicenseKey {
		ctx, cancel := context.WithTimeout(m.root, 2*time.Second)
		defer cancel()
		_, _ = m.client.request(ctx, "license/deactivate", oldToken, map[string]string{"machineId": m.device})
	}
	return err
}

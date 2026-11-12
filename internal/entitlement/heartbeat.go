package entitlement

import (
	"errors"
)

func (m *Manager) tick() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	expired := m.expiredLocked(m.opts.Now())
	if expired {
		m.lockLocked("授权验证已超时，请联网重新验证")
		m.retry = true
	}
	due := !m.opts.Now().Before(m.next) && (m.state.Allowed || m.retry)
	m.mu.Unlock()
	if expired {
		m.emit()
	}
	if !due {
		return
	}
	// 网络心跳独立于每秒有效期检查，慢请求不能延长使用时间。
	if !m.op.TryLock() {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.op.Unlock()
		return
	}
	m.wg.Add(1)
	m.next = m.opts.Now().Add(heartbeat)
	m.mu.Unlock()
	go func() { defer m.wg.Done(); defer m.op.Unlock(); m.pulse() }()
}

func (m *Manager) pulse() {
	if m.root.Err() != nil {
		return
	}
	m.mu.Lock()
	epoch := m.epoch
	token, mode := m.token, m.credential.Mode
	m.mu.Unlock()
	if token == "" {
		_ = m.retryLocked()
		return
	}
	verifiedAt := m.opts.Now()
	reply, err := m.client.request(m.root, mode+"/heartbeat", token, map[string]string{"machineId": m.device})
	m.mu.Lock()
	stale := epoch != m.epoch || m.closed
	m.mu.Unlock()
	if stale {
		return
	}
	var api *APIError
	if errors.As(err, &api) && api.Code == 1203 {
		_ = m.retryLocked()
		return
	}
	if err == nil {
		err = validate(reply, mode, false)
		if err != nil {
			m.client.log("event=license-validation operation=%s result=invalid_payload", licenseOperation(mode+"/heartbeat"))
		}
	}
	if err != nil {
		m.failRequest(err, epoch)
		return
	}
	m.mu.Lock()
	if !m.closed && epoch == m.epoch {
		if m.expiredLocked(m.opts.Now()) {
			m.lockLocked("授权验证已超时，请联网重新验证")
		}
		m.state.Allowed = true
		m.state.State = mode
		m.state.Message = ""
		m.state.ExpiresAt = reply.ExpiredAt
		_ = m.acceptLocked(reply, verifiedAt)
	}
	m.mu.Unlock()
	m.emit()
}

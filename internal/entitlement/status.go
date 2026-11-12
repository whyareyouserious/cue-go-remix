package entitlement

import (
	"context"
	"errors"
	"time"
)

type Status struct {
	State        string `json:"state"`
	Allowed      bool   `json:"allowed"`
	Mode         string `json:"mode"`
	ExpiresAt    int64  `json:"expiresAt"`
	GraceSeconds int    `json:"graceSeconds"`
	Message      string `json:"message"`
	Saved        bool   `json:"saved"`
	MaskedKey    string `json:"maskedKey,omitempty"`
}

var ErrLocked = errors.New("请先激活 Cue 或领取试用")

const grace = 120 * time.Second

const heartbeat = 30 * time.Second

func (m *Manager) expiredLocked(now time.Time) bool {
	if !m.state.Allowed {
		return false
	}
	return m.deadlineExceeded(now)
}

func (m *Manager) deadlineExceeded(now time.Time) bool {
	// 同时比较实际时钟与单调时钟，休眠/时钟回拨不能延长本轮宽限。
	return !now.Before(m.expires) || now.UnixMilli() >= m.expires.UnixMilli() || now.Sub(m.lastOK) >= grace || now.UnixMilli()-m.lastOK.UnixMilli() >= grace.Milliseconds() || now.UnixMilli() < m.lastOK.UnixMilli()-1000
}

func (m *Manager) lockLocked(message string) {
	if m.revoke != nil {
		m.revoke()
		m.revoke = nil
	}
	m.work = nil
	m.state.Allowed = false
	m.state.State = "locked"
	m.state.Message = message
	m.state.GraceSeconds = 0
}

func (m *Manager) snapshotLocked() Status {
	s := m.state
	if s.State == "grace" {
		now := m.opts.Now()
		elapsed := max(now.Sub(m.lastOK), time.Duration(now.UnixMilli()-m.lastOK.UnixMilli())*time.Millisecond)
		left := min(grace-elapsed, m.expires.Sub(now), time.Duration(m.expires.UnixMilli()-now.UnixMilli())*time.Millisecond)
		s.GraceSeconds = max(0, int(left.Seconds()))
	}
	return s
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.snapshotLocked()
	if m.expiredLocked(m.opts.Now()) {
		s.Allowed = false
		s.State = "locked"
		s.Message = "授权验证已超时，请联网重新验证"
		s.GraceSeconds = 0
	}
	return s
}

func (m *Manager) WorkContext() (context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.expiredLocked(m.opts.Now()) {
		if m.revoke != nil {
			m.revoke()
		}
		return nil, ErrLocked
	}
	if m.closed || !m.state.Allowed || m.work == nil {
		return nil, ErrLocked
	}
	return m.work, nil
}

func (m *Manager) emit() {
	if m.opts.Changed != nil {
		m.opts.Changed(m.Status())
	}
}

func (m *Manager) fail(err error) {
	m.mu.Lock()
	m.failLocked(err)
	m.mu.Unlock()
	m.emit()
}

func (m *Manager) failRequest(err error, epoch uint64) {
	m.mu.Lock()
	if epoch != m.epoch {
		m.mu.Unlock()
		return
	}
	m.failLocked(err)
	m.mu.Unlock()
	m.emit()
}

func (m *Manager) failLocked(err error) {
	if !m.closed {
		m.retry = temporary(err)
		m.next = m.opts.Now().Add(10 * time.Second)
		if m.retry && m.state.Allowed && !m.expiredLocked(m.opts.Now()) {
			m.state.State = "grace"
			m.state.Message = err.Error()
		} else {
			m.lockLocked(err.Error())
		}
	}
}

func validate(r Reply, mode string, activate bool) error {
	if r.AppStatus == nil || *r.AppStatus != 2 || r.Mode != mode || r.ServerTime <= 0 || r.ExpiredAt <= r.ServerTime || (activate && r.Token == "") || (!activate && !r.Valid) {
		return &APIError{}
	}
	return nil
}

func (m *Manager) acceptLocked(r Reply, verifiedAt time.Time) error {
	now := m.opts.Now()
	// 从请求发出时保守计时，网络传输和凭据存储不能延长服务端剩余有效期。
	m.lastOK = verifiedAt
	m.expires = verifiedAt.Add(time.UnixMilli(r.ExpiredAt).Sub(time.UnixMilli(r.ServerTime)))
	m.next = verifiedAt.Add(heartbeat)
	m.retry = false
	if m.deadlineExceeded(now) {
		m.lockLocked("本次验证已超时，请重新验证")
		m.retry = true
		return errors.New("本次验证已超时，请重新验证")
	}
	if m.work == nil || m.work.Err() != nil {
		m.work, m.revoke = context.WithCancel(m.root)
	}
	return nil
}

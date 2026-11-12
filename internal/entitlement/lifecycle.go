package entitlement

import (
	"context"
	"time"
)

func (m *Manager) Start() {
	m.mu.Lock()
	if m.started || m.closed {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.wg.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.wg.Done()
		_ = m.Retry()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.root.Done():
				return
			case <-ticker.C:
				m.tick()
			}
		}
	}()
}

// Revalidate 在休眠/唤醒事件中撤销本轮工作，重新联网验证。
// 两个事件采用同一操作，不依赖桌面框架异步回调的先后顺序。
func (m *Manager) Revalidate() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.epoch++
	if m.state.Allowed || m.retry || m.state.State == "checking" {
		m.lockLocked("系统休眠后需重新联网验证")
		m.token = ""
		m.retry = true
		m.next = m.opts.Now()
	}
	m.mu.Unlock()
	m.emit()
}

func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.lockLocked("应用已退出")
	m.cancel()
	m.mu.Unlock()
	m.op.Lock()
	token, mode := m.token, m.credential.Mode
	m.op.Unlock()
	m.wg.Wait()
	if mode == "license" && token != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = m.client.request(ctx, "license/deactivate", token, map[string]string{"machineId": m.device})
	}
}

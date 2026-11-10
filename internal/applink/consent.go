package applink

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	"cue-go-remix/internal/model"
)

func (s *Server) emit(event model.Event) {
	if s.deps.Emit != nil {
		s.deps.Emit(event)
	}
}
func (s *Server) callerName(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.callers[id]; ok && c.Name != "" {
		return c.Name
	}
	return id
}
func (s *Server) resolveConsent(callerID, name, scope string) bool {
	key := callerID + "\x00" + scope
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	decision := s.callers[callerID].Scopes[scope]
	if decision == "granted" || decision == "denied" {
		s.mu.Unlock()
		return decision == "granted"
	}
	p, exists := s.pendingKeys[key]
	if !exists {
		// A second caller must reconnect after the visible request is resolved.
		// Never replace a consent dialog that the user is already reviewing.
		if len(s.pending) > 0 {
			s.mu.Unlock()
			return false
		}
		id, err := randomID(16)
		if err != nil {
			s.mu.Unlock()
			return false
		}
		p = &pendingConsent{id, callerID, name, scope, make(chan struct{}), false}
		s.pending[id] = p
		s.pendingKeys[key] = p
	}
	s.mu.Unlock()
	if !exists {
		s.emit(model.Event{Type: "consent", ID: p.id, Data: ConsentRequest{Caller: name, CallerID: callerID, CallerName: name, Scope: scope, Verification: "token"}})
	}
	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.allowed
	case <-s.done:
		return false
	case <-timer.C:
		s.finishConsent(p.id, false, false)
		return false
	}
}

// Consent resolves a displayed request ID, not a caller ID. Unknown or expired
// request IDs have no effect, preventing a stale UI response from granting anew.
func (s *Server) Consent(id string, allowed bool) { s.finishConsent(id, allowed, true) }
func (s *Server) finishConsent(id string, allowed, persist bool) {
	s.mu.Lock()
	p, ok := s.pending[id]
	if !ok || s.closed {
		s.mu.Unlock()
		return
	}
	delete(s.pending, id)
	delete(s.pendingKeys, p.callerID+"\x00"+p.scope)
	var saveErr error
	if persist {
		c := s.callers[p.callerID]
		c.ID = p.callerID
		c.Name = p.name
		if c.Scopes == nil {
			c.Scopes = map[string]string{}
		}
		decision := "denied"
		if allowed {
			decision = "granted"
		}
		c.Scopes[p.scope] = decision
		c.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		s.callers[p.callerID] = c
		saveErr = s.persistLocked()
		if saveErr != nil {
			allowed = false
			c.Scopes[p.scope] = "denied"
			s.callers[p.callerID] = c
		}
	}
	p.allowed = allowed
	close(p.done)
	s.mu.Unlock()
	if saveErr != nil {
		s.emit(model.Event{Type: "error", Text: "保存本地连接授权失败，已拒绝本次请求"})
	}
	s.emit(model.Event{Type: "callers", Data: s.ListCallers()})
}
func (s *Server) ListCallers() []Caller {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Caller, 0, len(s.callers))
	for _, c := range s.callers {
		copy := Caller{ID: c.ID, Name: c.Name, UpdatedAt: c.UpdatedAt, Scopes: []string{}}
		for k, v := range c.Scopes {
			if v == "granted" {
				copy.Scopes = append(copy.Scopes, k)
			}
		}
		sort.Strings(copy.Scopes)
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Server) Revoke(id string) error {
	s.mu.Lock()
	delete(s.callers, id)
	for requestID, p := range s.pending {
		if p.callerID == id {
			delete(s.pending, requestID)
			delete(s.pendingKeys, p.callerID+"\x00"+p.scope)
			p.allowed = false
			close(p.done)
		}
	}
	conns := []net.Conn{}
	for conn, sess := range s.connections {
		if sess.callerID == id {
			conns = append(conns, conn)
		}
	}
	err := s.persistLocked()
	s.mu.Unlock()
	for _, conn := range conns {
		conn.Close()
	}
	s.record("info", "已撤销本地调用方授权")
	s.emit(model.Event{Type: "callers", Data: s.ListCallers()})
	if err != nil {
		return fmt.Errorf("已断开连接，但保存撤销结果失败：%w", err)
	}
	return nil
}
func (s *Server) persistLocked() error {
	return atomicJSON(filepath.Join(s.dir, "consent.json"), map[string]any{"schema": 1, "appId": AppID, "callers": s.callers})
}
func atomicJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".cue-link-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

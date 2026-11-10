// Package applink implements a consent-gated local JSON-RPC connection.
// Its token authenticates the local user boundary, not the claimed application.
package applink

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"cue-go-remix/internal/model"
)

const AppID = "com.cue.remix"
const maxFrame = 1024 * 1024

type Dependencies struct {
	Emit         func(model.Event)
	Status       func() model.Status
	SetCapturing func(bool) error
	Slides       func() []model.Slide
}
type Caller struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	UpdatedAt string   `json:"updatedAt"`
}
type storedCaller struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Scopes    map[string]string `json:"scopes"`
	UpdatedAt string            `json:"updatedAt"`
}
type ConsentRequest struct {
	Caller       string `json:"caller"`
	CallerID     string `json:"callerId"`
	CallerName   string `json:"callerName"`
	Scope        string `json:"scope"`
	Verification string `json:"verification"`
}
type instance struct {
	Schema          int    `json:"schema"`
	AppID           string `json:"appId"`
	AppSlug         string `json:"appSlug"`
	AppName         string `json:"appName"`
	AppVersion      string `json:"appVersion"`
	ProtocolVersion string `json:"protocolVersion"`
	Platform        string `json:"platform"`
	Transport       string `json:"transport"`
	SocketPath      string `json:"socketPath"`
	PID             int    `json:"pid"`
	InstanceID      string `json:"instanceId"`
	Token           string `json:"token"`
	StartedAt       string `json:"startedAt"`
}
type pendingConsent struct {
	id, callerID, name, scope string
	done                      chan struct{}
	allowed                   bool
}
type session struct {
	conn      net.Conn
	callerID  string
	connected bool
	scopes    map[string]bool
}
type logEvent struct {
	Seq     int64  `json:"seq"`
	Time    string `json:"ts"`
	Level   string `json:"level"`
	Message string `json:"msg"`
}
type Server struct {
	dir         string
	deps        Dependencies
	mu          sync.Mutex
	listener    net.Listener
	instance    instance
	cleanup     func()
	connections map[net.Conn]*session
	callers     map[string]storedCaller
	pending     map[string]*pendingConsent
	pendingKeys map[string]*pendingConsent
	events      []logEvent
	sequence    int64
	done        chan struct{}
	closed      bool
	wg          sync.WaitGroup
}

func New(dir string, deps Dependencies) *Server {
	return &Server{dir: filepath.Join(dir, "applink"), deps: deps, connections: map[net.Conn]*session{}, callers: map[string]storedCaller{}, pending: map[string]*pendingConsent{}, pendingKeys: map[string]*pendingConsent{}, done: make(chan struct{})}
}
func randomID(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("本地连接服务已关闭")
	}
	if s.listener != nil {
		return nil
	}
	if err := privateDirectory(s.dir); err != nil {
		return err
	}
	if data, err := os.ReadFile(filepath.Join(s.dir, "consent.json")); err == nil {
		var saved struct {
			Callers map[string]storedCaller `json:"callers"`
		}
		if json.Unmarshal(data, &saved) == nil && saved.Callers != nil {
			s.callers = saved.Callers
		}
	}
	token, err := randomID(32)
	if err != nil {
		return err
	}
	id, err := randomID(16)
	if err != nil {
		return err
	}
	listener, path, transport, cleanup, err := listenLocal(s.dir, id)
	if err != nil {
		return fmt.Errorf("启动本地连接服务失败：%w", err)
	}
	platform := runtime.GOOS
	if platform == "windows" {
		platform = "win32"
	}
	s.instance = instance{1, AppID, "cue-remix", "Cue", "0.1.0", "1", platform, transport, path, os.Getpid(), id, token, time.Now().UTC().Format(time.RFC3339Nano)}
	if err := atomicJSON(filepath.Join(s.dir, "run.json"), s.instance); err != nil {
		listener.Close()
		cleanup()
		return err
	}
	s.listener, s.cleanup = listener, cleanup
	s.wg.Add(1)
	go s.acceptLoop(listener)
	return nil
}

func (s *Server) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.done)
	listener, cleanup := s.listener, s.cleanup
	conns := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		conns = append(conns, conn)
	}
	s.mu.Unlock()
	if listener != nil {
		listener.Close()
	}
	for _, conn := range conns {
		conn.Close()
	}
	s.wg.Wait()
	if cleanup != nil {
		cleanup()
	}
	// Do not remove a run file replaced by another running instance.
	path := filepath.Join(s.dir, "run.json")
	if data, err := os.ReadFile(path); err == nil {
		var current instance
		if json.Unmarshal(data, &current) == nil && current.InstanceID == s.instance.InstanceID {
			_ = os.Remove(path)
		}
	}
}

func (s *Server) acceptLoop(listener net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed || len(s.connections) >= 8 {
			s.mu.Unlock()
			conn.Close()
			continue
		}
		sess := &session{conn: conn, scopes: map[string]bool{}}
		s.connections[conn] = sess
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(sess)
	}
}

type request struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string           { return e.Message }
func rpcErr(code int, message string) error { return &rpcError{code, message} }

func (s *Server) serve(sess *session) {
	defer s.wg.Done()
	defer func() { sess.conn.Close(); s.mu.Lock(); delete(s.connections, sess.conn); s.mu.Unlock() }()
	scanner := bufio.NewScanner(sess.conn)
	scanner.Buffer(make([]byte, 4096), maxFrame)
	windowStart, count := time.Now(), 0
	for {
		_ = sess.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		if !scanner.Scan() {
			return
		}
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.reply(sess, nil, nil, rpcErr(-32700, "JSON 格式无效"))
			continue
		}
		var id any
		if req.Version != "2.0" || req.Method == "" || json.Unmarshal(req.ID, &id) != nil || id == nil {
			s.reply(sess, req.ID, nil, rpcErr(-32600, "需要带 id 的 JSON-RPC 2.0 请求"))
			continue
		}
		switch id.(type) {
		case string, float64:
		default:
			s.reply(sess, nil, nil, rpcErr(-32600, "请求 id 必须为数字或字符串"))
			continue
		}
		if time.Since(windowStart) >= 10*time.Second {
			windowStart, count = time.Now(), 0
		}
		count++
		if count > 120 {
			s.reply(sess, req.ID, nil, rpcErr(-32003, "请求过于频繁"))
			continue
		}
		params := map[string]json.RawMessage{}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil || params == nil {
				s.reply(sess, req.ID, nil, rpcErr(-32602, "params 必须是对象"))
				continue
			}
		}
		result, err := s.dispatch(sess, req.Method, params)
		if !s.reply(sess, req.ID, result, err) {
			return
		}
	}
}
func (s *Server) reply(sess *session, id json.RawMessage, result any, err error) bool {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	response := map[string]any{"jsonrpc": "2.0", "id": id}
	if err != nil {
		var rpc *rpcError
		if !errors.As(err, &rpc) {
			rpc = &rpcError{-32603, "本地操作失败，请在 Cue 中查看状态"}
		}
		response["error"] = rpc
	} else {
		response["result"] = result
	}
	data, err := json.Marshal(response)
	if err != nil {
		return false
	}
	if len(data) > maxFrame {
		return s.reply(sess, id, nil, rpcErr(-32603, "响应超过大小限制"))
	}
	_ = sess.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = sess.conn.Write(append(data, '\n'))
	return err == nil
}
func stringParam(params map[string]json.RawMessage, name string) string {
	var v string
	_ = json.Unmarshal(params[name], &v)
	return v
}
func (s *Server) dispatch(sess *session, method string, params map[string]json.RawMessage) (any, error) {
	if method == "connect" {
		return s.connect(sess, params)
	}
	if !sess.connected {
		return nil, rpcErr(-32000, "请先调用 connect")
	}
	switch method {
	case "describe":
		return map[string]any{"protocolVersion": "1", "instanceId": s.instance.InstanceID, "app": appInfo(), "scopes": []string{"read", "action"}, "granted": scopeList(sess.scopes), "methods": []string{"describe", "get_state", "get_recent_events", "get_last_error", "capture_diagnostics", "list_actions", "invoke_action"}, "actions": actionList()}, nil
	case "list_actions":
		return map[string]any{"actions": actionList()}, nil
	case "invoke_action":
		if err := s.require(sess, "action"); err != nil {
			return nil, err
		}
		switch stringParam(params, "name") {
		case "set_capturing":
			var args struct {
				Active *bool `json:"active"`
			}
			if json.Unmarshal(params["arguments"], &args) != nil || args.Active == nil {
				return nil, rpcErr(-32602, "arguments.active 必须是布尔值")
			}
			if s.deps.SetCapturing == nil {
				return nil, rpcErr(-32603, "听写控制暂不可用")
			}
			if err := s.deps.SetCapturing(*args.Active); err != nil {
				s.record("error", "本地调用方未能改变听写状态")
				return nil, err
			}
			s.record("info", "本地调用方改变了听写状态")
			return map[string]any{"result": map[string]any{"capturing": *args.Active}}, nil
		case "get_slides":
			if !s.resolveConsent(sess.callerID, s.callerName(sess.callerID), "slides") {
				return nil, rpcErr(-32001, "用户未授权此程序读取幻灯片内容")
			}
			if err := s.require(sess, "action"); err != nil {
				return nil, err
			}
			var slides []model.Slide
			if s.deps.Slides != nil {
				slides = s.deps.Slides()
			}
			if slides == nil {
				slides = []model.Slide{}
			}
			s.record("info", "已向获授权的本地调用方提供幻灯片描述")
			return map[string]any{"result": map[string]any{"count": len(slides), "slides": slides}}, nil
		default:
			return nil, rpcErr(-32602, "未知的本地操作")
		}
	case "get_state", "get_recent_events", "get_last_error", "capture_diagnostics":
		if err := s.require(sess, "read"); err != nil {
			return nil, err
		}
		switch method {
		case "get_state":
			return map[string]any{"state": s.safeState(), "at": time.Now().UTC().Format(time.RFC3339Nano)}, nil
		case "get_recent_events":
			events, seq := s.recent(params)
			return map[string]any{"events": events, "sequence": seq}, nil
		case "get_last_error":
			return map[string]any{"event": s.lastError()}, nil
		default:
			events, _ := s.recent(params)
			return map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339Nano), "app": appInfo(), "environment": map[string]string{"os": runtime.GOOS, "arch": runtime.GOARCH, "runtime": runtime.Version()}, "state": s.safeState(), "lastError": s.lastError(), "events": events}, nil
		}
	default:
		return nil, rpcErr(-32601, "未知的 JSON-RPC 方法")
	}
}

func (s *Server) connect(sess *session, params map[string]json.RawMessage) (any, error) {
	if sess.connected {
		return nil, rpcErr(-32600, "此连接已经完成握手")
	}
	want, got := sha256.Sum256([]byte(s.instance.Token)), sha256.Sum256([]byte(stringParam(params, "token")))
	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		return nil, rpcErr(-32002, "连接令牌无效")
	}
	var client struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if json.Unmarshal(params["client"], &client) != nil || client.ID == "" || len(client.ID) > 200 || strings.ContainsAny(client.ID, "\r\n\x00") {
		return nil, rpcErr(-32602, "需要有效的 client.id")
	}
	if client.Name == "" {
		client.Name = client.ID
	}
	if len(client.Name) > 200 || strings.ContainsAny(client.Name, "\r\n\x00") {
		return nil, rpcErr(-32602, "client.name 无效")
	}
	scopes := []string{"read"}
	if p, exists := params["scopes"]; exists {
		if json.Unmarshal(p, &scopes) != nil {
			return nil, rpcErr(-32602, "scopes 必须是数组")
		}
	}
	s.mu.Lock()
	sess.callerID = client.ID
	s.mu.Unlock()
	granted, denied := []string{}, []string{}
	for _, scope := range []string{"read", "action"} {
		requested := false
		for _, value := range scopes {
			if value == scope {
				requested = true
				break
			}
		}
		if !requested {
			continue
		}
		if s.resolveConsent(client.ID, client.Name, scope) {
			sess.scopes[scope] = true
			granted = append(granted, scope)
		} else {
			denied = append(denied, scope)
		}
	}
	sess.connected = true
	s.record("info", "本地调用方完成授权握手")
	return map[string]any{"protocolVersion": "1", "instanceId": s.instance.InstanceID, "app": appInfo(), "granted": granted, "denied": denied, "verification": "token"}, nil
}
func appInfo() map[string]string {
	return map[string]string{"id": AppID, "slug": "cue-remix", "name": "Cue", "version": "0.1.0"}
}
func scopeList(scopes map[string]bool) []string {
	out := []string{}
	for k, v := range scopes {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
func actionList() []map[string]any {
	return []map[string]any{
		{"name": "set_capturing", "description": "开始或停止听写", "scope": "action", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"active": map[string]string{"type": "boolean"}}, "required": []string{"active"}}},
		{"name": "get_slides", "description": "读取本次会议的幻灯片描述，另需内容授权", "scope": "action", "inputSchema": map[string]any{"type": "object"}},
	}
}
func (s *Server) require(sess *session, scope string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !sess.scopes[scope] || s.callers[sess.callerID].Scopes[scope] != "granted" {
		return rpcErr(-32001, "用户未授权此程序执行该操作")
	}
	return nil
}
func (s *Server) safeState() map[string]any {
	status := model.Status{}
	if s.deps.Status != nil {
		status = s.deps.Status()
	}
	return map[string]any{"listening": status.Listening, "busy": status.Busy, "platform": runtime.GOOS, "transcriptTurns": len(status.Transcript), "slideCount": len(status.Slides), "warningCount": len(status.Warnings)}
}
func (s *Server) record(level, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	s.events = append(s.events, logEvent{s.sequence, time.Now().UTC().Format(time.RFC3339Nano), level, message})
	if len(s.events) > 256 {
		s.events = append([]logEvent(nil), s.events[len(s.events)-256:]...)
	}
}
func (s *Server) recent(params map[string]json.RawMessage) ([]logEvent, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := 50
	_ = json.Unmarshal(params["limit"], &limit)
	limit = max(1, min(256, limit))
	var after int64
	_ = json.Unmarshal(params["afterSeq"], &after)
	levels := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3, "fatal": 4}
	floor := levels[stringParam(params, "minLevel")]
	since, _ := time.Parse(time.RFC3339Nano, stringParam(params, "since"))
	out := []logEvent{}
	for _, event := range s.events {
		if event.Seq <= after || levels[event.Level] < floor {
			continue
		}
		if !since.IsZero() {
			when, _ := time.Parse(time.RFC3339Nano, event.Time)
			if !when.After(since) {
				continue
			}
		}
		out = append(out, event)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, s.sequence
}
func (s *Server) lastError() *logEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].Level == "error" || s.events[i].Level == "fatal" {
			copy := s.events[i]
			return &copy
		}
	}
	return nil
}

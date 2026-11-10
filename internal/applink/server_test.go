package applink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"cue-go-remix/internal/model"
)

type testClient struct {
	conn   net.Conn
	reader *bufio.Reader
	next   int
}
type testResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

func newClient(t *testing.T, s *Server) *testClient {
	t.Helper()
	conn, err := dialLocal(s.instance.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &testClient{conn, bufio.NewReader(conn), 0}
}
func (c *testClient) call(method string, params any) (testResponse, error) {
	c.next++
	_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	b, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.next, "method": method, "params": params})
	if err != nil {
		return testResponse{}, err
	}
	if _, err = c.conn.Write(append(b, '\n')); err != nil {
		return testResponse{}, err
	}
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return testResponse{}, err
	}
	var result testResponse
	err = json.Unmarshal(line, &result)
	return result, err
}
func mustRPC(t *testing.T, c *testClient, method string, params any) testResponse {
	t.Helper()
	r, err := c.call(method, params)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func connectParams(s *Server, id string, scopes ...string) map[string]any {
	return map[string]any{"token": s.instance.Token, "client": map[string]string{"id": id, "name": "自称助手"}, "scopes": scopes}
}
func expectError(t *testing.T, r testResponse, code int) {
	t.Helper()
	if r.Error == nil || r.Error.Code != code {
		t.Fatalf("期望错误 %d，实际 %+v", code, r)
	}
}
func startTestServer(t *testing.T, dir string, deps Dependencies) *Server {
	t.Helper()
	s := New(dir, deps)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestAuthenticationAndReadProjection(t *testing.T) {
	var s *Server
	prompts := 0
	s = New(t.TempDir(), Dependencies{
		Emit: func(e model.Event) {
			if e.Type == "consent" {
				prompts++
				s.Consent(e.ID, true)
			}
		},
		Status: func() model.Status {
			return model.Status{Listening: true, Busy: true, DataDir: "private-path", Transcript: []model.Turn{{Text: "private-transcript"}}, Slides: []model.Slide{{Text: "private-slide"}}, Warnings: []string{"private-key"}}
		},
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := newClient(t, s)
	expectError(t, mustRPC(t, c, "get_state", map[string]any{}), -32000)
	bad := connectParams(s, "test.reader", "read")
	bad["token"] = "wrong"
	expectError(t, mustRPC(t, c, "connect", bad), -32002)
	if prompts != 0 {
		t.Fatal("无效令牌触发授权")
	}
	r := mustRPC(t, c, "connect", connectParams(s, "test.reader", "read"))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	r = mustRPC(t, c, "get_state", map[string]any{})
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if strings.Contains(string(r.Result), "private-") {
		t.Fatalf("状态泄露内容：%s", r.Result)
	}
	var state struct {
		State map[string]any `json:"state"`
	}
	if err := json.Unmarshal(r.Result, &state); err != nil {
		t.Fatal(err)
	}
	if state.State["transcriptTurns"] != float64(1) || state.State["slideCount"] != float64(1) {
		t.Fatal("状态计数错误")
	}
	r = mustRPC(t, c, "capture_diagnostics", map[string]any{})
	if strings.Contains(string(r.Result), "private-") {
		t.Fatal("诊断泄露内容")
	}
	expectError(t, mustRPC(t, c, "invoke_action", map[string]any{"name": "set_capturing", "arguments": map[string]bool{"active": true}}), -32001)
	if prompts != 1 {
		t.Fatalf("授权次数错误：%d", prompts)
	}
	if got := s.ListCallers(); len(got) != 1 || !reflect.DeepEqual(got[0].Scopes, []string{"read"}) {
		t.Fatalf("授权列表错误：%+v", got)
	}
}

func TestActionAndSlideConsentAreSeparate(t *testing.T) {
	var s *Server
	var scopes []string
	var captured bool
	s = New(t.TempDir(), Dependencies{
		Emit: func(e model.Event) {
			if e.Type == "consent" {
				request := e.Data.(ConsentRequest)
				scopes = append(scopes, request.Scope)
				s.Consent(e.ID, request.Scope != "slides")
			}
		},
		SetCapturing: func(active bool) error { captured = active; return nil },
		Slides:       func() []model.Slide { return []model.Slide{{Text: "不得在未授权时披露"}} },
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := newClient(t, s)
	r := mustRPC(t, c, "connect", connectParams(s, "test.actions", "read", "action"))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	r = mustRPC(t, c, "invoke_action", map[string]any{"name": "set_capturing", "arguments": map[string]bool{"active": true}})
	if r.Error != nil || !captured {
		t.Fatal("授权后的听写控制失败")
	}
	r = mustRPC(t, c, "invoke_action", map[string]any{"name": "get_slides", "arguments": map[string]any{}})
	expectError(t, r, -32001)
	if !reflect.DeepEqual(scopes, []string{"read", "action", "slides"}) {
		t.Fatalf("没有独立询问幻灯片权限：%v", scopes)
	}
	if strings.Contains(string(r.Result), "不得") {
		t.Fatal("未获内容授权仍返回幻灯片")
	}
	expectError(t, mustRPC(t, c, "invoke_action", map[string]any{"name": "set_capturing", "arguments": map[string]any{"active": "true"}}), -32602)
}

func TestConsentPersistenceAndRevocation(t *testing.T) {
	dir := t.TempDir()
	var s *Server
	s = New(dir, Dependencies{Emit: func(e model.Event) {
		if e.Type == "consent" {
			s.Consent(e.ID, true)
		}
	}})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	c := newClient(t, s)
	if r := mustRPC(t, c, "connect", connectParams(s, "test.persist", "read")); r.Error != nil {
		t.Fatal(r.Error)
	}
	s.Close()
	if _, err := os.Stat(filepath.Join(dir, "applink", "run.json")); !os.IsNotExist(err) {
		t.Fatal("退出后残留运行信息")
	}
	prompts := 0
	s2 := New(dir, Dependencies{Emit: func(e model.Event) {
		if e.Type == "consent" {
			prompts++
		}
	}})
	if err := s2.Start(); err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	c2 := newClient(t, s2)
	if r := mustRPC(t, c2, "connect", connectParams(s2, "test.persist", "read")); r.Error != nil {
		t.Fatal(r.Error)
	}
	if prompts != 0 {
		t.Fatal("已持久化授权仍然询问")
	}
	if err := s2.Revoke("test.persist"); err != nil {
		t.Fatal(err)
	}
	if _, err := c2.call("get_state", map[string]any{}); err == nil {
		t.Fatal("撤销后旧连接未关闭")
	}
	if len(s2.ListCallers()) != 0 {
		t.Fatal("撤销后仍保存授权")
	}
	data, err := os.ReadFile(filepath.Join(dir, "applink", "consent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "test.persist") {
		t.Fatal("撤销没有持久化")
	}
}

func TestSingleVisibleConsentAndUnknownAnswers(t *testing.T) {
	requests := make(chan model.Event, 10)
	s := startTestServer(t, t.TempDir(), Dependencies{Emit: func(e model.Event) {
		if e.Type == "consent" {
			requests <- e
		}
	}})
	c1, c2 := newClient(t, s), newClient(t, s)
	complete := make(chan error, 1)
	go func() {
		r, err := c1.call("connect", connectParams(s, "first", "read"))
		if err == nil && r.Error != nil {
			err = r.Error
		}
		complete <- err
	}()
	var event model.Event
	select {
	case event = <-requests:
	case <-time.After(5 * time.Second):
		t.Fatal("未发起授权")
	}
	s.Consent("stale-id", true)
	s.mu.Lock()
	pendingCount := len(s.pending)
	s.mu.Unlock()
	if pendingCount != 1 {
		t.Fatal("过期回答改变了当前授权")
	}
	r := mustRPC(t, c2, "connect", connectParams(s, "second", "read"))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(string(r.Result), `"denied":["read"]`) {
		t.Fatalf("忙碌时应拒绝新授权：%s", r.Result)
	}
	select {
	case <-requests:
		t.Fatal("并发请求覆盖了授权弹窗")
	default:
	}
	s.Consent(event.ID, true)
	if err := <-complete; err != nil {
		t.Fatal(err)
	}
	if got := s.ListCallers(); len(got) != 1 || got[0].ID != "first" {
		t.Fatalf("忙碌拒绝不应持久化：%v", got)
	}
}

func TestLogsAreBoundedAndDoNotIncludeInputs(t *testing.T) {
	s := New(t.TempDir(), Dependencies{})
	for i := 0; i < 400; i++ {
		s.record("info", fmt.Sprint(i))
	}
	events, seq := s.recent(map[string]json.RawMessage{"limit": json.RawMessage("999")})
	if len(events) != 256 || seq != 400 || events[0].Seq != 145 {
		t.Fatal("日志窗口没有上限")
	}
	s.record("error", "执行失败")
	if got := s.lastError(); got == nil || got.Message != "执行失败" {
		t.Fatal("无法查询最后错误")
	}
}

func TestCloseUnblocksPendingConsent(t *testing.T) {
	requests := make(chan model.Event, 1)
	s := startTestServer(t, t.TempDir(), Dependencies{Emit: func(e model.Event) {
		if e.Type == "consent" {
			requests <- e
		}
	}})
	c := newClient(t, s)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = c.call("connect", connectParams(s, "pending", "read")) }()
	select {
	case <-requests:
	case <-time.After(5 * time.Second):
		t.Fatal("未发起授权")
	}
	s.Close()
	wg.Wait()
}

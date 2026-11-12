package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cue-go-remix/internal/entitlement"
	secure "cue-go-remix/internal/entitlement/platform"
	"cue-go-remix/internal/model"
)

func TestLockedAppRejectsModelAndAudioWork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	a := testApp(t, server.URL)
	if _, err := a.ForgetLicense(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"answerThis", "assist", "say", "leetcode", "recap"} {
		if err := a.Ask(model.AskRequest{Mode: mode, Text: "测试问题"}); !errors.Is(err, entitlement.ErrLocked) {
			t.Fatal(mode, err)
		}
	}
	if err := a.StartListening(); !errors.Is(err, entitlement.ErrLocked) {
		t.Fatal(err)
	}
	appendTurns(t, a, 1, 4)
	a.makeNotes(*a.store.Current())
	_ = a.StopListening()
	awaitTask(t, a)
	if calls.Load() != 0 || a.GetStatus().Listening || a.GetStatus().Busy {
		t.Fatal("未授权触发了采集或模型请求")
	}
}

func TestRevocationCancelsAnswerAndNotes(t *testing.T) {
	entered := make(chan struct{}, 2)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	a := testApp(t, server.URL)
	appendTurns(t, a, 1, 4)
	if err := a.Ask(model.AskRequest{Mode: "answerThis", Text: "问题"}); err != nil {
		t.Fatal(err)
	}
	a.makeNotes(*a.store.Current())
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("任务没有开始")
		}
	}
	if _, err := a.ForgetLicense(); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, a)
	if calls.Load() != 2 || a.GetStatus().Busy || a.store.Current().Notes != "" {
		t.Fatal("撤销授权未清理后台任务")
	}
}

func TestOldSessionCannotBorrowNewAuthorizationForNotes(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	a := testApp(t, server.URL)
	appendTurns(t, a, 1, 4)
	old, _ := a.authorizedContext()
	a.sessionLicense = old
	if _, err := a.ForgetLicense(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ActivateLicense("fixture-card"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.authorizedContext(); err != nil {
		t.Fatal(err)
	}
	// 模拟旧音频清理直到重新激活后才结束。
	a.listenMu.Lock()
	a.stopListeningLocked(true)
	a.listenMu.Unlock()
	awaitTask(t, a)
	if calls.Load() != 0 {
		t.Fatal("旧会话借新授权自动发送纪要")
	}
}

// These tests use the application service without creating a Wails window or
// requesting microphone/screen permissions. All model calls go to localhost.
func testApp(t *testing.T, base string) *Runtime {
	t.Helper()
	t.Setenv("CUE_DATA_DIR", t.TempDir())
	a, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	a.license.Close()
	a.license = testLicense(t, a.ctx)
	v := a.GetSettings()
	v.Chat = model.Endpoint{Provider: "custom", BaseURL: base, Model: "relay-fast", SmartModel: "relay-smart", Stream: false}
	if _, err = a.SaveSettings(v); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Shutdown)
	return a
}

func awaitTask(t *testing.T, a *Runtime) {
	t.Helper()
	done := make(chan struct{})
	go func() { a.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("应用后台任务未结束")
	}
}

func TestAskRelayCancelClearsBusyAndAllowsNextQuestion(t *testing.T) {
	entered := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gateway/v1/chat/completions" {
			t.Errorf("中转路径错误 %s", r.URL.Path)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "relay-smart" || len(body.Messages) != 2 || !strings.Contains(body.Messages[1].Content, "解释事务隔离") {
			t.Errorf("实际请求错误 %+v", body)
		}
		if requests.Add(1) == 1 {
			close(entered)
			<-r.Context().Done()
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"隔离级别说明"}}]}`)
	}))
	defer server.Close()
	a := testApp(t, server.URL+"/gateway/v1")
	v := a.GetSettings()
	v.Smart = true
	if _, err := a.SaveSettings(v); err != nil {
		t.Fatal(err)
	}
	request := model.AskRequest{Mode: "answerThis", Text: "解释事务隔离"}
	if err := a.Ask(request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("模型未收到请求")
	}
	if err := a.Ask(request); err == nil {
		t.Fatal("忙碌期间应拒绝第二次请求")
	}
	a.Cancel()
	awaitTask(t, a)
	if a.GetStatus().Busy {
		t.Fatal("取消后busy未清除")
	}
	if err := a.Ask(request); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, a)
	if requests.Load() != 2 || a.GetStatus().Busy {
		t.Fatal("取消后未能发起新请求")
	}
}

func TestShutdownCancelsPendingAnswerAndRejectsNewWork(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	a := testApp(t, server.URL)
	if err := a.Ask(model.AskRequest{Mode: "answerThis", Text: "解释数据库"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("模型未收到请求")
	}
	done := make(chan struct{})
	go func() { a.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("退出没有取消挂起的网络请求")
	}
	if err := a.Ask(model.AskRequest{Mode: "answerThis", Text: "再问一次"}); err == nil {
		t.Fatal("退出中不应接收新任务")
	}
}

func appendTurns(t *testing.T, a *Runtime, start, count int) {
	t.Helper()
	for i := start; i < start+count; i++ {
		if _, err := a.store.Append("them", fmt.Sprintf("会议事项%d", i)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNotesCatchUpAfterMorePausedTranscriptArrives(t *testing.T) {
	first := make(chan struct{})
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if n == 1 {
			close(first)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":"会议纪要%d"}}]}`, n)
	}))
	defer server.Close()
	a := testApp(t, server.URL)
	appendTurns(t, a, 1, 4)
	m := a.store.Current()
	a.makeNotes(*m)
	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("未开始生成纪要")
	}
	appendTurns(t, a, 5, 2)
	// An overlapping stop request is coalesced while the first summary runs.
	a.makeNotes(*a.store.Current())
	close(release)
	awaitTask(t, a)
	latest := a.store.Current()
	if latest.NotesTurns != 6 || latest.Notes != "会议纪要2" || requests.Load() != 2 {
		t.Fatalf("纪要未覆盖新增转录：%+v，请求%d", latest, requests.Load())
	}
}

func TestActiveMeetingDoesNotContinuouslyRegenerateNotes(t *testing.T) {
	first := make(chan struct{})
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if n == 1 {
			close(first)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":"会议纪要%d"}}]}`, n)
	}))
	defer server.Close()
	a := testApp(t, server.URL)
	appendTurns(t, a, 1, 4)
	a.makeNotes(*a.store.Current())
	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("未开始生成纪要")
	}
	a.mu.Lock()
	a.listening = true
	a.mu.Unlock()
	appendTurns(t, a, 5, 2)
	close(release)
	awaitTask(t, a)
	if requests.Load() != 1 {
		t.Fatalf("活跃会议不应不断补生成纪要，实际%d次", requests.Load())
	}
	if m := a.store.Current(); m.NotesTurns != 4 {
		t.Fatalf("纪要覆盖计数不应冒充最新：%d", m.NotesTurns)
	}
	if err := a.StopListening(); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, a)
	if m := a.store.Current(); m.NotesTurns != 6 || requests.Load() != 2 {
		t.Fatalf("下次停止应补齐纪要：%d次，覆盖%d", requests.Load(), m.NotesTurns)
	}
}

// 既有业务测试使用真正的授权管理器和本地协议服务，不新增生产绕过开关。
func testLicense(t *testing.T, ctx context.Context) *entitlement.Manager {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UnixMilli()
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"appStatus": 2, "mode": "license", "token": "local-fixture", "valid": true, "serverTime": now, "expiredAt": now + 86400000}})
	}))
	t.Cleanup(server.Close)
	m := entitlement.New(ctx, entitlement.Options{BaseURL: server.URL, AllowHTTP: true, MachineID: func() (string, error) { return "fixture", nil }, Load: func() (secure.Credential, error) { return secure.Credential{}, secure.ErrNotFound }, Save: func(secure.Credential) error { return nil }, Delete: func() error { return nil }})
	if err := m.Activate("fixture-only"); err != nil {
		t.Fatal(err)
	}
	return m
}

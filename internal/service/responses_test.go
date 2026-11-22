package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cue-go-remix/internal/model"
)

func TestResponsesChannelProbeAndAskUseSameProtocol(t *testing.T) {
	paths := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/responses" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":{"message":"This gateway only supports /v1/responses; /v1/chat/completions is disabled"}}`)
			return
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request["stream"] != true || r.Header.Get("Authorization") != "Bearer chat-secret-A" {
			t.Error("检测和问答必须沿用渠道的流式模式与密钥")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n")
	}))
	defer server.Close()
	a := testChannelRuntime(t, server.URL+"/v1")
	draft := a.GetSettings().ChatChannels[0]
	failed, err := a.CheckChannelEndpoint("chat", draft)
	if err != nil || failed.Success || !strings.Contains(failed.Message, "接口协议不兼容") {
		t.Fatalf("原协议检测应提示协议不匹配：%+v %v", failed, err)
	}
	draft.ChatProtocol, draft.Stream = model.ChatProtocolResponses, true
	result, err := a.CheckChannelEndpoint("chat", draft)
	if err != nil || !result.Success {
		t.Fatalf("Responses 草稿检测失败：%+v %v", result, err)
	}
	if a.GetSettings().Chat.ChatProtocol != "" {
		t.Fatal("连接检测不应保存草稿")
	}
	s := a.GetSettings()
	s.ChatChannels[0] = draft
	if _, err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.Ask(model.AskRequest{Mode: "answerThis", Text: "fixture-question"}); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, a)
	if len(paths) != 3 {
		t.Fatalf("应只有三次显式请求，得到 %d 次", len(paths))
	}
	for _, want := range []string{"/v1/chat/completions", "/v1/responses", "/v1/responses"} {
		if got := <-paths; got != want {
			t.Fatalf("请求路径错误：%s", got)
		}
	}
	data, err := os.ReadFile(filepath.Join(a.DataDir(), "logs", "cue.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"protocol=chat-completions", "protocol=responses", "reason=http-403 category=protocol-mismatch", "event=answer-done"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("日志缺少 %s", want)
		}
	}
	for _, secret := range []string{"chat-secret-A", "fixture-question", server.URL} {
		if strings.Contains(string(data), secret) {
			t.Fatal("协议诊断日志不得泄露密钥、地址或正文")
		}
	}
	report, err := a.ExportDiagnostics()
	if err != nil || !strings.Contains(report, "protocol=responses") || !strings.Contains(report, "category=protocol-mismatch") {
		t.Fatalf("导出必须保留固定协议和错误分类：%v", err)
	}
}

func TestResponsesProbeWaitsForSlowFirstResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(10100 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n")
	}))
	defer server.Close()
	a := testChannelRuntime(t, server.URL+"/v1")
	ep := a.GetSettings().ChatChannels[0]
	ep.ChatProtocol, ep.Stream = model.ChatProtocolResponses, true
	result, err := a.CheckChannelEndpoint("chat", ep)
	if err != nil || !result.Success {
		t.Fatalf("检测不应在首个响应到达前于 10 秒误报超时：%+v %v", result, err)
	}
}

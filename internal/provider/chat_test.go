package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cue-go-remix/internal/model"
)

func TestCustomChatJSONAndRequest(t *testing.T) {
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gateway/v1/chat/completions" {
			t.Errorf("路径 %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer relay-secret" {
			t.Error("缺少中转鉴权")
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"你好，中转站"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	var tokens strings.Builder
	answer, err := Chat(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL + "/gateway/v1/", APIKey: "relay-secret", Model: "my-model"}, Request{System: "中文", Text: "你好", Image: "data:image/jpeg;base64,YQ=="}, func(s string) { tokens.WriteString(s) }, nil)
	if err != nil || answer != "你好，中转站" || tokens.String() != answer {
		t.Fatalf("%q %v", answer, err)
	}
	if request["model"] != "my-model" || request["stream"] != false {
		t.Fatalf("请求模型/stream: %#v", request)
	}
	messages := list(request["messages"])
	if len(messages) != 2 || object(messages[0])["content"] != "中文" {
		t.Fatal("系统提示词丢失")
	}
	parts := list(object(messages[1])["content"])
	if len(parts) != 2 || object(parts[1])["type"] != "image_url" {
		t.Fatal("截图请求丢失")
	}
}

func TestStreamFragmentedSSEAndJSONFallback(t *testing.T) {
	for _, jsonFallback := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonFallback), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("重复接口路径 %s", r.URL.Path)
				}
				if jsonFallback {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"你好"}}]}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				flusher := w.(http.Flusher)
				parts := []string{": heartbeat\r\n\r\n", "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"hidden\"}}]}\r\n\r\n", "data: {\"choices\":[{\"delta\":{\"content\":\"你", "好\"}}]}\n\n", "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n", "data: [DONE]\n\n"}
				for _, p := range parts {
					_, _ = io.WriteString(w, p)
					flusher.Flush()
				}
			}))
			defer server.Close()
			var tokens strings.Builder
			activity := 0
			answer, err := Chat(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL + "/v1/chat/completions", Model: "test", Stream: true}, Request{Text: "hello"}, func(s string) { tokens.WriteString(s) }, func() { activity++ })
			if err != nil || answer != "你好" || tokens.String() != answer {
				t.Fatalf("%q %v", answer, err)
			}
			if activity == 0 {
				t.Error("没有流式活动通知")
			}
		})
	}
}

func TestStreamCancellationPreservesPartialAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"部分\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	answer, err := Chat(ctx, model.Endpoint{Provider: "custom", BaseURL: server.URL, Model: "test", Stream: true}, Request{}, func(string) { cancel() }, nil)
	if answer != "部分" || !errors.Is(err, context.Canceled) {
		t.Fatalf("%q %v", answer, err)
	}
}

func TestRelayErrorsRedactSecrets(t *testing.T) {
	key := "supersecret-relay-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Rejected Bearer " + key, "code": "invalid_api_key"}})
	}))
	defer server.Close()
	_, err := Chat(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL, APIKey: key, Model: "test"}, Request{}, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 401 || strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "API Key 无效") {
		t.Fatalf("错误未正确脱敏: %v", err)
	}
}

func TestProviderNativeProtocols(t *testing.T) {
	for _, p := range []string{"anthropic", "gemini"} {
		t.Run(p, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var v map[string]any
				_ = json.NewDecoder(r.Body).Decode(&v)
				w.Header().Set("Content-Type", "text/event-stream")
				if p == "anthropic" {
					if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "key" || r.Header.Get("anthropic-version") == "" {
						t.Error("Anthropic 请求错误")
					}
					if len(list(object(list(v["messages"])[0])["content"])) != 2 {
						t.Error("Anthropic 图片缺失")
					}
					_, _ = io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"答案\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
				} else {
					if r.URL.Path != "/v1beta/models/gemini-test:streamGenerateContent" || r.URL.Query().Get("alt") != "sse" || r.Header.Get("x-goog-api-key") != "key" {
						t.Errorf("Gemini 请求错误 %s", r.URL)
					}
					_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"内部推理\"},{\"text\":\"答案\"}]},\"finishReason\":\"STOP\"}]}\n\n")
				}
			}))
			defer server.Close()
			base := server.URL + "/v1"
			if p == "gemini" {
				base = server.URL + "/v1beta"
			}
			answer, err := Chat(context.Background(), model.Endpoint{Provider: p, BaseURL: base, APIKey: "key", Model: "gemini-test", Stream: true}, Request{Text: "问题", Image: "data:image/png;base64,YQ=="}, nil, nil)
			if err != nil || answer != "答案" {
				t.Fatalf("%q %v", answer, err)
			}
		})
	}
}

func TestIncompleteAndEmptyResponsesFail(t *testing.T) {
	for name, response := range map[string]string{"incomplete": "data: {\"choices\":[{\"delta\":{\"content\":\"部分\"}}]}\n\n", "empty": "data: [DONE]\n\n", "malformed": "data: oops\n\n"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()
			_, err := Chat(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL, Model: "test", Stream: true}, Request{}, nil, nil)
			if err == nil {
				t.Error("应报告不完整/空/无效响应")
			}
		})
	}
}

func TestNoCredentialRedirect(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := Chat(ctx, model.Endpoint{Provider: "custom", BaseURL: server.URL, APIKey: "key", Model: "test"}, Request{}, nil, nil)
	if err == nil || called {
		t.Fatalf("不能跟随重定向泄露凭据 %v", err)
	}
}

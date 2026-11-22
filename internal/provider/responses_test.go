package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cue-go-remix/internal/model"
)

func TestResponsesOnlyGateway(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/gateway/v1/responses" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":{"message":"This gateway only supports /v1/responses; /v1/chat/completions is disabled"}}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("Responses 请求未使用渠道密钥")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request["instructions"] != "系统指令" || request["model"] != "fixture-model" || request["store"] != false || request["stream"] != false || request["messages"] != nil {
			t.Errorf("Responses 请求字段错误：%#v", request)
		}
		parts := list(object(list(request["input"])[0])["content"])
		if len(parts) != 2 || object(parts[0])["type"] != "input_text" || object(parts[0])["text"] != "问题" || object(parts[1])["type"] != "input_image" || object(parts[1])["image_url"] != "data:image/png;base64,YQ==" {
			t.Errorf("Responses 文本或截图编码错误：%#v", request)
		}
		_, _ = io.WriteString(w, `{"status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"不展示的推理"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"正确回答"}]}]}`)
	}))
	defer server.Close()
	ep := model.Endpoint{Provider: "custom", BaseURL: server.URL + "/gateway/v1", APIKey: "fixture-secret", Model: "fixture-model"}
	_, err := Chat(context.Background(), ep, Request{Text: "问题"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "接口协议不兼容") || !strings.Contains(err.Error(), "Responses") {
		t.Errorf("应明确提示接口协议不兼容，得到 %v", err)
	}
	if calls != 1 {
		t.Fatal("协议不匹配时不得自动重试计费请求")
	}
	ep.ChatProtocol = model.ChatProtocolResponses
	var tokens strings.Builder
	answer, err := Chat(context.Background(), ep, Request{System: "系统指令", Text: "问题", Image: "data:image/png;base64,YQ=="}, func(s string) { tokens.WriteString(s) }, nil)
	if err != nil || answer != "正确回答" || tokens.String() != answer {
		t.Fatalf("Responses 网关调用失败：%q %v", answer, err)
	}
}

func TestResponsesStreamingAndJSONFallback(t *testing.T) {
	for name, body := range map[string]string{
		"逐字流":    "event: response.created\ndata: {\"type\":\"response.created\"}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"你\"}\n\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"不展示的推理\"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"好\"}\n\ndata: {\"type\":\"response.output_text.done\",\"text\":\"你好\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"你好\"}]}]}}\n\n",
		"只终态":    "event: response.completed\ndata: {\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"你好\"}]}]}}\n\n",
		"返回JSON": `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"你好"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/responses" {
					t.Errorf("完整 Responses 地址不得重复拼接：%s", r.URL.Path)
				}
				if name == "返回JSON" {
					w.Header().Set("Content-Type", "application/json")
				} else {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			var tokens strings.Builder
			answer, err := Chat(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL + "/v1/responses", Model: "fixture", ChatProtocol: model.ChatProtocolResponses, Stream: true}, Request{Text: "你好"}, func(s string) { tokens.WriteString(s) }, nil)
			if err != nil || answer != "你好" || tokens.String() != answer {
				t.Fatalf("终态重复或响应错误：%q %q %v", answer, tokens.String(), err)
			}
		})
	}
}

func TestResponsesFailuresAndCancellation(t *testing.T) {
	for name, body := range map[string]string{
		"提前断流":    "data: {\"type\":\"response.output_text.delta\",\"delta\":\"部分\"}\n\n",
		"流内错误":    "event: error\ndata: {\"type\":\"error\",\"message\":\"fixture-secret\",\"code\":\"failed\"}\n\n",
		"失败终态":    "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"fixture-secret\",\"code\":\"failed\"}}}\n\n",
		"不完整终态":   "data: {\"type\":\"response.incomplete\",\"response\":{\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"}}}\n\n",
		"失败JSON":  `{"status":"failed","error":{"message":"fixture-secret"}}`,
		"不完整JSON": `{"status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"部分"}]}]}`,
		"空回答":     `{"status":"completed","output":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(name, "JSON") || name == "空回答" {
					w.Header().Set("Content-Type", "application/json")
				} else {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			_, err := Chat(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL, Model: "fixture", APIKey: "fixture-secret", ChatProtocol: model.ChatProtocolResponses, Stream: true}, Request{}, nil, nil)
			if err == nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatalf("错误应如实报告且脱敏：%v", err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"部分\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	answer, err := Chat(ctx, model.Endpoint{Provider: "custom", BaseURL: server.URL, Model: "fixture", ChatProtocol: model.ChatProtocolResponses, Stream: true}, Request{}, func(string) { cancel() }, nil)
	if answer != "部分" || !errors.Is(err, context.Canceled) {
		t.Fatalf("取消应保留已生成回答：%q %v", answer, err)
	}
}

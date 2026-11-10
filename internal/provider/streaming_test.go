package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cue-go-remix/internal/model"
	"github.com/coder/websocket"
)

func TestOpenAIStreamingDrainsFinalWords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/realtime" || r.URL.Query().Get("intent") != "transcription" || r.Header.Get("Authorization") != "Bearer speech-key" {
			t.Error("流式地址或鉴权错误")
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var setup map[string]any
		_ = json.Unmarshal(data, &setup)
		if setup["type"] != "session.update" {
			t.Error("缺少实时会话配置")
		}
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"session.created"}`))
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"session.updated"}`))
		_, data, err = conn.Read(r.Context())
		if err != nil {
			return
		}
		var appendEvent map[string]any
		_ = json.Unmarshal(data, &appendEvent)
		if appendEvent["type"] != "input_audio_buffer.append" {
			t.Error("缺少音频事件")
		}
		pcm, _ := base64.StdEncoding.DecodeString(stringValue(appendEvent["audio"]))
		if len(pcm) != 4800 {
			t.Errorf("应重采样成24k: %d", len(pcm))
		}
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"conversation.item.input_audio_transcription.delta","delta":"最后"}`))
		_, data, err = conn.Read(r.Context())
		if err != nil {
			return
		}
		if !strings.Contains(string(data), "input_audio_buffer.commit") {
			t.Error("结束时未提交尾段")
		}
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"input_audio_buffer.committed","item_id":"item-1"}`))
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"conversation.item.input_audio_transcription.completed","item_id":"item-1","transcript":"最后一句话"}`))
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	var mu sync.Mutex
	var final []string
	var callbackErrors []error
	s, err := OpenStream(context.Background(), model.Endpoint{Provider: "openai", BaseURL: server.URL + "/v1", APIKey: "speech-key", Model: "gpt-4o-mini-transcribe"}, "zh", func(text string, done bool) {
		mu.Lock()
		defer mu.Unlock()
		if done {
			final = append(final, text)
		}
	}, func(err error) { mu.Lock(); defer mu.Unlock(); callbackErrors = append(callbackErrors, err) })
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Send(make([]int16, 1600)); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(final) != 1 || final[0] != "最后一句话" {
		t.Fatalf("尾句丢失 %v", final)
	}
	if len(callbackErrors) > 0 {
		t.Fatalf("主动停止不应报连接错误 %v", callbackErrors)
	}
	if err = s.Send([]int16{1}); err == nil {
		t.Error("关闭后不应接收音频")
	}
}

func TestDeepgramStreamingFlushesSegments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token key" {
			t.Error("Deepgram 鉴权错误")
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		kind, _, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		if kind != websocket.MessageBinary {
			t.Error("Deepgram 需要二进制PCM")
		}
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"Results","is_final":true,"speech_final":false,"channel":{"alternatives":[{"transcript":"完成"}]}}`))
		_, _, err = conn.Read(r.Context())
		if err != nil {
			return
		}
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"Results","is_final":true,"speech_final":true,"channel":{"alternatives":[{"transcript":"任务"}]}}`))
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"Metadata"}`))
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	var result string
	s, err := OpenStream(context.Background(), model.Endpoint{Provider: "deepgram", BaseURL: server.URL + "/v1", APIKey: "key"}, "zh", func(text string, final bool) {
		if final {
			result = text
		}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Send([]int16{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if result != "完成 任务" {
		t.Fatalf("转写分片未合并 %q", result)
	}
}

func TestStreamingHandshakeHonoursCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			if _, _, err = conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := OpenStream(ctx, model.Endpoint{Provider: "openai", BaseURL: server.URL, APIKey: "key"}, "auto", nil, nil)
	if err == nil {
		t.Fatal("未配置完成的会话必须响应取消")
	}
}

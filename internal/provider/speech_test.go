package provider

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cue-go-remix/internal/model"
)

func TestCustomTranscriptionMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relay/v1/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer speech-key" {
			t.Errorf("独立语音端点或密钥错误 %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("model") != "whisper-custom" || r.FormValue("language") != "zh" {
			t.Error("语音模型或语言丢失")
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		if len(data) != 50 || string(data[:4]) != "RIFF" || binary.LittleEndian.Uint32(data[24:]) != 16000 || int16(binary.LittleEndian.Uint16(data[46:])) != -123 {
			t.Error("PCM WAV 编码错误")
		}
		_, _ = io.WriteString(w, `{"text":"你好，会议"}`)
	}))
	defer server.Close()
	text, err := Transcribe(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL + "/relay/v1/audio/transcriptions", APIKey: "speech-key", Model: "whisper-custom"}, []int16{0, -123, 123}, "zh")
	if err != nil || text != "你好，会议" {
		t.Fatalf("%q %v", text, err)
	}
}

func TestTranscriptionSilenceRemainsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"text":""}`) }))
	defer server.Close()
	text, err := Transcribe(context.Background(), model.Endpoint{Provider: "custom", BaseURL: server.URL}, []int16{0, 0}, "auto")
	if err != nil || text != "" {
		t.Fatalf("%q %v", text, err)
	}
}

func TestURLSelection(t *testing.T) {
	for _, test := range []struct{ base, provider, want string }{
		{"https://relay.example", "custom", "https://relay.example/v1/chat/completions"},
		{"https://relay.example/api/v1/", "custom", "https://relay.example/api/v1/chat/completions"},
		{"https://relay.example/v1/chat/completions", "custom", "https://relay.example/v1/chat/completions"},
		{"https://relay.example/team%2Fprivate/v1/?tenant=a%2Fb", "custom", "https://relay.example/team%2Fprivate/v1/chat/completions?tenant=a%2Fb"},
		{"https://relay.example/中文 服务/v1", "custom", "https://relay.example/%E4%B8%AD%E6%96%87%20%E6%9C%8D%E5%8A%A1/v1/chat/completions"},
		{"https://azure.example/openai", "azure", "https://azure.example/openai/v1/chat/completions"},
		{"https://azure.example", "azure", "https://azure.example/openai/v1/chat/completions"},
		{"https://azure.example/openai/deployments/model/chat/completions?api-version=2024-10-21", "azure", "https://azure.example/openai/deployments/model/chat/completions?api-version=2024-10-21"},
	} {
		got, err := endpointURL(model.Endpoint{BaseURL: test.base, Provider: test.provider}, "/chat/completions")
		if err != nil || got != test.want {
			t.Errorf("%s => %s %v", test.base, got, err)
		}
	}
	if _, err := endpointURL(model.Endpoint{BaseURL: "https://relay.example/v1/audio/transcriptions"}, "/chat/completions"); err == nil {
		t.Error("不能把语音完整接口拼成聊天路径")
	}
}

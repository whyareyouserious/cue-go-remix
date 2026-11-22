package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cue-go-remix/internal/model"
)

// testChannelRuntime 创建两个独立账号池；同地址、不同密钥用于验证账号之间不借用凭据。
func testChannelRuntime(t *testing.T, baseURL string) *Runtime {
	t.Helper()
	a, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Shutdown)
	channel := func(id, name, key, modelName string) model.APIChannel {
		return model.APIChannel{
			ID: id, Name: name, Status: "untested",
			Endpoint: model.Endpoint{Provider: "custom", BaseURL: baseURL, APIKey: key, Model: modelName, SmartModel: modelName},
		}
	}
	s := a.GetSettings()
	s.ChatChannels = []model.APIChannel{
		channel("shared_a", "聊天账号 A", "chat-secret-A", "chat-model-a"),
		channel("chat_b", "聊天账号 B", "chat-secret-B", "chat-model-b"),
	}
	s.SpeechChannels = []model.APIChannel{
		channel("shared_a", "语音账号 A", "speech-secret-A", "whisper-1"),
		channel("speech_b", "语音账号 B", "speech-secret-B", "whisper-1"),
	}
	s.ActiveChatChannelID, s.ActiveSpeechChannelID = "shared_a", "shared_a"
	if _, err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	return a
}

func receiveChannelAuthorization(t *testing.T, headers <-chan string) string {
	t.Helper()
	select {
	case header := <-headers:
		return header
	case <-time.After(2 * time.Second):
		t.Fatal("渠道测试未收到 HTTP 请求")
		return ""
	}
}

func channelHTTPServer(headers chan<- string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			_, _ = io.WriteString(w, `{"text":"测试转写"}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
}

func TestChannelEndpointUsesSelectedInactiveAccount(t *testing.T) {
	headers := make(chan string, 4)
	server := channelHTTPServer(headers)
	defer server.Close()
	a := testChannelRuntime(t, server.URL)
	s := a.GetSettings()
	for _, test := range []struct {
		name, kind, expected string
		channel              model.APIChannel
	}{
		{"聊天非激活账号", "chat", "Bearer chat-secret-B", s.ChatChannels[1]},
		{"语音非激活账号", "speech", "Bearer speech-secret-B", s.SpeechChannels[1]},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := a.CheckChannelEndpoint(test.kind, test.channel)
			if err != nil || !result.Success {
				t.Fatalf("非激活账号检测失败：%+v，%v", result, err)
			}
			if got := receiveChannelAuthorization(t, headers); got != test.expected {
				t.Fatal("检测请求未使用指定非激活账号的凭据")
			}
		})
	}
	if got := a.GetSettings(); got.ActiveChatChannelID != s.ActiveChatChannelID || got.ActiveSpeechChannelID != s.ActiveSpeechChannelID {
		t.Fatal("检测账号不应切换当前激活服务")
	}
}

func TestChannelEndpointNeverBorrowsUnrelatedCredentials(t *testing.T) {
	headers := make(chan string, 8)
	server := channelHTTPServer(headers)
	defer server.Close()
	otherServer := channelHTTPServer(headers)
	defer otherServer.Close()
	a := testChannelRuntime(t, server.URL)
	saved := a.GetSettings().ChatChannels[0]
	for _, test := range []struct {
		name     string
		kind     string
		mutate   func(*model.APIChannel)
		expected string
	}{
		{"未知账号不借当前密钥", "chat", func(c *model.APIChannel) { c.ID = "new_account" }, ""},
		{"同账号更换地址不带旧密钥", "chat", func(c *model.APIChannel) { c.BaseURL = otherServer.URL }, ""},
		{"显式清除不带旧密钥", "chat", func(c *model.APIChannel) { c.ClearKey = true }, ""},
		{"跨池相同标识只读取本池", "speech", func(c *model.APIChannel) { c.Model = "whisper-1" }, "Bearer speech-secret-A"},
		{"新账号使用显式草稿密钥", "chat", func(c *model.APIChannel) { c.ID, c.APIKey = "new_account", "draft-only-secret" }, "Bearer draft-only-secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := saved
			test.mutate(&candidate)
			result, err := a.CheckChannelEndpoint(test.kind, candidate)
			if err != nil || !result.Success {
				t.Fatalf("本机检测失败：%+v，%v", result, err)
			}
			if got := receiveChannelAuthorization(t, headers); got != test.expected {
				t.Fatal("检测请求凭据隔离不正确")
			}
		})
	}
	// 更换到需要凭据的协议应在发请求前失败，不能沿用原账号的 custom 密钥。
	saved.Provider = "openai"
	result, err := a.CheckChannelEndpoint("chat", saved)
	if err != nil || result.Success || !strings.Contains(result.Message, "API Key") {
		t.Fatalf("更换服务商后应要求重新填写凭据：%+v，%v", result, err)
	}
	if len(headers) != 0 {
		t.Fatal("缺少新服务商凭据时不应发送请求")
	}
	if key, err := a.GetChannelKey("chat", "shared_a"); err != nil || key != "chat-secret-A" {
		t.Fatal("检测草稿不应更改已存凭据")
	}
}

func TestChannelActiveSwitchControlsActualAsk(t *testing.T) {
	headers := make(chan string, 4)
	server := channelHTTPServer(headers)
	defer server.Close()
	a := testChannelRuntime(t, server.URL)
	for _, test := range []struct{ id, expected string }{
		{"shared_a", "Bearer chat-secret-A"},
		{"chat_b", "Bearer chat-secret-B"},
	} {
		s := a.GetSettings()
		s.ActiveChatChannelID = test.id
		if _, err := a.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
		if err := a.Ask(model.AskRequest{Mode: "answerThis", Text: "测试当前账号"}); err != nil {
			t.Fatal(err)
		}
		awaitTask(t, a)
		if got := receiveChannelAuthorization(t, headers); got != test.expected {
			t.Fatal("实际聊天请求未使用已保存的当前账号凭据")
		}
	}
}

func TestChannelSpeechActiveSwitchRejectedWhileListening(t *testing.T) {
	a := testChannelRuntime(t, "http://127.0.0.1:1")
	a.mu.Lock()
	a.listening = true
	a.mu.Unlock()
	t.Cleanup(func() {
		a.mu.Lock()
		a.listening = false
		a.mu.Unlock()
	})
	s := a.GetSettings()
	s.ActiveSpeechChannelID = "speech_b"
	if _, err := a.SaveSettings(s); err == nil || !strings.Contains(err.Error(), "结束会话") {
		t.Fatalf("录音期间仅切换激活 ID 也应被拒绝：%v", err)
	}
	if a.GetSettings().ActiveSpeechChannelID != "shared_a" {
		t.Fatal("被拒绝的语音账号切换已落盘")
	}
	// 只编辑非激活语音账号的展示名，不应影响正在使用的音频端点。
	s = a.GetSettings()
	s.SpeechChannels[1].Name = "非激活语音账号新名称"
	if _, err := a.SaveSettings(s); err != nil {
		t.Fatalf("录音期间安全的非激活账号编辑不应被误拒：%v", err)
	}
}

func TestChannelCopyKeyIsScopedAndSettingsStayRedacted(t *testing.T) {
	a := testChannelRuntime(t, "http://127.0.0.1:1")
	for _, test := range []struct{ kind, id, expected string }{
		{"chat", "shared_a", "chat-secret-A"},
		{"chat", "chat_b", "chat-secret-B"},
		{"speech", "shared_a", "speech-secret-A"},
		{"speech", "speech_b", "speech-secret-B"},
	} {
		got, err := a.GetChannelKey(test.kind, test.id)
		if err != nil || got != test.expected {
			t.Fatalf("指定账号复制失败：类型=%s，账号=%s，错误=%v", test.kind, test.id, err)
		}
	}
	for _, test := range []struct{ kind, id string }{
		{"chat", "speech_b"}, {"speech", "chat_b"}, {"chat", "missing"}, {"other", "shared_a"},
	} {
		if key, err := a.GetChannelKey(test.kind, test.id); err == nil || key != "" {
			t.Fatal("不存在或跨池账号不应返回密钥")
		}
	}
	s := a.GetSettings()
	endpoints := []model.Endpoint{s.Chat, s.Speech}
	for _, pool := range [][]model.APIChannel{s.ChatChannels, s.SpeechChannels} {
		for _, channel := range pool {
			endpoints = append(endpoints, channel.Endpoint)
		}
	}
	for _, endpoint := range endpoints {
		if endpoint.APIKey != "" || !endpoint.HasKey {
			t.Fatal("复制单条密钥后，普通设置仍必须隐藏完整账号池的密钥")
		}
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-") {
		t.Fatal("前端设置 JSON 暴露了已存密钥")
	}
	// 前端副本的后续修改不能反向修改仓储中其他账号。
	s.ChatChannels[1].APIKey = "mutated-copy"
	if key, err := a.GetChannelKey("chat", "chat_b"); err != nil || key != "chat-secret-B" {
		t.Fatal("普通设置副本与内部账号池共享了可变数据")
	}
}

package service

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cue-go-remix/internal/entitlement"
	"cue-go-remix/internal/model"
)

func TestLockedLicenseRejectsChannelProbe(t *testing.T) {
	a, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Shutdown)
	ep := model.Endpoint{Provider: "custom", BaseURL: "http://127.0.0.1:1/v1", Model: "fixture", ChatProtocol: model.ChatProtocolResponses}
	if _, err := a.CheckEndpoint("chat", ep); !errors.Is(err, entitlement.ErrLocked) {
		t.Fatalf("未授权不应执行旧入口模型检测：%v", err)
	}
	if _, err := a.CheckChannelEndpoint("chat", model.APIChannel{ID: "fixture", Endpoint: ep}); !errors.Is(err, entitlement.ErrLocked) {
		t.Fatalf("未授权不应执行账号模型检测：%v", err)
	}
}

func TestLicenseRevocationCancelsResponsesProbeAndAnswer(t *testing.T) {
	entered := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path != "/v1/responses" {
			t.Errorf("协议路径错误：%s", r.URL.Path)
		}
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	a := testChannelRuntime(t, server.URL+"/v1")
	s := a.GetSettings()
	s.ChatChannels[0].ChatProtocol = model.ChatProtocolResponses
	s.ChatChannels[0].Stream = true
	if _, err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	probeDone := make(chan struct{})
	go func() {
		defer close(probeDone)
		_, _ = a.CheckChannelEndpoint("chat", a.GetSettings().ChatChannels[0])
	}()
	if err := a.Ask(model.AskRequest{Mode: "answerThis", Text: "fixture"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("Responses 请求未开始")
		}
	}
	if _, err := a.ForgetLicense(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-probeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("授权撤销后检测请求未取消")
	}
	awaitTask(t, a)
	if a.GetStatus().Busy {
		t.Fatal("授权撤销后问答仍处于忙碌状态")
	}
}

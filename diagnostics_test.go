package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsNeverRecordsSettingsSecretsOrArbitraryUIValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CUE_DATA_DIR", dir)
	a, err := newApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.shutdown)
	settings := a.GetSettings()
	settings.Chat.Provider = "custom"
	settings.Chat.APIKey = "fake-private-api-key"
	settings.Chat.BaseURL = "https://private-relay.example/v1?token=private-query"
	if _, err := a.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	a.RecordUIEvent("provider-select", "custom")
	a.RecordUIEvent("key-focus", "chat")
	a.RecordUIEvent("provider-select", settings.Chat.APIKey)
	a.RecordUIEvent("settings-save", settings.Chat.BaseURL)
	a.RecordUIEvent("private-transcript", "用户原文")
	data, err := os.ReadFile(filepath.Join(dir, "logs", "cue.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{settings.Chat.APIKey, "private-relay", "private-query", "用户原文", "private-transcript"} {
		if strings.Contains(text, secret) {
			t.Fatal("诊断日志包含敏感内容")
		}
	}
	for _, event := range []string{"settings-save-done", "action=provider-select value=custom", "action=key-focus value=chat"} {
		if !strings.Contains(text, event) {
			t.Fatalf("缺少诊断事件 %s", event)
		}
	}
}

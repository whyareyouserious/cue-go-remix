package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsNeverRecordsSettingsSecretsOrArbitraryUIValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CUE_DATA_DIR", dir)
	a, err := New(Options{DataDir: dir, LicenseURL: "https://license.example.test/api"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Shutdown)
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
	for _, event := range []string{`event=license-config origin="https://license.example.test" valid=true`, "settings-save-done", "action=provider-select value=custom", "action=key-focus value=chat"} {
		if !strings.Contains(text, event) {
			t.Fatalf("缺少诊断事件 %s", event)
		}
	}

	// 注入带敏感内容的模拟错误和警告
	a.report(errors.New("dial tcp 10.0.0.1:443 with key sk-1234567890abcdef: 401 unauthorized"))
	a.warn("自定义警告：包含私人会议标题“机密项目A”与 token=secret")

	// 导出脱敏诊断
	diagJSON, err := a.ExportDiagnostics()
	if err != nil {
		t.Fatalf("ExportDiagnostics 失败: %v", err)
	}

	var exported DiagnosticsExport
	if err := json.Unmarshal([]byte(diagJSON), &exported); err != nil {
		t.Fatalf("导出的诊断非有效 JSON: %v", err)
	}

	if exported.AppVersion != "0.1.0" {
		t.Fatalf("版本号错误，应为 0.1.0，得到: %s", exported.AppVersion)
	}

	for _, secret := range []string{"fake-private-api-key", "sk-1234567890abcdef", "机密项目A", "secret", "private-relay"} {
		if strings.Contains(diagJSON, secret) {
			t.Fatalf("导出的脱敏诊断包含敏感信息: %s", secret)
		}
	}

	if len(exported.RecentErrors) != 2 {
		t.Fatalf("期望近期脱敏错误数量为 2，得到: %d", len(exported.RecentErrors))
	}
	if exported.RecentErrors[0].Category != "http-401" {
		t.Fatalf("第 1 条错误分类期望 http-401，得到: %s", exported.RecentErrors[0].Category)
	}
	if exported.RecentErrors[1].Category != "general-warning" {
		t.Fatalf("第 2 条错误分类期望 general-warning，得到: %s", exported.RecentErrors[1].Category)
	}

	// 尝试用恶意未知 kind 探测，必须在写入 trace 前被严格拦截拒绝
	badKind := "malicious_kind_secret_leak"
	res, checkErr := a.CheckEndpoint(badKind, settings.Chat)
	if checkErr == nil || res.Success {
		t.Fatal("未知 kind 探测应被拒绝")
	}

	// 再次导出脱敏诊断，验证恶意 kind 绝未被 trace 进入日志，也未被导出
	diagJSONAfter, err := a.ExportDiagnostics()
	if err != nil {
		t.Fatalf("ExportDiagnostics 失败: %v", err)
	}
	if strings.Contains(diagJSONAfter, badKind) {
		t.Fatalf("导出的脱敏诊断意外包含了未校验的恶意 kind: %s", badKind)
	}

	// 模拟写入带有 id 和未知字段的 answer-failed 正常日志，以及恶意伪时间/脏字段
	logFile, err := os.OpenFile(filepath.Join(dir, "logs", "cue.log"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = logFile.WriteString("pid=99999 2026/11/21 03:00:00.123456 event=answer-failed id=secret-req-id-12345 stage=provider cancelled=false kind=http-401 elapsed_ms=45\n")
	_, _ = logFile.WriteString("pid=99999 fake:pseudo:time event=unknown-malicious-event secret=leak reason=arbitrary_bad_string\n")
	_ = logFile.Close()

	diagJSONFinal, err := a.ExportDiagnostics()
	if err != nil {
		t.Fatal(err)
	}
	// 验证 answer-failed 保留，但 id=secret-req-id-12345 被清洗丢弃
	if !strings.Contains(diagJSONFinal, "event=answer-failed") || !strings.Contains(diagJSONFinal, "kind=http-401") {
		t.Fatalf("answer-failed 正常事件不应被整行丢弃: %s", diagJSONFinal)
	}
	if strings.Contains(diagJSONFinal, "secret-req-id-12345") || strings.Contains(diagJSONFinal, "fake:pseudo:time") || strings.Contains(diagJSONFinal, "unknown-malicious-event") || strings.Contains(diagJSONFinal, "arbitrary_bad_string") {
		t.Fatalf("脱敏诊断包含了未清洗的非法字段或伪时间: %s", diagJSONFinal)
	}

	// 清空近期错误测试
	a.ClearRecentErrors()
	if len(a.GetRecentErrors()) != 0 {
		t.Fatal("ClearRecentErrors 未清空近期错误列表")
	}
}

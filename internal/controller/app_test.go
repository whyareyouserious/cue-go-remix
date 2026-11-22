package controller

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
	"cue-go-remix/internal/service"
)

// 只替换文件对话框；导出与配置脱敏仍经过真实业务层。
type dialogDesktop struct {
	Desktop
	path, suggested string
	err             error
	opened          int
}

func (d *dialogDesktop) ExportMeetingPath(name string) (string, error) {
	d.opened++
	d.suggested = name
	return d.path, d.err
}

func (d *dialogDesktop) ExportDiagnosticPath(name string) (string, error) {
	d.opened++
	d.suggested = name
	return d.path, d.err
}

func (d *dialogDesktop) ImportModelPath() (string, error) { return d.path, d.err }

func (d *dialogDesktop) UpdateShortcuts(shortcuts map[string]string, persistFn func() error) error {
	if d.err != nil {
		return d.err
	}
	if persistFn != nil {
		return persistFn()
	}
	return nil
}

func testController(t *testing.T) (*App, *dialogDesktop) {
	t.Helper()
	dir := t.TempDir()
	meeting := model.Meeting{ID: "fixture", Title: "测试会议", StartedAt: time.Now().UnixMilli(), EndedAt: time.Now().UnixMilli(), Notes: "测试纪要", Turns: []model.Turn{{Channel: "them", Text: "导出转录", Time: time.Now().UnixMilli()}}}
	if err := repository.WriteJSON(filepath.Join(dir, "meetings.json"), []model.Meeting{meeting}); err != nil {
		t.Fatal(err)
	}
	runtime, err := service.New(service.Options{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	// 不启动 Runtime，避免开启本地助手服务。
	t.Cleanup(runtime.Shutdown)
	desktop := &dialogDesktop{}
	return New(runtime, desktop), desktop
}

func TestExportMeetingAcrossLayers(t *testing.T) {
	app, dialog := testController(t)
	if _, err := app.ExportMeeting("missing"); err == nil || dialog.opened != 0 {
		t.Fatal("不存在的会议不应打开文件对话框")
	}
	if path, err := app.ExportMeeting("fixture"); err != nil || path != "" {
		t.Fatal("取消导出应返回空路径且不报错", path, err)
	}
	dialog.err = errors.New("模拟文件对话框失败")
	if _, err := app.ExportMeeting("fixture"); !errors.Is(err, dialog.err) {
		t.Fatal("文件对话框错误丢失", err)
	}
	dialog.err = nil
	dialog.path = filepath.Join(t.TempDir(), "meeting.md")
	path, err := app.ExportMeeting("fixture")
	if err != nil || path != dialog.path || dialog.suggested != "cue-fixture.md" {
		t.Fatal("导出路径或默认文件名错误", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"# 测试会议", "测试纪要", "## 转录", "导出转录"} {
		if !strings.Contains(string(data), part) {
			t.Fatal("导出内容缺少", part)
		}
	}
}

func TestImportCancellationAndSettingsRedaction(t *testing.T) {
	app, dialog := testController(t)
	if err := app.ImportModel("unknown"); err != nil {
		t.Fatal("取消文件选择不应开始导入", err)
	}
	dialog.err = errors.New("模拟文件对话框失败")
	if err := app.ImportModel("unknown"); !errors.Is(err, dialog.err) {
		t.Fatal("文件对话框错误丢失", err)
	}
	settings := app.GetSettings()
	settings.Chat.APIKey = "fixture-secret"
	if _, err := app.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if got := app.GetSettings(); got.Chat.APIKey != "" || !got.Chat.HasKey {
		t.Fatal("绑定入口返回了密钥或丢失已保存状态")
	}
}

func TestFrontendBindingMatchesController(t *testing.T) {
	typeOfApp := reflect.TypeOf(App{})
	bridge, err := os.ReadFile("../../frontend/src/api/wails.ts")
	if err != nil {
		t.Fatal(err)
	}
	// Wails 用 Go 包完整路径组成调用名，移动 App 后不能沿用 main.App。
	prefix := typeOfApp.PkgPath() + "." + typeOfApp.Name() + ".${method}"
	if !strings.Contains(string(bridge), "`"+prefix+"`") {
		t.Fatal("前端 Wails 调用名与 Go 绑定类型不一致")
	}
	for _, internal := range []string{"Start", "Shutdown", "Settings", "SavePosition", "Trace", "Emit"} {
		if _, exposed := reflect.PointerTo(typeOfApp).MethodByName(internal); exposed {
			t.Fatal("内部生命周期或原始配置不应暴露给前端", internal)
		}
	}
}

func TestAskWithoutActivationAcrossLayers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("模型请求路径错误：%s", r.URL.Path)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"无需卡密即可回答"}}]}`)
	}))
	defer server.Close()
	completed := make(chan model.Event, 1)
	runtime, err := service.New(service.Options{DataDir: t.TempDir(), Emit: func(event model.Event) {
		if event.Type == "answer-done" || event.Type == "answer-error" {
			completed <- event
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Shutdown)
	app := New(runtime, &dialogDesktop{})
	settings := app.GetSettings()
	settings.Chat = model.Endpoint{Provider: "custom", BaseURL: server.URL + "/v1", Model: "local-test"}
	if _, err := app.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := app.Ask(model.AskRequest{Mode: "answerThis", Text: "验证无卡密问答"}); err != nil {
		t.Fatal("配置模型后应可直接提问：", err)
	}
	select {
	case event := <-completed:
		if event.Type != "answer-done" || event.Text != "无需卡密即可回答" {
			t.Fatalf("跨层问答未成功：%+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("跨层问答没有完成")
	}
}

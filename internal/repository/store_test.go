package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cue-go-remix/internal/model"
)

func TestSecretRoundTripAndHostChange(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	v := s.Settings()
	v.Chat.APIKey = "private-secret"
	v.Speech.APIKey = "speech-secret"
	red, e := s.SaveSettings(v)
	if e != nil {
		t.Fatal(e)
	}
	if red.Chat.APIKey != "" || !red.Chat.HasKey {
		t.Fatal("密钥不应返回前端")
	}
	red.Smart = true
	if _, e = s.SaveSettings(red); e != nil {
		t.Fatal(e)
	}
	if s.Settings().Chat.APIKey != "private-secret" {
		t.Fatal("同服务空字段应保留密钥")
	}
	red.Chat.BaseURL = "https://relay.example/v1"
	// 该用例验证旧单端点调用；新客户端通过 ChatChannels 编辑渠道。
	red.ChatChannels = nil
	if _, e = s.SaveSettings(red); e != nil {
		t.Fatal(e)
	}
	if s.Settings().Chat.APIKey != "" {
		t.Fatal("切换主机不得复用密钥")
	}
	if s.Settings().Speech.APIKey != "speech-secret" {
		t.Fatal("语音密钥应独立保存")
	}
}
func TestRecoveryAndMeetingResume(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	v := s.Settings()
	v.AIRules = "第一版"
	if _, e = s.SaveSettings(v); e != nil {
		t.Fatal(e)
	}
	v.AIRules = "第二版"
	if _, e = s.SaveSettings(v); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "settings.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	recovered, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if recovered.Settings().AIRules != "第一版" {
		t.Fatal("应从有效备份恢复")
	}
	if _, e = recovered.Append("them", "我们明天九点再讨论"); e != nil {
		t.Fatal(e)
	}
	again, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	m := again.Current()
	if m == nil || len(m.Turns) != 1 {
		t.Fatal("应恢复未结束会议")
	}
	if e = again.Delete(m.ID); e == nil {
		t.Fatal("不可删除进行中的会议")
	}
	closed, e := again.End()
	if e != nil || closed.EndedAt == 0 {
		t.Fatal("结束会议未保存", e)
	}
	if e = again.Delete(m.ID); e != nil {
		t.Fatal(e)
	}
}
func TestStaleMeetingAndRetention(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	_, _ = s.Append("you", "旧会议")
	s.meetings[0].Turns[0].Time = time.Now().Add(-31 * time.Minute).UnixMilli()
	old := s.current
	_, _ = s.Append("them", "新会议")
	if s.current == old || s.meetings[0].EndedAt == 0 {
		t.Fatal("超时会话应分开")
	}
	for i := 0; i < 55; i++ {
		_, _ = s.End()
		if _, e = s.Append("you", "下一场"); e != nil {
			t.Fatal(e)
		}
	}
	if len(s.Meetings()) != 50 {
		t.Fatal("保留数量错误")
	}
}

func TestSettingsDeepCopy(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	cfg := s.Settings()
	if len(cfg.Scenarios) == 0 {
		t.Fatal("默认场景不应为空")
	}
	// 修改返回的深拷贝切片
	cfg.Scenarios[0].Name = "被修改的名字"
	cfg.Scenarios[0].Profile.Projects = append(cfg.Scenarios[0].Profile.Projects, model.ProjectExperience{Name: "注入项目"})

	// 重新读取 store，确保内部未被污染
	original := s.Settings()
	if original.Scenarios[0].Name == "被修改的名字" {
		t.Fatal("Settings() 未能实现 Scenarios 切片深拷贝")
	}
	if len(original.Scenarios[0].Profile.Projects) != 0 {
		t.Fatal("Settings() 未能实现 Projects 切片深拷贝")
	}
}

func TestSaveSettingsPreservesShortcuts(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// 先通过专门的 SaveShortcuts 路径更新自定义快捷键
	customShortcuts := map[string]string{
		"say":         "CmdOrCtrl+J",
		"assist":      "CmdOrCtrl+K",
		"leetcode":    "CmdOrCtrl+L",
		"toggle":      "CmdOrCtrl+;",
		"quit":        "CmdOrCtrl+Q",
		"passthrough": "CmdOrCtrl+P",
	}
	if _, err := s.SaveShortcuts(customShortcuts); err != nil {
		t.Fatal(err)
	}

	// 模拟普通 SaveSettings 提交了旧的/空白的 draft shortcuts
	draft := s.Settings()
	draft.Shortcuts = map[string]string{
		"say": "CmdOrCtrl+OldDraft",
	}
	saved, err := s.SaveSettings(draft)
	if err != nil {
		t.Fatal(err)
	}

	// 验证已生效的 shortcuts 没有被旧 draft 覆盖
	if saved.Shortcuts["say"] != "CmdOrCtrl+J" {
		t.Fatalf("普通 SaveSettings 不应覆盖专门路径生效的快捷键，得到: %s", saved.Shortcuts["say"])
	}
	if s.Settings().Shortcuts["say"] != "CmdOrCtrl+J" {
		t.Fatalf("存储层内部快捷键被意外覆盖，得到: %s", s.Settings().Shortcuts["say"])
	}
}

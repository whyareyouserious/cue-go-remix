package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

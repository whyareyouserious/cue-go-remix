package assistant

import (
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/store"
	"strings"
	"testing"
)

func TestCodingAndSelectedQuestionIsolation(t *testing.T) {
	s := store.DefaultSettings()
	s.Background = "个人秘密"
	s.AIRules = "回答规则"
	turns := []model.Turn{{Channel: "them", Text: "会议秘密"}}
	r, screen, e := Build(s, model.AskRequest{Mode: "leetcode"}, turns, nil, "")
	if e != nil || !screen {
		t.Fatal(e)
	}
	if strings.Contains(r.System, "个人秘密") || strings.Contains(r.System, "回答规则") || strings.Contains(r.Text, "会议秘密") {
		t.Fatal("编码模式混入会议/个人上下文")
	}
	r, screen, e = Build(s, model.AskRequest{Mode: "answerThis", Text: "当前问题"}, turns, nil, "")
	if e != nil || screen || strings.Contains(r.Text, "会议秘密") || !strings.Contains(r.Text, "当前问题") {
		t.Fatal("指定问题模式上下文错误", e)
	}
}
func TestEmptyRecapAndRecentLimit(t *testing.T) {
	s := store.DefaultSettings()
	if _, _, e := Build(s, model.AskRequest{Mode: "recap"}, nil, nil, ""); e == nil {
		t.Fatal("空会议不能生成回顾")
	}
	turns := []model.Turn{{Channel: "them", Text: "不应看到这一轮"}}
	for i := 0; i < 16; i++ {
		turns = append(turns, model.Turn{Channel: "you", Text: "近期内容"})
	}
	r, _, e := Build(s, model.AskRequest{Mode: "say"}, turns, nil, "")
	if e != nil || strings.Contains(r.Text, "不应看到这一轮") {
		t.Fatal("近期轮数限制错误", e)
	}
}

package assistant

import (
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
	"strings"
	"testing"
)

func TestCodingAndSelectedQuestionIsolation(t *testing.T) {
	s := repository.DefaultSettings()
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
	s := repository.DefaultSettings()
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

func TestScenarioProfileAndPreferencesPromptInjection(t *testing.T) {
	s := repository.DefaultSettings()
	s.Scenarios = []model.Scenario{
		{
			ID:   "interview",
			Name: "面试场景",
			Profile: model.Profile{
				Role:      "资深架构师",
				Industry:  "云计算",
				Years:     "8年",
				Skills:    "Go, K8s, 微服务",
				Strengths: "分布式系统高可用",
				Goals:     "拿到大厂技术专家Offer",
				Projects: []model.ProjectExperience{
					{Name: "核心网关重构", Role: "主导架构", Achievements: "QPS提升300%"},
				},
			},
			Preferences: model.Preferences{
				Length:    "concise",
				Tone:      "direct",
				Structure: "conclusion-first",
				Examples:  "always",
				Custom:    "优先给出架构图构思",
			},
		},
		{
			ID:   "report",
			Name: "工作汇报",
			Profile: model.Profile{
				Role:     "研发经理",
				Industry: "企业SaaS",
			},
			Preferences: model.Preferences{
				Length:    "detailed",
				Tone:      "professional",
				Structure: "bullet-points",
			},
		},
	}
	s.ActiveScenarioID = "interview"
	turns := []model.Turn{{Channel: "them", Text: "请介绍一下你最近的项目"}}

	// 1. 约定模式注入面试场景
	r, _, err := Build(s, model.AskRequest{Mode: "say"}, turns, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.System, "资深架构师") || !strings.Contains(r.System, "云计算") || !strings.Contains(r.System, "核心网关重构") {
		t.Fatalf("未能正确注入面试场景个人定位: %s", r.System)
	}
	if !strings.Contains(r.System, "极简扼要") || !strings.Contains(r.System, "结论先行") || !strings.Contains(r.System, "优先给出架构图构思") {
		t.Fatalf("未能正确注入面试场景回答偏好: %s", r.System)
	}
	if !strings.Contains(r.System, "【优先级规范】当前场景的个人定位背景与回答偏好优先级高于任何通用背景或全局规则") {
		t.Fatalf("未能向模型提示词注入场景优先级规范指令: %s", r.System)
	}

	// 2. 切换场景注入工作汇报
	s.ActiveScenarioID = "report"
	r, _, err = Build(s, model.AskRequest{Mode: "say"}, turns, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.System, "研发经理") || !strings.Contains(r.System, "条理分点") {
		t.Fatalf("未能正确切换场景注入: %s", r.System)
	}
	if strings.Contains(r.System, "资深架构师") {
		t.Fatal("不应包含旧场景个人定位")
	}

	// 3. leetcode 模式下严禁注入任何场景背景与偏好
	rLeetcode, _, err := Build(s, model.AskRequest{Mode: "leetcode"}, turns, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rLeetcode.System, "研发经理") || strings.Contains(rLeetcode.System, "条理分点") {
		t.Fatal("leetcode 模式错误混入了场景数据！")
	}
}

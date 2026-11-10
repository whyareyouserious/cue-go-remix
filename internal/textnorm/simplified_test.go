package textnorm

import "testing"

func TestSimplifiedTranscripts(t *testing.T) {
	tests := []struct{ input, want string }{
		{"這個轉換好像有點慢，謝謝大家", "这个转换好像有点慢，谢谢大家"},
		{"乾隆年間天氣乾燥，頭髮容易乾", "乾隆年间天气干燥，头发容易干"},
		{"請回覆，反反覆覆確認覆盆子", "请回复，反反复复确认覆盆子"},
		{"我瞭解了，從瞭望臺看得一目瞭然", "我了解了，从瞭望台看得一目了然"},
		{"著名作品的著作權", "著名作品的著作权"},
		{"Cue API 2.0：網絡延遲 123ms\n🙂", "Cue API 2.0：网络延迟 123ms\n🙂"},
		{"軟體、滑鼠", "软体、滑鼠"}, // 字形转换不擅自替换地区用词。
		{"可以深刻意", "可以深刻意"}, // 识别错词不能靠简繁转换猜测修正。
		{"中文测试没有繁体字。", "中文测试没有繁体字。"},
		{"", ""},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			got := Simplified(test.input)
			if got != test.want {
				t.Fatalf("转换结果 %q，期望 %q", got, test.want)
			}
			if again := Simplified(got); again != got {
				t.Fatalf("重复归一化改变了文本：%q → %q", got, again)
			}
		})
	}
}

func TestInterimRevisionUsesCompletePhrase(t *testing.T) {
	// 实时字幕每次重新归一化完整暂定句；不能将单字转换结果不可逆地累加。
	if got := Simplified("乾"); got != "干" {
		t.Fatalf("单字首选转换错误：%q", got)
	}
	if got := Simplified("乾隆"); got != "乾隆" {
		t.Fatalf("新上下文没有保留人名：%q", got)
	}
}

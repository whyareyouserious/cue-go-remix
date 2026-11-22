package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cue-go-remix/internal/model"
)

// channelFixture 使用虚构地址与凭据建立渠道，不发送任何网络请求。
func channelFixture(id, key string) model.APIChannel {
	return model.APIChannel{ID: id, Name: "测试渠道 " + id, Endpoint: model.Endpoint{Provider: "custom", BaseURL: "https://channel.example/v1", APIKey: key, Model: "test-model", Stream: false}, Status: "untested"}
}

func channelStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestChannelProtocolPersistsWithoutChangingCredentials(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{channelFixture("protocol_account", "fixture-protocol-secret")}
	v.ActiveChatChannelID = "protocol_account"
	red, err := s.SaveSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	if red.Chat.ChatProtocol != "" {
		t.Fatal("旧渠道必须保留原协议默认值")
	}
	red.ChatChannels[0].ChatProtocol = model.ChatProtocolResponses
	if _, err = s.SaveSettings(red); err != nil {
		t.Fatal(err)
	}
	again, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	got := again.Settings()
	if got.Chat.ChatProtocol != model.ChatProtocolResponses || got.ChatChannels[0].ChatProtocol != model.ChatProtocolResponses || got.Chat.APIKey != "fixture-protocol-secret" {
		t.Fatal("协议未持久化，或仅切换协议错误地丢失凭据")
	}
	for _, protocol := range []string{"unknown", "responses\nprivate-data"} {
		bad := Redact(got)
		bad.ChatChannels[0].ChatProtocol = protocol
		if _, err := again.SaveSettings(bad); err == nil || !reflect.DeepEqual(got, again.Settings()) {
			t.Fatal("无效协议必须拒绝且不改变配置")
		}
	}
	bad := Redact(got)
	bad.ChatChannels[0].Provider = "anthropic"
	if _, err := again.SaveSettings(bad); err == nil {
		t.Fatal("原生协议不应被 Responses 兼容字段覆盖")
	}
	bad = Redact(got)
	bad.SpeechChannels = []model.APIChannel{channelFixture("speech_protocol", "fixture-speech-secret")}
	bad.SpeechChannels[0].ChatProtocol = model.ChatProtocolResponses
	bad.ActiveSpeechChannelID = "speech_protocol"
	if _, err := again.SaveSettings(bad); err == nil {
		t.Fatal("语音渠道不能使用聊天协议字段")
	}
}

func TestLegacyChannelMigrationKeepsSelectedServiceAndSecrets(t *testing.T) {
	dir := t.TempDir()
	legacy := DefaultSettings()
	legacy.Chat.APIKey = "migration-chat-key"
	legacy.Speech.APIKey = "migration-speech-key"
	if err := WriteJSON(filepath.Join(dir, "settings.json"), legacy); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if len(got.ChatChannels) != 1 || len(got.SpeechChannels) != 1 || got.ActiveChatChannelID != got.ChatChannels[0].ID || got.ActiveSpeechChannelID != got.SpeechChannels[0].ID {
		t.Fatal("旧配置未生成并激活对应的独立渠道")
	}
	if got.ChatChannels[0].APIKey != legacy.Chat.APIKey || got.SpeechChannels[0].APIKey != legacy.Speech.APIKey || got.Chat != legacy.Chat || got.Speech != legacy.Speech {
		t.Fatal("迁移改变了既有提供商、模型或密钥")
	}
	if _, err := s.SaveSettings(Redact(got)); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	reopened := again.Settings()
	if reopened.ActiveChatChannelID != got.ActiveChatChannelID || reopened.Chat.APIKey != legacy.Chat.APIKey || reopened.Speech.APIKey != legacy.Speech.APIKey {
		t.Fatal("保存重开后迁移的身份或凭据丢失")
	}
}

func TestChannelSecretsAreScopedByKindIDAndEndpoint(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{channelFixture("account_a", "chat-key-a"), channelFixture("account_b", "chat-key-b")}
	v.SpeechChannels = []model.APIChannel{channelFixture("account_a", "speech-key-a")}
	v.ActiveChatChannelID, v.ActiveSpeechChannelID = "account_a", "account_a"
	red, err := s.SaveSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	red.ActiveChatChannelID = "account_b"
	if _, err := s.SaveSettings(red); err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if got.Chat.APIKey != "chat-key-b" || got.ChatChannels[0].APIKey != "chat-key-a" || got.Speech.APIKey != "speech-key-a" {
		t.Fatal("同域不同账号或不同类型渠道发生密钥串用")
	}
	red = Redact(got)
	red.ChatChannels = append(red.ChatChannels, channelFixture("unknown_account", ""))
	red.ActiveChatChannelID = "unknown_account"
	if _, err := s.SaveSettings(red); err != nil {
		t.Fatal(err)
	}
	if s.Settings().Chat.APIKey != "" {
		t.Fatal("未知渠道 ID 不得继承同域账号的密钥")
	}
	red = Redact(s.Settings())
	red.ChatChannels[0].BaseURL = "https://another-channel.example/v1"
	red.ChatChannels[1].ClearKey = true
	if _, err := s.SaveSettings(red); err != nil {
		t.Fatal(err)
	}
	got = s.Settings()
	if got.ChatChannels[0].APIKey != "" || got.ChatChannels[1].APIKey != "" || got.Speech.APIKey != "speech-key-a" {
		t.Fatal("更换地址或显式清除凭据未生效，或误清另一类渠道")
	}
}

func TestChannelRedactionAndClonePreserveIsolation(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{channelFixture("chat", "private-chat-marker")}
	v.SpeechChannels = []model.APIChannel{channelFixture("speech", "private-speech-marker")}
	v.ActiveChatChannelID, v.ActiveSpeechChannelID = "chat", "speech"
	red, err := s.SaveSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(red)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-chat-marker") || strings.Contains(string(data), "private-speech-marker") || !red.ChatChannels[0].HasKey || !red.SpeechChannels[0].HasKey {
		t.Fatal("返回设置遗漏渠道脱敏或密钥存在标记")
	}
	v.ChatChannels[0].Name = "修改调用方输入"
	red.ChatChannels[0].Name = "修改脱敏副本"
	copy := s.Settings()
	copy.SpeechChannels[0].APIKey = "修改完整副本"
	if s.Settings().ChatChannels[0].Name != "测试渠道 chat" || s.Settings().SpeechChannels[0].APIKey != "private-speech-marker" {
		t.Fatal("渠道切片没有深拷贝，外部修改污染了存储")
	}
	var flat map[string]any
	encoded, _ := json.Marshal(red.ChatChannels[0])
	if err := json.Unmarshal(encoded, &flat); err != nil {
		t.Fatal(err)
	}
	if flat["provider"] != "custom" || flat["Endpoint"] != nil {
		t.Fatal("APIChannel 的端点 JSON 字段必须保持扁平")
	}
}

func TestDeletingLastChannelDoesNotReviveLegacyEndpoint(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{channelFixture("only", "last-key")}
	v.ActiveChatChannelID = "only"
	red, err := s.SaveSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	red.ChatChannels = []model.APIChannel{}
	red.ActiveChatChannelID = ""
	if _, err := s.SaveSettings(red); err != nil {
		t.Fatal(err)
	}
	if s.Settings().Chat != (model.Endpoint{}) || s.Settings().ChatChannels == nil {
		t.Fatal("删除最后渠道后旧端点仍生效，或空列表被转回迁移状态")
	}
	again, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if again.Settings().Chat != (model.Endpoint{}) || len(again.Settings().ChatChannels) != 0 {
		t.Fatal("重启后删除的渠道被旧端点复活")
	}
}

func TestPrepareSettingsResolvesSpeechWithoutMutatingState(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.SpeechChannels = []model.APIChannel{channelFixture("speech_a", "speech-a"), channelFixture("speech_b", "speech-b")}
	v.ActiveSpeechChannelID = "speech_a"
	if _, err := s.SaveSettings(v); err != nil {
		t.Fatal(err)
	}
	before := s.Settings()
	path := filepath.Join(s.Dir(), "settings.json")
	beforeFile, _ := os.ReadFile(path)
	draft := Redact(before)
	draft.ActiveSpeechChannelID = "speech_b"
	prepared, err := s.PrepareSettings(draft)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Speech.APIKey != "speech-b" || prepared.Speech == before.Speech {
		t.Fatal("准备配置没有解析新的语音渠道，录音保护会比较错误的端点")
	}
	afterFile, _ := os.ReadFile(path)
	if !reflect.DeepEqual(before, s.Settings()) || string(beforeFile) != string(afterFile) {
		t.Fatal("只读准备配置不应改变内存或持久配置")
	}
	for _, id := range []string{"local_sherpa", "local_whisper"} {
		draft.ActiveSpeechChannelID = id
		draft.WhisperModel, draft.WhisperPath, draft.WhisperThreads = "small", "/tmp/example-whisper", 3
		prepared, err = s.PrepareSettings(draft)
		if err != nil {
			t.Fatal(err)
		}
		if prepared.Speech.APIKey != "" || prepared.Speech.BaseURL != "" || prepared.WhisperModel != "small" || prepared.WhisperPath != draft.WhisperPath || prepared.WhisperThreads != 3 || len(prepared.SpeechChannels) != 2 {
			t.Fatal("本地引擎污染 API 池、保留云端密钥或改变 Whisper 参数")
		}
	}
}

func TestInvalidChannelsAndWriteFailureLeaveSettingsIntact(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{channelFixture("valid", "valid-key")}
	v.SpeechChannels = []model.APIChannel{}
	v.ActiveChatChannelID = "valid"
	if _, err := s.SaveSettings(v); err != nil {
		t.Fatal(err)
	}
	before := s.Settings()
	path := filepath.Join(s.Dir(), "settings.json")
	beforeFile, _ := os.ReadFile(path)
	cases := map[string]func(*model.Settings){
		"重复ID":    func(v *model.Settings) { v.ChatChannels = append(v.ChatChannels, v.ChatChannels[0]) },
		"非法ID":    func(v *model.Settings) { v.ChatChannels[0].ID = "<invalid>" },
		"保留ID":    func(v *model.Settings) { v.ChatChannels[0].ID = "local_sherpa" },
		"未知激活ID":  func(v *model.Settings) { v.ActiveChatChannelID = "missing" },
		"缺少名称":    func(v *model.Settings) { v.ChatChannels[0].Name = " " },
		"缺少模型":    func(v *model.Settings) { v.ChatChannels[0].Model = " " },
		"非法URL":   func(v *model.Settings) { v.ChatChannels[0].BaseURL = "file:///tmp/provider" },
		"URL账号密码": func(v *model.Settings) { v.ChatChannels[0].BaseURL = "https://user:pass@example.test/v1" },
		"错误协议类型":  func(v *model.Settings) { v.ChatChannels[0].Provider = "sherpa" },
		"语音实时协议": func(v *model.Settings) {
			c := channelFixture("speech", "")
			c.Stream = true
			v.SpeechChannels = []model.APIChannel{c}
			v.ActiveSpeechChannelID = c.ID
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			draft := Redact(before)
			change(&draft)
			if _, err := s.SaveSettings(draft); err == nil {
				t.Fatal("无效渠道配置被保存")
			}
			current, _ := os.ReadFile(path)
			if !reflect.DeepEqual(before, s.Settings()) || string(beforeFile) != string(current) {
				t.Fatal("校验失败改变了已生效配置")
			}
		})
	}
	blockFile(t, path)
	draft := Redact(before)
	draft.ChatChannels[0].Name = "无法落盘的修改"
	if _, err := s.SaveSettings(draft); err == nil || !reflect.DeepEqual(before, s.Settings()) {
		t.Fatal("写盘失败未保持原配置")
	}
}

func TestLegacyNilPoolUpdatesOnlyActiveChannel(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{channelFixture("first", "first-key"), channelFixture("second", "second-key")}
	v.ActiveChatChannelID = "first"
	if _, err := s.SaveSettings(v); err != nil {
		t.Fatal(err)
	}
	legacy := Redact(s.Settings())
	legacy.ChatChannels = nil
	legacy.Chat.BaseURL = "https://changed.example/v1"
	if _, err := s.SaveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if got.Chat.APIKey != "" || got.ChatChannels[1].APIKey != "second-key" || len(got.ChatChannels) != 2 {
		t.Fatal("旧单端点保存没有隔离地更新原激活渠道")
	}
}

func TestLegacyOfficialDefaultURLsRemainValidAfterMigration(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic", "gemini", "cerebras", "deepseek", "groq", "minimax", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			dir := t.TempDir()
			legacy := DefaultSettings()
			legacy.Chat = model.Endpoint{Provider: provider, Model: "existing-model", APIKey: "existing-default-url-key"}
			if err := WriteJSON(filepath.Join(dir, "settings.json"), legacy); err != nil {
				t.Fatal(err)
			}
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.SaveSettings(Redact(s.Settings())); err != nil {
				t.Fatalf("旧官方默认地址迁移后无法保存：%v", err)
			}
			if got := s.Settings().Chat; got.BaseURL != "" || got.APIKey != legacy.Chat.APIKey {
				t.Fatal("保存改变了默认地址语义或丢失密钥")
			}
		})
	}
}

func TestChannelKeyPreviewsFollowCredentialsWithoutBeingStored(t *testing.T) {
	s := channelStore(t)
	v := s.Settings()
	v.ChatChannels = []model.APIChannel{
		channelFixture("first", "aaaa-private-first-1111"),
		channelFixture("second", "bbbb-private-second-2222"),
		channelFixture("short", "short123"),
		channelFixture("empty", ""),
	}
	v.SpeechChannels = []model.APIChannel{channelFixture("speech", "cccc-private-speech-3333")}
	v.ActiveChatChannelID, v.ActiveSpeechChannelID = "first", "speech"
	v.ChatChannels[0].MaskedAPIKey = "伪造的预览"
	red, err := s.SaveSettings(v)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"aaaa*****1111", "bbbb*****2222", "*****", ""} {
		if red.ChatChannels[i].MaskedAPIKey != want || red.ChatChannels[i].APIKey != "" {
			t.Fatalf("第 %d 个聊天账号预览不正确或完整密钥未清除", i+1)
		}
	}
	if red.SpeechChannels[0].MaskedAPIKey != "cccc*****3333" || red.SpeechChannels[0].APIKey != "" {
		t.Fatal("语音账号预览未使用自身凭据")
	}
	data, _ := json.Marshal(red)
	for _, secret := range []string{"aaaa-private-first-1111", "bbbb-private-second-2222", "short123", "cccc-private-speech-3333"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("脱敏设置返回了完整密钥")
		}
	}
	emptyJSON, _ := json.Marshal(red.ChatChannels[3])
	if strings.Contains(string(emptyJSON), "maskedApiKey") {
		t.Fatal("空凭据不应返回密钥预览字段")
	}

	// 前端原样回传预览或伪造预览，均不能影响后台保存的真实凭据。
	red.ChatChannels[1].MaskedAPIKey = "客户端伪造预览"
	red, err = s.SaveSettings(red)
	if err != nil {
		t.Fatal(err)
	}
	if s.Settings().ChatChannels[1].APIKey != "bbbb-private-second-2222" || red.ChatChannels[1].MaskedAPIKey != "bbbb*****2222" {
		t.Fatal("回传预览污染了真实密钥或响应预览")
	}
	red.ChatChannels[0].APIKey = "new1-private-replacement-9999"
	red.ChatChannels[1].ClearKey = true
	red, err = s.SaveSettings(red)
	if err != nil {
		t.Fatal(err)
	}
	if red.ChatChannels[0].MaskedAPIKey != "new1*****9999" || red.ChatChannels[1].MaskedAPIKey != "" || red.ChatChannels[1].HasKey {
		t.Fatal("替换或清除密钥后预览未刷新")
	}

	// nil 池旧入口修改有效端点时，同样生成新预览，其他账号预览保持各自身份。
	red.ChatChannels = nil
	red.Chat.APIKey = "old1-private-legacy-8888"
	red, err = s.SaveSettings(red)
	if err != nil {
		t.Fatal(err)
	}
	if red.ChatChannels[0].MaskedAPIKey != "old1*****8888" || red.ChatChannels[2].MaskedAPIKey != "*****" {
		t.Fatal("旧接口保存没有返回真实凭据对应的预览")
	}
	for _, channel := range append(s.Settings().ChatChannels, s.Settings().SpeechChannels...) {
		if channel.MaskedAPIKey != "" {
			t.Fatal("内部完整配置不应保留响应预览")
		}
	}
	for _, name := range []string{"settings.json", "settings.json.bak"} {
		stored, err := os.ReadFile(filepath.Join(s.Dir(), name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(stored), "maskedApiKey") || strings.Contains(string(stored), "*****") {
			t.Fatal("密钥预览不应进入配置或备份文件")
		}
	}
}

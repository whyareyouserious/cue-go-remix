package whisper

import (
	"context"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"

	"cue-go-remix/internal/store"
)

// Opt-in real inference, using a downloaded model and a public speech fixture.
func TestRealWhisperRuntime(t *testing.T) {
	exe, model, wav := os.Getenv("CUE_TEST_WHISPER_RUNTIME"), os.Getenv("CUE_TEST_WHISPER_MODEL"), os.Getenv("CUE_TEST_WHISPER_WAV")
	if exe == "" || model == "" || wav == "" {
		t.Skip("未配置本地 Whisper 集成测试资源")
	}
	b, err := os.ReadFile(wav)
	if err != nil {
		t.Fatal(err)
	}
	var pcm []int16
	for at := 12; at+8 <= len(b); {
		n := int(binary.LittleEndian.Uint32(b[at+4 : at+8]))
		end := at + 8 + n
		if end > len(b) {
			t.Fatal("无效 WAV")
		}
		if string(b[at:at+4]) == "data" {
			for i := at + 8; i+2 <= end; i += 2 {
				pcm = append(pcm, int16(binary.LittleEndian.Uint16(b[i:i+2])))
			}
			break
		}
		at = end + n%2
	}
	if len(pcm) == 0 {
		t.Fatal("WAV 中没有音频")
	}
	settings := store.DefaultSettings()
	settings.WhisperPath = exe
	settings.Language = "en"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	session, err := Start(ctx, settings, model)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	text, err := session.Transcribe(ctx, pcm)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
	if !strings.Contains(strings.ToLower(text), "country") {
		t.Fatalf("转写结果未识别已知语句：%s", text)
	}
}

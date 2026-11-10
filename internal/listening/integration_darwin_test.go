//go:build darwin

package listening

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cue-go-remix/internal/model"
)

// Exercise the production helper framing, channel routing, listening session,
// and shipped native recognizer with a public fixture, without a microphone.
func TestRealCaptureProtocolToStreamingText(t *testing.T) {
	fixture := os.Getenv("CUE_TEST_SHERPA_WAV")
	if fixture == "" {
		t.Skip("未配置公开中文样本")
	}
	fixture, _ = filepath.Abs(fixture)
	t.Chdir("../..")
	helper := filepath.Join(t.TempDir(), "cue-fixture-audio")
	script := `#!/usr/bin/python3
import os,signal,struct,sys,time,wave
signal.signal(signal.SIGINT,lambda *_:sys.exit(0))
def frame(kind,data):
 sys.stdout.buffer.write(bytes([kind])+struct.pack('<I',len(data))+data)
 sys.stdout.buffer.flush()
with wave.open(os.environ['CUE_TEST_SHERPA_WAV'],'rb') as f:
 assert f.getframerate()==16000 and f.getnchannels()==1 and f.getsampwidth()==2
 pcm=f.readframes(f.getnframes())
frame(0,b'')
for at in range(0,len(pcm),3200):
 frame(1,pcm[at:at+3200])
 frame(2,pcm[at:at+3200])
 time.sleep(.1)
for _ in range(15):
 frame(1,bytes(3200));frame(2,bytes(3200));time.sleep(.1)
signal.pause()
`
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUE_AUDIO_HELPER", helper)
	t.Setenv("CUE_TEST_SHERPA_WAV", fixture)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var mu sync.Mutex
	partials, finals := map[string]int{}, map[string]int{}
	done := make(chan struct{}, 1)
	s, err := Start(ctx, model.Settings{Speech: model.Endpoint{Provider: "sherpa", Stream: true}, Language: "zh", MeetingAudio: true}, nil, func(ch, text string, final bool) {
		mu.Lock()
		defer mu.Unlock()
		if text != "" {
			if final {
				finals[ch]++
			} else {
				partials[ch]++
			}
		}
		if finals["you"] > 0 && finals["them"] > 0 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}, func(err error) { t.Errorf("完整链路出错：%v", err) })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("采集协议到流式识别没有出字")
	}
	s.Stop()
	for _, ch := range []string{"you", "them"} {
		if partials[ch] < 2 || finals[ch] != 1 {
			t.Fatalf("通道%s更新/最终结果错误：%v/%v", ch, partials, finals)
		}
	}
}

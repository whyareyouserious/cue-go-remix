package sherpa

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Opt-in test replays public Mandarin fixtures through the actual shipped
// native model. No microphone, system capture, remote API or paid service.
func TestRealStreamingMandarin(t *testing.T) {
	dir, wav := os.Getenv("CUE_TEST_SHERPA_DIR"), os.Getenv("CUE_TEST_SHERPA_WAV")
	if dir == "" || wav == "" {
		t.Skip("未配置真实中文流式模型与公开样本")
	}
	dir, _ = filepath.Abs(dir)
	b, err := os.ReadFile(wav)
	if err != nil {
		t.Fatal(err)
	}
	var pcm []int16
	for at := 12; at+8 <= len(b); {
		n := int(binary.LittleEndian.Uint32(b[at+4 : at+8]))
		end := at + 8 + n
		if end > len(b) {
			t.Fatal("WAV数据不完整")
		}
		if string(b[at:at+4]) == "fmt " {
			if n < 16 || binary.LittleEndian.Uint16(b[at+8:]) != 1 || binary.LittleEndian.Uint16(b[at+10:]) != 1 || binary.LittleEndian.Uint32(b[at+12:]) != 16000 || binary.LittleEndian.Uint16(b[at+22:]) != 16 {
				t.Fatal("需要16kHz单声道PCM16样本")
			}
		}
		if string(b[at:at+4]) == "data" {
			for i := at + 8; i+2 <= end; i += 2 {
				pcm = append(pcm, int16(binary.LittleEndian.Uint16(b[i:i+2])))
			}
		}
		at = end + n%2
	}
	if len(pcm) == 0 {
		t.Fatal("空音频")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var mu sync.Mutex
	partials := map[string]int{}
	first := map[string]time.Duration{}
	finals := map[string][]string{}
	var sentAudio int64
	var decodeMax int64
	started := time.Now()
	s, err := StartAt(ctx, dir, true, func(ch, text string, final bool) {
		mu.Lock()
		defer mu.Unlock()
		if final {
			finals[ch] = append(finals[ch], text)
		} else if text != "" {
			partials[ch]++
			if first[ch] == 0 {
				first[ch] = time.Since(started)
			}
		}
	}, func(err error) { t.Errorf("流式识别失败：%v", err) }, func(m Metric) {
		mu.Lock()
		defer mu.Unlock()
		if m.Phase == "slow-decode" && m.ElapsedMS > decodeMax {
			decodeMax = m.ElapsedMS
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	load := time.Since(started)
	started = time.Now()
	// Replay at real time, including both channels, to measure displayed latency
	// rather than confuse audio duration with compute time.
	for at := 0; at < len(pcm); at += 1600 {
		end := min(at+1600, len(pcm))
		for _, ch := range []string{"you", "them"} {
			if err = s.Send(ch, pcm[at:end]); err != nil {
				t.Fatal(err)
			}
		}
		if wait := time.Duration(end)*time.Second/16000 - time.Since(started); wait > 0 {
			time.Sleep(wait)
		}
		sentAudio = int64(end)
	}
	// A natural pause must finalize before Stop, without a second copy on drain.
	for i := 0; i < 15; i++ {
		for _, ch := range []string{"you", "them"} {
			if err = s.Send(ch, make([]int16, 1600)); err != nil {
				t.Fatal(err)
			}
		}
		sentAudio += 1600
		if wait := time.Duration(sentAudio)*time.Second/16000 - time.Since(started); wait > 0 {
			time.Sleep(wait)
		}
	}
	mu.Lock()
	beforeStop := partials["you"] > 1 && partials["them"] > 1
	finalBeforeStop := len(finals["you"]) > 0 && len(finals["them"]) > 0
	mu.Unlock()
	stop := time.Now()
	if err = s.Stop(); err != nil {
		t.Fatal(err)
	}
	if !beforeStop {
		t.Fatal("说话期间未多次产生增量字幕")
	}
	if !finalBeforeStop {
		t.Fatal("停顿1.5秒仍未自动确认句子")
	}
	for _, ch := range []string{"you", "them"} {
		text := strings.Join(finals[ch], " ")
		t.Logf("通道=%s 模型加载=%s 首次暂定=%s 更新次数=%d 尾句收尾=%s 结果=%s", ch, load, first[ch], partials[ch], time.Since(stop), text)
		if text == "" {
			t.Fatal("未保存最终结果")
		}
		if len(finals[ch]) != 1 {
			t.Fatalf("一段音频重复提交了%d条最终记录", len(finals[ch]))
		}
		if expected := os.Getenv("CUE_TEST_SHERPA_EXPECT"); expected != "" && text != expected {
			t.Fatalf("中文结果不符，实际：%s", text)
		}
		if first[ch] > 4*time.Second {
			t.Fatal("首字延迟超过4秒")
		}
	}
}

//go:build darwin

package platform

import (
	"context"
	"encoding/binary"
	"os"
	"os/signal"
	"sync/atomic"
	"testing"
	"time"
)

// The real executable is replaced by this test process. It speaks the helper
// wire protocol without requesting microphone or screen permissions.
func TestMain(m *testing.M) {
	if mode := os.Getenv("CUE_TEST_CAPTURE_MODE"); mode != "" {
		fakeCapture(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func fakeCapture(mode string) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	frame := func(kind byte, data []byte) {
		b := make([]byte, 5+len(data))
		b[0] = kind
		binary.LittleEndian.PutUint32(b[1:5], uint32(len(data)))
		copy(b[5:], data)
		if _, err := os.Stdout.Write(b); err != nil {
			os.Exit(0)
		}
	}
	if mode == "start-error" {
		frame(3, []byte("麦克风权限被拒绝"))
		return
	}
	frame(0, nil)
	if mode == "recovery" {
		frame(4, []byte("microphone-config-changed"))
		frame(4, []byte("private-device-name-must-not-be-logged"))
		frame(4, []byte("microphone-restarting"))
		frame(4, []byte("microphone-restarted"))
	}
	frame(1, []byte{0, 0, 1, 0, 255, 255})
	if mode == "runtime-error" {
		frame(3, []byte("设备已断开"))
		<-signals
		return
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-signals:
			return
		case <-ticker.C:
			frame(2, []byte{2, 0, 3, 0})
		}
	}
}

func TestAudioRecoveryEventsAreWhitelisted(t *testing.T) {
	useFakeCapture(t, "recovery")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	states := make(chan string, 8)
	stop, err := StartAudio(ctx, false, nil, func(err error) { t.Error(err) }, func(state string) { states <- state })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	for _, expected := range []string{"microphone-config-changed", "microphone-restarting", "microphone-restarted"} {
		select {
		case actual := <-states:
			if actual != expected {
				t.Fatalf("事件错误：%s", actual)
			}
		case <-ctx.Done():
			t.Fatal("缺少采集恢复日志")
		}
	}
	stop()
	if len(states) != 0 {
		t.Fatal("未过滤未知诊断内容")
	}
}
func useFakeCapture(t *testing.T, mode string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUE_AUDIO_HELPER", exe)
	t.Setenv("CUE_TEST_CAPTURE_MODE", mode)
}
func TestAudioStopDrainsAndJoinsHelper(t *testing.T) {
	useFakeCapture(t, "normal")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := make(chan string, 10)
	var returned atomic.Bool
	stop, err := StartAudio(ctx, true, func(channel string, pcm []int16) {
		if returned.Load() {
			t.Error("stop 返回后仍有音频回调")
		}
		if channel == "you" && (len(pcm) != 3 || pcm[1] != 1 || pcm[2] != -1) {
			t.Error("PCM 字节序不正确")
		}
		select {
		case got <- channel:
		default:
		}
	}, func(err error) { t.Errorf("意外音频错误：%v", err) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case ch := <-got:
			seen[ch] = true
		case <-ctx.Done():
			t.Fatal("未收到双路音频")
		}
	}
	stop()
	returned.Store(true)
	stop() // idempotent and non-blocking after the first stop
}
func TestAudioStartupAndRuntimeErrors(t *testing.T) {
	for _, mode := range []string{"start-error", "runtime-error"} {
		t.Run(mode, func(t *testing.T) {
			useFakeCapture(t, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errors := make(chan error, 1)
			stop, err := StartAudio(ctx, false, nil, func(err error) { errors <- err })
			if mode == "start-error" {
				if err == nil {
					stop()
					t.Fatal("启动失败未返回错误")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			select {
			case err := <-errors:
				if err == nil {
					t.Fatal("缺少运行错误")
				}
			case <-ctx.Done():
				t.Fatal("运行错误未传播")
			}
		})
	}
}
func TestAudioMissingHelper(t *testing.T) {
	t.Setenv("CUE_AUDIO_HELPER", "/does-not-exist/cue-audio")
	// Explicit override must not silently fall through to a different helper.
	_, err := audioHelperPath()
	if err == nil {
		t.Fatal("无效显式路径应返回错误")
	}
}

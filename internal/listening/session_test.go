package listening

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cue-go-remix/internal/model"
	"cue-go-remix/internal/provider"
)

func voice(n int) []int16 {
	v := make([]int16, n)
	for i := range v {
		v[i] = 2000
	}
	return v
}

func segmented(t *testing.T, pcm []int16, chunk int) [][]int16 {
	t.Helper()
	var got [][]int16
	s := newSession(context.Background(), model.Settings{}, nil, func(err error) { t.Errorf("意外错误：%v", err) })
	s.transcribe = func(_ context.Context, pcm []int16) (string, error) {
		got = append(got, append([]int16(nil), pcm...))
		return "", nil
	}
	go s.worker()
	for offset := 0; offset < len(pcm); offset += chunk {
		end := offset + chunk
		if end > len(pcm) {
			end = len(pcm)
		}
		s.feed("you", pcm[offset:end])
	}
	s.Stop()
	return got
}

func TestVADPreservesShortRepliesAndIgnoresClicks(t *testing.T) {
	short := append(voice(1600), make([]int16, 11200)...)
	if got := segmented(t, short, 640); len(got) != 1 {
		t.Fatalf("100ms短回答应保留，得到%d段", len(got))
	}
	click := append(voice(320), make([]int16, 11200)...)
	if got := segmented(t, click, 640); len(got) != 0 {
		t.Fatalf("20ms短脉冲不应转写，得到%d段", len(got))
	}
	if got := segmented(t, make([]int16, 32000), 511); len(got) != 0 {
		t.Fatal("纯静音不能转写")
	}
}

func TestVADDoesNotDependOnDevicePacketBoundaries(t *testing.T) {
	pcm := append(make([]int16, 2301), voice(4109)...)
	pcm = append(pcm, make([]int16, 12517)...)
	pcm = append(pcm, voice(1671)...)
	whole := segmented(t, pcm, len(pcm))
	for _, size := range []int{1, 7, 319, 320, 511, 1600} {
		got := segmented(t, pcm, size)
		if !reflect.DeepEqual(got, whole) {
			t.Fatalf("分包大小%d改变了转写边界", size)
		}
	}
	if len(whole) != 2 {
		t.Fatalf("应保留两个短回答，得到%d段", len(whole))
	}
}

func TestStopDrainsTailBeforeDone(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	stopped := make(chan struct{})
	var received []int16
	var text string
	s := newSession(context.Background(), model.Settings{}, func(channel, value string, final bool) {
		if channel != "them" || !final {
			t.Error("声道/最终字幕标记错误")
		}
		text = value
	}, func(err error) { t.Errorf("意外错误：%v", err) })
	s.transcribe = func(ctx context.Context, pcm []int16) (string, error) {
		received = append([]int16(nil), pcm...)
		close(entered)
		select {
		case <-release:
			return "最后一句", nil
		case <-ctx.Done():
			t.Error("正常停止提前取消了尾句")
			return "", ctx.Err()
		}
	}
	go s.worker()
	s.feed("them", voice(1580))
	s.stopCapture = func() { s.feed("them", voice(60)) }
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("停止时未送出最后的音频")
	}
	select {
	case <-s.Done():
		t.Fatal("尾句完成前Done不应关闭")
	default:
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("完成尾句后停止未返回")
	}
	if len(received) != 1640 || text != "最后一句" {
		t.Fatalf("尾句数据丢失：样本%d，文本%q", len(received), text)
	}
	if s.Err() != nil {
		t.Fatalf("正常停止不应有致命错误：%v", s.Err())
	}
	select {
	case <-s.Done():
	default:
		t.Error("停止完成应关闭Done")
	}
	s.feed("them", voice(3200)) // Closed sessions never enqueue into the closed queue.
	s.Stop()
}

type fakeStream struct {
	send   func([]int16) error
	closed atomic.Int32
}

func (f *fakeStream) Send(pcm []int16) error {
	if f.send != nil {
		return f.send(pcm)
	}
	return nil
}
func (f *fakeStream) Close() error { f.closed.Add(1); return nil }

func TestSlowStreamDoesNotBlockOtherChannel(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	otherSent := make(chan struct{})
	s := newSession(context.Background(), model.Settings{}, nil, func(err error) { t.Errorf("意外错误：%v", err) })
	s.transcribe = func(context.Context, []int16) (string, error) { return "", nil }
	you := &fakeStream{send: func([]int16) error { close(blocked); <-release; return nil }}
	them := &fakeStream{send: func([]int16) error { close(otherSent); return nil }}
	s.streams["you"] = you
	s.streams["them"] = them
	go s.worker()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); s.feed("you", voice(320)) }()
	<-blocked
	go s.feed("them", voice(320))
	select {
	case <-otherSent:
	case <-time.After(time.Second):
		close(release)
		wg.Wait()
		s.Stop()
		t.Fatal("一个声道的网络发送阻塞了另一声道")
	}
	close(release)
	wg.Wait()
	s.Stop()
	if you.closed.Load() != 1 || them.closed.Load() != 1 {
		t.Error("双声道未各关闭一次")
	}
}

func TestFatalStreamErrorReportedOnceAndStopsCapture(t *testing.T) {
	var reported atomic.Int32
	s := newSession(context.Background(), model.Settings{}, nil, func(err error) {
		var fatal *FatalError
		if !errors.As(err, &fatal) {
			t.Errorf("应为FatalError: %v", err)
		}
		reported.Add(1)
	})
	s.transcribe = func(context.Context, []int16) (string, error) { return "", nil }
	s.streams["you"] = &fakeStream{send: func([]int16) error { return errors.New("socket closed") }}
	go s.worker()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.feed("you", voice(320)) }()
	}
	wg.Wait()
	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("致命错误没有取消采集context")
	}
	if reported.Load() != 1 || s.Err() == nil {
		t.Fatalf("错误重复上报或未保存：%d %v", reported.Load(), s.Err())
	}
	s.Stop()
}

func TestInvalidSpeechKeyStopsQueuedRequests(t *testing.T) {
	var calls atomic.Int32
	s := newSession(context.Background(), model.Settings{}, nil, nil)
	s.transcribe = func(context.Context, []int16) (string, error) {
		calls.Add(1)
		return "", &provider.APIError{Status: 401, Message: "API Key 无效"}
	}
	go s.worker()
	for i := 0; i < 5; i++ {
		s.feed("you", append(voice(1600), make([]int16, 11200)...))
	}
	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("无效Key没有停止会话")
	}
	s.Stop()
	if calls.Load() != 1 {
		t.Fatalf("无效Key继续发了%d次请求", calls.Load())
	}
}

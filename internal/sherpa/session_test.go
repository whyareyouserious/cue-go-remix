package sherpa

import (
	"context"
	"sync"
	"testing"
	"time"
	"unsafe"
)

type fakeDecoder struct {
	next          uintptr
	ready         map[uintptr]bool
	texts         map[uintptr]string
	finished      map[uintptr]bool
	decodeEntered chan struct{}
	decodeRelease chan struct{}
	once          sync.Once
	closed        bool
	destroyed     int
}

func fakeNative() *fakeDecoder {
	return &fakeDecoder{ready: map[uintptr]bool{}, texts: map[uintptr]string{}, finished: map[uintptr]bool{}}
}
func (d *fakeDecoder) NewStream() (uintptr, error)   { d.next++; return d.next, nil }
func (d *fakeDecoder) Accept(s uintptr, p []float32) { d.ready[s] = true }
func (d *fakeDecoder) Ready(s uintptr) bool          { return d.ready[s] }
func (d *fakeDecoder) Decode(s uintptr) {
	if d.decodeEntered != nil {
		d.once.Do(func() { close(d.decodeEntered); <-d.decodeRelease })
	}
	d.ready[s] = false
	if s == 1 {
		d.texts[s] = "這個轉換"
	} else {
		d.texts[s] = "對方問題"
	}
}
func (d *fakeDecoder) Result(s uintptr) (string, error) { return d.texts[s], nil }
func (d *fakeDecoder) Endpoint(s uintptr) bool          { return false }
func (d *fakeDecoder) Reset(s uintptr)                  { d.texts[s] = "" }
func (d *fakeDecoder) Finish(s uintptr)                 { d.finished[s] = true }
func (d *fakeDecoder) DestroyStream(s uintptr)          { d.destroyed++ }
func (d *fakeDecoder) Close()                           { d.closed = true }

// Capture callbacks must reach an interim result without silence or Stop.
// This catches the former 15-second/final-only behavior at the stream boundary.
func TestInterimBeforePauseAndStopDrainsBothChannels(t *testing.T) {
	type result struct {
		channel, text string
		final         bool
	}
	events := make(chan result, 20)
	d := fakeNative()
	s, err := startWithDecoder(context.Background(), d, true, func(c, v string, f bool) { events <- result{c, v, f} }, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range []string{"you", "them"} {
		if err = s.Send(ch, make([]int16, 1600)); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-events:
			want := "这个转换"
			if ch == "them" {
				want = "对方问题"
			}
			if got.channel != ch || got.text != want || got.final {
				t.Fatalf("暂停前未输出对应的简体暂定字幕：%+v", got)
			}
		case <-time.After(time.Second):
			t.Fatal("连续音频未及时输出暂定字幕")
		}
	}
	if err = s.Stop(); err != nil {
		t.Fatal(err)
	}
	if !d.closed || d.destroyed != 2 {
		t.Fatalf("资源未释放：%+v", d)
	}
	close(events)
	finals := map[string]int{}
	for e := range events {
		if e.final {
			finals[e.channel]++
		}
	}
	if finals["you"] != 1 || finals["them"] != 1 {
		t.Fatalf("停止应各保存一次尾句：%v", finals)
	}
	if err = s.Send("you", []int16{1}); err == nil {
		t.Fatal("停止后仍接受输入")
	}
	if err = s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestSlowDecoderNeverBlocksCaptureAndBoundsBacklog(t *testing.T) {
	d := fakeNative()
	d.decodeEntered = make(chan struct{})
	d.decodeRelease = make(chan struct{})
	s, err := startWithDecoder(context.Background(), d, true, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Send("you", make([]int16, 1600)); err != nil {
		t.Fatal(err)
	}
	<-d.decodeEntered
	if err = s.Send("them", make([]int16, 1600)); err != nil {
		t.Fatal(err)
	}
	if err = s.Send("you", make([]int16, 48000)); err != nil {
		t.Fatal(err)
	}
	if err = s.Send("you", make([]int16, 1600)); err == nil {
		t.Fatal("没有阻止超过3秒的音频积压")
	}
	close(d.decodeRelease)
	if err = s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledStreamReleasesNativeState(t *testing.T) {
	d := fakeNative()
	ctx, cancel := context.WithCancel(context.Background())
	s, err := startWithDecoder(ctx, d, false, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = s.Stop()
	if !d.closed || d.destroyed != 1 {
		t.Fatal("取消后未释放模型/流")
	}
}

func TestNativeConfigABI64(t *testing.T) {
	// Values verified by compiling offsetof/sizeof against the pinned C header.
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("当前发布目标均为64位")
	}
	var m modelConfig
	var c recognizerConfig
	if unsafe.Sizeof(m) != 136 || unsafe.Sizeof(c) != 272 {
		t.Fatalf("C ABI布局错误 model=%d recognizer=%d", unsafe.Sizeof(m), unsafe.Sizeof(c))
	}
	if unsafe.Offsetof(c.DecodingMethod) != 144 || unsafe.Offsetof(c.HotwordsFile) != 176 || unsafe.Offsetof(c.HRDirectory) != 248 {
		t.Fatal("C ABI字段偏移错误")
	}
}

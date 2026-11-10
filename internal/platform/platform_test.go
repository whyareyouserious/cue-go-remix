package platform

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestScreenshotResizesAndPreservesAspect(t *testing.T) {
	src := image.NewRGBA(image.Rect(11, 17, 3211, 1817))
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			src.SetRGBA(x, y, color.RGBA{50, 120, 200, 255})
		}
	}
	encoded, err := encodeScreenshot(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "data:image/jpeg;base64,") {
		t.Fatal("非 JPEG 数据 URL")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, "data:image/jpeg;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 1568 || got.Y != 882 {
		t.Fatalf("截图尺寸错误：%v", got)
	}
	r, g, b, _ := img.At(20, 20).RGBA()
	if math.Abs(float64(r>>8)-50) > 3 || math.Abs(float64(g>>8)-120) > 3 || math.Abs(float64(b>>8)-200) > 3 {
		t.Fatal("截图颜色转换错误")
	}
}

func TestResamplerPacketBoundariesDoNotChangeSignal(t *testing.T) {
	for _, rate := range []int{16000, 44100, 48000, 96000} {
		samples := make([]float64, rate)
		for i := range samples {
			samples[i] = 0.25 * math.Sin(2*math.Pi*440*float64(i)/float64(rate))
		}
		want := newResampler(rate).push(samples)
		r := newResampler(rate)
		var got []int16
		for start := 0; start < len(samples); start += 127 {
			got = append(got, r.push(samples[start:min(start+127, len(samples))])...)
		}
		if len(got) != 16000 {
			t.Fatalf("%d Hz: 采样时长改变，输出 %d", rate, len(got))
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%d Hz: 分包改变音频", rate)
		}
		peak := int16(0)
		for _, v := range got {
			if v > peak {
				peak = v
			}
		}
		if peak < 8000 || peak > 8192 {
			t.Fatalf("%d Hz: 音量错误 %d", rate, peak)
		}
	}
}

func TestResamplerClipsToPCM16(t *testing.T) {
	got := newResampler(16000).push([]float64{-2, -1, 0, 1, 2})
	want := []int16{-32767, -32767, 0, 32767, 32767}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("剪裁错误：%v", got)
	}
}

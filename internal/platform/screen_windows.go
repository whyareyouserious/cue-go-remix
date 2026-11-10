//go:build windows

package platform

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var gdi32 = syscall.NewLazyDLL("gdi32.dll")

type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}
type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

func Screenshot(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// A screen DC must be released on the same thread on which it was obtained.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w, _, _ := user32.NewProc("GetSystemMetrics").Call(0)
	h, _, _ := user32.NewProc("GetSystemMetrics").Call(1)
	if w == 0 || h == 0 || w > 32768 || h > 32768 {
		return "", fmt.Errorf("无法读取主屏幕大小")
	}
	dc, _, err := user32.NewProc("GetDC").Call(0)
	if dc == 0 {
		return "", fmt.Errorf("无法获取屏幕设备：%v", err)
	}
	defer user32.NewProc("ReleaseDC").Call(0, dc)
	mem, _, err := gdi32.NewProc("CreateCompatibleDC").Call(dc)
	if mem == 0 {
		return "", fmt.Errorf("无法建立截图缓冲区：%v", err)
	}
	defer gdi32.NewProc("DeleteDC").Call(mem)
	info := bitmapInfo{Header: bitmapInfoHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}}
	var pixels unsafe.Pointer
	bmp, _, err := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if bmp == 0 || pixels == nil {
		return "", fmt.Errorf("无法创建截图位图：%v", err)
	}
	defer gdi32.NewProc("DeleteObject").Call(bmp)
	old, _, _ := gdi32.NewProc("SelectObject").Call(mem, bmp)
	defer gdi32.NewProc("SelectObject").Call(mem, old)
	ok, _, err := gdi32.NewProc("BitBlt").Call(mem, 0, 0, w, h, dc, 0, 0, 0x00CC0020|0x40000000)
	if ok == 0 {
		return "", fmt.Errorf("屏幕截图失败：%v", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	raw := unsafe.Slice((*byte)(pixels), int(w*h*4))
	for i := 0; i < len(raw); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = raw[i+2], raw[i+1], raw[i], 255
	}
	return encodeScreenshot(img)
}

func Capabilities() map[string]any {
	return map[string]any{"platform": "windows", "screen": true, "microphone": true, "systemAudio": true, "permissions": []string{"允许桌面应用访问麦克风"}, "note": "系统音频通过 WASAPI 回环采集默认播放设备；修改默认音频设备后请重启听写。"}
}

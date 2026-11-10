//go:build windows

package platform

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var ole32 = syscall.NewLazyDLL("ole32.dll")

type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}

var classEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
var iidEnumerator = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
var iidAudioClient = guid{0x1CB9AD4C, 0xDBFA, 0x4c32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
var iidCaptureClient = guid{0xC8ADBD64, 0xE71E, 0x48A0, [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}

//go:uintptrescapes
//go:noinline
func comCall(obj uintptr, index int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	method := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	args = append([]uintptr{obj}, args...)
	r, _, _ := syscall.SyscallN(method, args...)
	return r
}
func hresult(r uintptr, what string) error {
	if int32(uint32(r)) < 0 {
		return fmt.Errorf("%s失败（HRESULT 0x%08X），请检查音频设备和麦克风权限", what, uint32(r))
	}
	return nil
}

// StartAudio opens the default communications microphone and the default render
// endpoint independently. All COM objects stay on the goroutine's locked thread.
func StartAudio(parent context.Context, system bool, onPCM func(string, []int16), onError func(error), onState ...func(string)) (func(), error) {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	var once sync.Once
	stop := func() { once.Do(cancel); wg.Wait() }
	channels := []string{"you"}
	if system {
		channels = append(channels, "them")
	}
	ready := make(chan error, len(channels))
	for _, channel := range channels {
		wg.Add(1)
		go func(channel string) {
			defer wg.Done()
			started := false
			err := captureWASAPI(ctx, channel, func() { started = true; ready <- nil }, onPCM)
			if !started {
				ready <- err
				return
			}
			if err != nil && ctx.Err() == nil && onError != nil {
				onError(err)
			}
		}(channel)
	}
	for range channels {
		select {
		case err := <-ready:
			if err != nil {
				stop()
				return nil, err
			}
		case <-ctx.Done():
			stop()
			return nil, ctx.Err()
		}
	}
	return stop, nil
}

func captureWASAPI(ctx context.Context, channel string, ready func(), onPCM func(string, []int16)) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 0)
	if err := hresult(hr, "初始化音频 COM"); err != nil {
		return err
	}
	defer ole32.NewProc("CoUninitialize").Call()
	var enumerator uintptr
	hr, _, _ = ole32.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&classEnumerator)), 0, 23, uintptr(unsafe.Pointer(&iidEnumerator)), uintptr(unsafe.Pointer(&enumerator)))
	if err := hresult(hr, "枚举音频设备"); err != nil {
		return err
	}
	defer comCall(enumerator, 2)
	flow, role := uintptr(1), uintptr(2) // eCapture / eCommunications
	flags := uintptr(0)
	if channel == "them" {
		flow, role, flags = 0, 0, 0x00020000
	} // eRender / eConsole / LOOPBACK
	var device uintptr
	if err := hresult(comCall(enumerator, 4, flow, role, uintptr(unsafe.Pointer(&device))), "获取默认音频设备"); err != nil {
		return err
	}
	defer comCall(device, 2)
	var client uintptr
	if err := hresult(comCall(device, 3, uintptr(unsafe.Pointer(&iidAudioClient)), 23, 0, uintptr(unsafe.Pointer(&client))), "打开音频设备"); err != nil {
		return err
	}
	defer comCall(client, 2)
	var format uintptr
	if err := hresult(comCall(client, 8, uintptr(unsafe.Pointer(&format))), "读取音频格式"); err != nil {
		return err
	}
	defer ole32.NewProc("CoTaskMemFree").Call(format)
	// WAVEFORMATEX has a packed 18-byte layout; reading byte offsets avoids Go
	// struct padding and also handles WAVEFORMATEXTENSIBLE's SubFormat GUID.
	fmtBytes := unsafe.Slice((*byte)(unsafe.Pointer(format)), 18)
	tag := binary.LittleEndian.Uint16(fmtBytes[0:2])
	channels := int(binary.LittleEndian.Uint16(fmtBytes[2:4]))
	rate := int(binary.LittleEndian.Uint32(fmtBytes[4:8]))
	blockAlign := int(binary.LittleEndian.Uint16(fmtBytes[12:14]))
	bits := int(binary.LittleEndian.Uint16(fmtBytes[14:16]))
	if tag == 0xfffe && binary.LittleEndian.Uint16(fmtBytes[16:18]) >= 22 {
		full := unsafe.Slice((*byte)(unsafe.Pointer(format)), 40)
		tag = uint16(binary.LittleEndian.Uint32(full[24:28]))
	}
	if channels < 1 || channels > 32 || rate < 8000 || rate > 384000 || blockAlign < channels*bits/8 {
		return fmt.Errorf("音频设备格式无效：%d 声道，%d Hz", channels, rate)
	}
	if (tag != 1 && tag != 3) || (tag == 3 && bits != 32 && bits != 64) || (tag == 1 && bits != 8 && bits != 16 && bits != 24 && bits != 32) {
		return fmt.Errorf("音频格式不受支持：编码 %d，%d 位", tag, bits)
	}
	// Shared mode consumes the endpoint's native format. No forced sample-rate
	// negotiation is required; mono/16k conversion happens in Go afterwards.
	if err := hresult(comCall(client, 3, 0, flags, 1000000, 0, format, 0), "初始化音频采集"); err != nil {
		return err
	}
	var capture uintptr
	if err := hresult(comCall(client, 14, uintptr(unsafe.Pointer(&iidCaptureClient)), uintptr(unsafe.Pointer(&capture))), "打开音频采集通道"); err != nil {
		return err
	}
	defer comCall(capture, 2)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := hresult(comCall(client, 10), "启动音频采集"); err != nil {
		return err
	}
	defer comCall(client, 11)
	ready()
	r := newResampler(rate)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		for {
			var packet uint32
			if err := hresult(comCall(capture, 5, uintptr(unsafe.Pointer(&packet))), "读取音频包"); err != nil {
				return err
			}
			if packet == 0 {
				break
			}
			var data uintptr
			var count, bufFlags uint32
			if err := hresult(comCall(capture, 3, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&bufFlags)), 0, 0), "获取音频缓冲区"); err != nil {
				return err
			}
			mono := make([]float64, int(count))
			if bufFlags&2 == 0 && data != 0 {
				raw := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(count)*blockAlign)
				bytesPerSample := bits / 8
				for frame := range mono {
					for ch := 0; ch < channels; ch++ {
						p := raw[frame*blockAlign+ch*bytesPerSample:]
						mono[frame] += decodeSample(p, tag, bits) / float64(channels)
					}
				}
			}
			if err := hresult(comCall(capture, 4, uintptr(count)), "释放音频缓冲区"); err != nil {
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
			pcm := r.push(mono)
			if len(pcm) > 0 && onPCM != nil {
				onPCM(channel, pcm)
			}
		}
	}
}

func decodeSample(p []byte, tag uint16, bits int) float64 {
	if tag == 3 {
		if bits == 32 {
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(p)))
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(p))
	}
	switch bits {
	case 8:
		return (float64(p[0]) - 128) / 128
	case 16:
		return float64(int16(binary.LittleEndian.Uint16(p))) / 32768
	case 24:
		v := int32(p[0]) | int32(p[1])<<8 | int32(p[2])<<16
		v = v << 8 >> 8
		return float64(v) / 8388608
	default:
		return float64(int32(binary.LittleEndian.Uint32(p))) / 2147483648
	}
}

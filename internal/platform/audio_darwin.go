//go:build darwin

package platform

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

func audioHelperPath() (string, error) {
	var paths []string
	if override := os.Getenv("CUE_AUDIO_HELPER"); override != "" {
		if info, err := os.Stat(override); err != nil || info.IsDir() || info.Mode()&0111 == 0 {
			return "", fmt.Errorf("CUE_AUDIO_HELPER 不是可执行的音频组件：%s", override)
		}
		return filepath.Abs(override)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(dir, "cue-audio"), filepath.Join(dir, "..", "Helpers", "cue-audio"), filepath.Join(dir, "..", "Resources", "cue-audio"))
	}
	paths = append(paths, filepath.Join("native", "bin", "cue-audio"))
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return filepath.Abs(p)
		}
	}
	return "", errors.New("缺少 macOS 音频组件 cue-audio，请先运行 macOS 构建命令或设置 CUE_AUDIO_HELPER 路径")
}

func StartAudio(parent context.Context, system bool, onPCM func(string, []int16), onError func(error), onState ...func(string)) (func(), error) {
	path, err := audioHelperPath()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	args := []string{}
	if system {
		args = append(args, "--system")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 3 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	// The helper emits structured error frames. OS runtime diagnostics remain
	// on stderr, separate from PCM and free of user audio content.
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("启动音频组件失败：%w", err)
	}
	ready := make(chan error, 1)
	done := make(chan struct{})
	var once sync.Once
	var stopped atomic.Bool
	stop := func() { stopped.Store(true); once.Do(cancel); <-done }
	go func() {
		defer close(done)
		started := false
		var captureErr error
		for {
			var header [5]byte
			if _, err := io.ReadFull(stdout, header[:]); err != nil {
				if !errors.Is(err, io.EOF) && ctx.Err() == nil {
					captureErr = fmt.Errorf("音频组件输出中断：%w", err)
				}
				break
			}
			n := binary.LittleEndian.Uint32(header[1:])
			if n > 1024*1024 {
				captureErr = errors.New("音频组件输出了过大的数据帧")
				cancel()
				break
			}
			payload := make([]byte, int(n))
			if _, err := io.ReadFull(stdout, payload); err != nil {
				captureErr = fmt.Errorf("音频数据帧不完整：%w", err)
				break
			}
			switch header[0] {
			case 0:
				if !started {
					started = true
					ready <- nil
				}
			case 1, 2:
				if n%2 != 0 {
					captureErr = errors.New("音频组件 PCM 数据格式错误")
					cancel()
					break
				}
				if !started || ctx.Err() != nil {
					continue
				}
				pcm := make([]int16, int(n)/2)
				for i := range pcm {
					pcm[i] = int16(binary.LittleEndian.Uint16(payload[i*2:]))
				}
				channel := "you"
				if header[0] == 2 {
					channel = "them"
				}
				if onPCM != nil {
					onPCM(channel, pcm)
				}
			case 3:
				captureErr = fmt.Errorf("音频采集失败：%s", string(payload))
				cancel()
			case 4:
				// Only fixed lifecycle events cross into persistent diagnostics.
				event := string(payload)
				if (event == "microphone-restarting" || event == "microphone-restarted" || event == "microphone-config-changed") && len(onState) > 0 && onState[0] != nil {
					onState[0](event)
				}
			}
		}
		waitErr := cmd.Wait()
		if captureErr == nil && waitErr != nil && parent.Err() == nil {
			captureErr = fmt.Errorf("音频组件已退出：%w", waitErr)
		}
		if captureErr == nil && started && ctx.Err() == nil {
			captureErr = errors.New("音频组件意外停止，请重新开始会话")
		}
		if !started {
			if captureErr == nil {
				captureErr = errors.New("音频组件在准备完成前退出")
			}
			ready <- captureErr
		} else if captureErr != nil && !stopped.Load() && parent.Err() == nil && onError != nil {
			onError(captureErr)
		}
	}()
	// System permission dialogs can stay open until the user responds.
	select {
	case err := <-ready:
		if err != nil {
			stop()
			return nil, err
		}
		return stop, nil
	case <-parent.Done():
		stop()
		return nil, parent.Err()
	}
}

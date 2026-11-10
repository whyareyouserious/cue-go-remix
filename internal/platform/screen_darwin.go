//go:build darwin

package platform

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
)

func Screenshot(ctx context.Context) (string, error) {
	dir, err := os.MkdirTemp("", "cue-screen-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	file := dir + "/screen.png"
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-D", "1", "-t", "png", file)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("截图失败，请在系统设置中允许 Cue 录制屏幕：%w", err)
	}
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("未获得屏幕截图，请检查屏幕录制权限：%w", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return "", fmt.Errorf("读取屏幕截图失败：%w", err)
	}
	return encodeScreenshot(img)
}

func Capabilities() map[string]any {
	return map[string]any{"platform": "darwin", "screen": true, "microphone": true, "systemAudio": true, "systemAudioMinimum": "macOS 13.0", "permissions": []string{"麦克风", "屏幕与系统音频录制"}, "note": "会议音频需要 ScreenCaptureKit；首次使用需系统授权，开启后会显示系统录屏指示。"}
}

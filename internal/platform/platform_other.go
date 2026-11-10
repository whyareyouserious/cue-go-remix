//go:build !darwin && !windows

package platform

import (
	"context"
	"errors"
	"runtime"
)

func Screenshot(context.Context) (string, error) {
	return "", errors.New("当前平台尚未实现桌面截图，请使用 macOS 或 Windows 版本")
}
func StartAudio(context.Context, bool, func(string, []int16), func(error), ...func(string)) (func(), error) {
	return nil, errors.New("当前平台尚未实现系统音频采集，请使用 macOS 或 Windows 版本")
}
func Capabilities() map[string]any {
	return map[string]any{"platform": runtime.GOOS, "screen": false, "microphone": false, "systemAudio": false}
}

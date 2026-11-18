//go:build !windows

package desktop

import "runtime"

// protectionAvailable 返回当前操作系统是否支持项目使用的录屏排除机制。
func protectionAvailable() bool { return runtime.GOOS == "darwin" }

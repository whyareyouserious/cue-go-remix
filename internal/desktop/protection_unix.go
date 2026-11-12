//go:build !windows

package desktop

import "runtime"

func protectionAvailable() bool { return runtime.GOOS == "darwin" }

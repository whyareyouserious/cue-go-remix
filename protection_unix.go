//go:build !windows

package main

import "runtime"

func protectionAvailable() bool { return runtime.GOOS == "darwin" }

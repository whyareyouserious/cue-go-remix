//go:build !windows

package whisper

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}

//go:build !windows

package applink

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

func privateDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("本地连接目录必须为真实目录")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != uint32(os.Getuid()) {
		return errors.New("本地连接目录不属于当前用户")
	}
	return os.Chmod(dir, 0700)
}
func listenLocal(dir, id string) (net.Listener, string, string, func(), error) {
	path := filepath.Join(dir, "cue-"+id[:12]+".sock")
	cleanup := func() {}
	if len([]byte(path)) >= 104 {
		tmp, err := os.MkdirTemp("/tmp", "cue-link-")
		if err != nil {
			return nil, "", "", cleanup, err
		}
		cleanup = func() { _ = os.Remove(tmp) }
		path = filepath.Join(tmp, "cue.sock")
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		cleanup()
		return nil, "", "", func() {}, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		l.Close()
		cleanup()
		return nil, "", "", func() {}, err
	}
	return l, path, "unix", cleanup, nil
}

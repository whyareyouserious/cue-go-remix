//go:build windows

package applink

import (
	"errors"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
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
	return nil
}
func listenLocal(_ string, id string) (net.Listener, string, string, func(), error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, "", "", func() {}, err
	}
	path := `\\.\pipe\cue-remix-` + id
	// Only this Windows account can connect. The token in the per-user run file
	// is an additional handshake check, not a replacement for the pipe ACL.
	l, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")", InputBufferSize: 65536, OutputBufferSize: 65536})
	return l, path, "npipe", func() {}, err
}

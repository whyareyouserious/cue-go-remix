//go:build windows

package applink

import (
	"github.com/Microsoft/go-winio"
	"net"
	"time"
)

func dialLocal(path string) (net.Conn, error) {
	timeout := 5 * time.Second
	return winio.DialPipe(path, &timeout)
}

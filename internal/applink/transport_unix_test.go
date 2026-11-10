//go:build !windows

package applink

import "net"

func dialLocal(path string) (net.Conn, error) { return net.Dial("unix", path) }

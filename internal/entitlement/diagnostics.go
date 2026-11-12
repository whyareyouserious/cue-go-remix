package entitlement

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"syscall"
)

func (c *Client) log(format string, args ...any) {
	if c.trace != nil {
		c.trace(format, args...)
	}
}

// 操作名来自固定协议，不把任意路径或底层错误原文写入日志。
func licenseOperation(path string) string {
	switch path {
	case "license/activate", "license/heartbeat", "license/deactivate", "trial/activate", "trial/heartbeat":
		return path
	default:
		return "unknown"
	}
}

func networkFailure(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var network net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &network) && network.Timeout()) {
		return "timeout"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "dns"
	}
	var certificate *tls.CertificateVerificationError
	if errors.As(err, &certificate) {
		return "tls_certificate"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection_refused"
	}
	return "network"
}

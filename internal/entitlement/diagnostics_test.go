package entitlement

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"

	secure "cue-go-remix/internal/entitlement/platform"
)

func TestLicenseRequestDiagnosticsDistinguishFailuresWithoutSecrets(t *testing.T) {
	const secret = "PRIVATE-LICENSE-TOKEN-DEVICE"
	for _, test := range []struct {
		name   string
		status int
		body   string
		err    error
		want   string
	}{
		{"proxy404", 404, secret, nil, "http_status=404 api_code=-1 result=http_status"},
		{"upstream502", 502, secret, nil, "http_status=502 api_code=-1 result=http_status"},
		{"business", 200, `{"code":1007,"message":"` + secret + `"}`, nil, "http_status=200 api_code=1007 result=business_error"},
		{"serverError", 200, `{"code":1302}`, nil, "http_status=200 api_code=1302 result=business_error"},
		{"invalidJSON", 200, secret, nil, "http_status=200 api_code=-1 result=invalid_response"},
		{"success", 200, `{"code":0,"data":{"token":"` + secret + `"}}`, nil, "http_status=200 api_code=0 result=ok"},
		{"network", 0, "", errors.New(secret), "http_status=0 api_code=-1 result=network"},
		{"timeout", 0, "", context.DeadlineExceeded, "http_status=0 api_code=-1 result=timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs strings.Builder
			c := NewClient("https://license.example.test/"+secret, false)
			c.trace = func(format string, args ...any) { fmt.Fprintf(&logs, format+"\n", args...) }
			c.http.Transport = bodyTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/cue/license/activate") {
					t.Fatalf("请求未使用 Cue 专用 POST 路由：%s %s", r.Method, r.URL.Path)
				}
				if test.err != nil {
					return nil, test.err
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})
			_, _ = c.request(context.Background(), "license/activate", secret, map[string]string{"licenseKey": secret, "machineId": secret})
			output := logs.String()
			for _, want := range []string{"event=license-request", `origin="https://license.example.test"`, "operation=license/activate", test.want, "elapsed_ms="} {
				if !strings.Contains(output, want) {
					t.Fatalf("缺少诊断字段 %q：%s", want, output)
				}
			}
			if strings.Contains(output, secret) {
				t.Fatal("日志泄露请求、响应、URL 路径或底层错误中的敏感值")
			}
		})
	}
}

func TestManagerLogsInvalidConfigurationBeforeAnyNetworkRequest(t *testing.T) {
	var logs strings.Builder
	m := New(context.Background(), Options{
		BaseURL:   "https://private-user:private-password@example.test/api?token=private-query",
		Trace:     func(format string, args ...any) { fmt.Fprintf(&logs, format+"\n", args...) },
		MachineID: func() (string, error) { return "private-device", nil },
		Load:      func() (secure.Credential, error) { return secure.Credential{}, secure.ErrNotFound },
	})
	t.Cleanup(m.Close)
	called := false
	m.client.http.Transport = bodyTestTransport(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("不应发送请求")
	})
	if err := m.Activate("private-key"); err == nil || called {
		t.Fatal("无效配置必须在联网前拒绝")
	}
	output := logs.String()
	for _, want := range []string{`event=license-config origin="" valid=false`, "result=invalid_endpoint"} {
		if !strings.Contains(output, want) {
			t.Fatalf("缺少配置失败诊断：%s", output)
		}
	}
	if strings.Contains(output, "private-") {
		t.Fatal("无效配置日志不得包含原始地址或凭据")
	}
}

func TestNetworkFailureCategories(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{context.Canceled, "cancelled"},
		{context.DeadlineExceeded, "timeout"},
		{&net.DNSError{Name: "private-host"}, "dns"},
		{&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}, "tls_certificate"},
		{fmt.Errorf("私有信息: %w", syscall.ECONNREFUSED), "connection_refused"},
	} {
		if got := networkFailure(test.err); got != test.want {
			t.Errorf("错误分类=%s，期望=%s", got, test.want)
		}
	}
}

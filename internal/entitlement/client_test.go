package entitlement

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type bodyTestTransport func(*http.Request) (*http.Response, error)

func (f bodyTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type bodyTestReader func([]byte) (int, error)

func (f bodyTestReader) Read(p []byte) (int, error) { return f(p) }

type trackedResponseBody struct {
	io.Reader
	closed bool
}

func (b *trackedResponseBody) Close() error { b.closed = true; return nil }

func clientWithBody(body io.ReadCloser) *Client {
	c := NewClient("https://license.example.test/api", false)
	c.http.Transport = bodyTestTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body, Request: request, Header: make(http.Header)}, nil
	})
	return c
}

func TestClientBodyReadIOErrorsAreTemporaryAndPrivate(t *testing.T) {
	for _, source := range []struct {
		name string
		err  error
	}{
		{"truncated", io.ErrUnexpectedEOF},
		{"connectionReset", errors.New("read failed: SECRET-TEST-TOKEN")},
	} {
		t.Run(source.name, func(t *testing.T) {
			body := &trackedResponseBody{Reader: io.MultiReader(
				strings.NewReader(`{"code":0,"data":{"token":"SECRET-TEST-TOKEN"`),
				bodyTestReader(func([]byte) (int, error) { return 0, source.err }),
			)}
			_, err := clientWithBody(body).request(context.Background(), "license/heartbeat", "", nil)
			if !temporary(err) {
				t.Fatalf("HTTP 正文 I/O 错误应进入断网容错，实际为 %v", err)
			}
			if strings.Contains(err.Error(), "SECRET-TEST-TOKEN") {
				t.Fatal("错误不得包含响应或底层 I/O 敏感文本")
			}
			if !body.closed {
				t.Fatal("读取失败后必须关闭响应体")
			}
		})
	}
}

func TestClientBodyReadCancellationPreservesContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := &trackedResponseBody{Reader: bodyTestReader(func([]byte) (int, error) {
		cancel()
		return 0, io.ErrUnexpectedEOF
	})}
	_, err := clientWithBody(body).request(ctx, "license/heartbeat", "", nil)
	if !errors.Is(err, context.Canceled) || temporary(err) {
		t.Fatalf("取消请求必须保留 context.Canceled，实际为 %v", err)
	}
	if !body.closed {
		t.Fatal("取消读取后必须关闭响应体")
	}
}

func TestClientMalformedAndOversizedBodiesAreNotTemporary(t *testing.T) {
	for _, source := range []struct {
		name string
		body string
	}{
		{"invalidJSON", `{"code":0,"data":{"token":"SECRET-TEST-TOKEN"}`},
		{"missingCode", `{"data":{}}`},
		{"oversizedJSON", `{"code":0,"data":{},"padding":"` + strings.Repeat("x", 64*1024) + `"}`},
	} {
		t.Run(source.name, func(t *testing.T) {
			body := &trackedResponseBody{Reader: strings.NewReader(source.body)}
			_, err := clientWithBody(body).request(context.Background(), "license/heartbeat", "", nil)
			var apiError *APIError
			if !errors.As(err, &apiError) || temporary(err) {
				t.Fatalf("无效响应不得进入断网容错，实际为 %v", err)
			}
			if strings.Contains(err.Error(), "SECRET-TEST-TOKEN") {
				t.Fatal("JSON 格式错误不得泄露响应内容")
			}
			if !body.closed {
				t.Fatal("无效响应必须关闭响应体")
			}
		})
	}
}

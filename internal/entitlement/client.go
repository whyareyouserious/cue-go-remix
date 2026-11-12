package entitlement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Reply 只在 Go 内部使用，Token 不暴露给前端或日志。
type Reply struct {
	Token         string `json:"token"`
	AppStatus     *int   `json:"appStatus"`
	Mode          string `json:"mode"`
	Valid         bool   `json:"valid"`
	ExpiredAt     int64  `json:"expiredAt"`
	ServerTime    int64  `json:"serverTime"`
	LicenseType   string `json:"licenseType"`
	DurationHours int    `json:"durationHours"`
}
type APIError struct {
	Code      int
	Temporary bool
}

func (e *APIError) Error() string {
	if e.Temporary {
		return "暂时无法连接授权服务，请检查网络后重试"
	}
	switch e.Code {
	case 1001:
		return "卡密不存在，请检查输入"
	case 1002:
		return "卡密已冻结，请联系管理员"
	case 1003:
		return "卡密已到期，请更换卡密"
	case 1004:
		return "卡密正在其他设备上使用"
	case 1005:
		return "卡密状态异常，请联系管理员"
	case 1006:
		return "此卡密不属于 Cue"
	case 1007:
		return "此卡密已绑定其他电脑，请联系管理员解绑"
	case 1101:
		return "这台电脑已用完 Cue 的试用，请输入卡密"
	case 1102:
		return "Cue 试用已到期，请输入卡密"
	case 1201:
		return "授权令牌无效，请重新验证"
	case 1202:
		return "授权会话已被撤销，请手动重新验证或联系管理员"
	case 1203:
		return "授权会话已超时，正在重新验证"
	case 1301:
		return "授权请求参数不正确"
	default:
		return "授权服务返回了无效响应，请稍后重试"
	}
}
func temporary(err error) bool { var e *APIError; return errors.As(err, &e) && e.Temporary }

type Client struct {
	base   string
	origin string
	http   *http.Client
	err    error
	trace  func(string, ...any)
}

func NewClient(base string, allowHTTP bool) *Client {
	c := &Client{http: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
		c.err = errors.New("授权服务地址未配置或不支持，请联系发行方")
		return c
	}
	c.base = strings.TrimRight(u.String(), "/")
	c.origin = u.Scheme + "://" + u.Host
	return c
}
func (c *Client) request(ctx context.Context, path, token string, body any) (Reply, error) {
	started := time.Now()
	status, code, result := 0, -1, "invalid_endpoint"
	defer func() {
		c.log("event=license-request origin=%q operation=%s http_status=%d api_code=%d result=%s elapsed_ms=%d",
			c.origin, licenseOperation(path), status, code, result, time.Since(started).Milliseconds())
	}()
	if c.err != nil {
		return Reply{}, c.err
	}
	data, err := json.Marshal(body)
	if err != nil {
		result = "encode_request"
		return Reply{}, &APIError{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/cue/"+path, bytes.NewReader(data))
	if err != nil {
		result = "invalid_request"
		return Reply{}, &APIError{}
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(req)
	if err != nil {
		result = networkFailure(err)
		if ctx.Err() != nil {
			result = networkFailure(ctx.Err())
			return Reply{}, ctx.Err()
		}
		return Reply{}, &APIError{Temporary: true}
	}
	defer response.Body.Close()
	status = response.StatusCode
	result = "http_status"
	if response.StatusCode >= 500 || response.StatusCode == 429 || response.StatusCode == 408 {
		return Reply{}, &APIError{Temporary: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Reply{}, &APIError{}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
	if err != nil && ctx.Err() != nil {
		result = networkFailure(ctx.Err())
		return Reply{}, ctx.Err()
	}
	if len(raw) > 64*1024 {
		result = "response_too_large"
		return Reply{}, &APIError{}
	}
	if err != nil {
		result = "response_read"
		return Reply{}, &APIError{Temporary: true}
	}
	var envelope struct {
		Code *int  `json:"code"`
		Data Reply `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Code == nil {
		result = "invalid_response"
		return Reply{}, &APIError{}
	}
	code = *envelope.Code
	if code != 0 {
		result = "business_error"
		return Reply{}, &APIError{Code: *envelope.Code, Temporary: *envelope.Code == 1302}
	}
	result = "ok"
	return envelope.Data, nil
}

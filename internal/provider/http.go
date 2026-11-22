// Package provider implements the network protocols used by Cue's AI services.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"cue-go-remix/internal/model"
)

const maxResponseBytes = 8 << 20

var errInactive = errors.New("服务连续 25 秒没有返回数据，请重试或切换模型")

var httpClient = &http.Client{
	// Never forward a configured key through an unexpected redirect.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// APIError 保留可识别的 HTTP 状态和固定错误分类，对外消息经过密钥脱敏。
type APIError struct {
	// Status 为 HTTP 状态码，流内业务错误为 0。
	Status int
	// Code 为服务商错误代码，仅用于错误提示，禁止直接写入日志。
	Code string
	// Message 为脱敏后的用户提示。
	Message string
	// Category 仅使用本地定义的分类，空值表示按 HTTP 状态分类。
	Category string
}

// Error 返回已经脱敏的用户提示，不追加原始服务响应。
func (e *APIError) Error() string { return e.Message }

var keyPattern = regexp.MustCompile(`(?i)(bearer\s+|token\s+|api[_-]?key[\s:=]+)[^\s,;"'<>]+|\b(?:sk-|sk_|AIza)[a-zA-Z0-9_-]{8,}`)

func sanitize(s, key string) string {
	if key != "" {
		s = strings.ReplaceAll(s, key, "[密钥已隐藏]")
		s = strings.ReplaceAll(s, url.QueryEscape(key), "[密钥已隐藏]")
	}
	s = keyPattern.ReplaceAllString(s, "[密钥已隐藏]")
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 300 {
		s = string(r[:300]) + "…"
	}
	return s
}

func serviceError(status int, body []byte, key string) error {
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
		Detail  string          `json:"detail"`
	}
	_ = json.Unmarshal(body, &payload)
	var detail struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
		Type    string `json:"type"`
	}
	if len(payload.Error) > 0 {
		if json.Unmarshal(payload.Error, &detail) != nil {
			_ = json.Unmarshal(payload.Error, &detail.Message)
		}
	}
	msg := detail.Message
	if msg == "" {
		msg = payload.Message
	}
	if msg == "" {
		msg = payload.Detail
	}
	code := ""
	if detail.Code != nil {
		code = fmt.Sprint(detail.Code)
	}
	if code == "" {
		code = detail.Type
	}
	label := "服务请求失败"
	lower := strings.ToLower(msg + " " + code)
	category := ""
	switch {
	case strings.Contains(lower, "only supports /v1/responses") || strings.Contains(lower, "only supports /responses") || (strings.Contains(lower, "responses") && strings.Contains(lower, "chat/completions is disabled")):
		label = "接口协议不兼容，请在渠道设置中将聊天协议改为 Responses"
		category = "protocol-mismatch"
	case strings.Contains(lower, "insufficient_quota"), strings.Contains(lower, "insufficient balance"), strings.Contains(lower, "credit balance"), status == 402:
		label = "账户余额或可用额度不足，请检查服务商账户"
	case status == 401:
		label = "API Key 无效或已过期，请检查密钥"
	case status == 403:
		label = "服务拒绝访问，请检查密钥权限和模型权限"
	case status == 404:
		label = "接口或模型不存在，请检查 Base URL 和模型名"
	case status == 429:
		label = "请求受到限流，请稍后重试"
	case status >= 300 && status < 400:
		label = "接口返回重定向，请填写最终 API 地址"
	case status >= 500:
		label = "服务商暂时不可用，请稍后重试"
	case status == 400 || status == 422:
		label = "服务不接受当前请求，请检查模型、协议及图片支持"
	}
	if status > 0 {
		label += fmt.Sprintf("（HTTP %d）", status)
	}
	if msg = sanitize(msg, key); msg != "" {
		label += "：" + msg
	}
	return &APIError{Status: status, Code: sanitize(code, key), Message: label, Category: category}
}

func normalizeProvider(p string) string { return strings.ToLower(strings.TrimSpace(p)) }

func defaultBase(p string) string {
	switch normalizeProvider(p) {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "cerebras":
		return "https://api.cerebras.ai/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "minimax":
		return "https://api.minimax.io/v1"
	case "ollama":
		return "http://127.0.0.1:11434/v1"
	case "deepgram":
		return "https://api.deepgram.com/v1"
	default:
		return ""
	}
}

func baseURL(ep model.Endpoint) (*url.URL, error) {
	base := strings.TrimSpace(ep.BaseURL)
	if base == "" {
		base = defaultBase(ep.Provider)
	}
	if base == "" {
		return nil, errors.New("请填写服务的 Base URL")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return nil, errors.New("Base URL 必须是有效的 http 或 https 地址，不能包含用户名、密码或片段")
	}
	escaped := strings.TrimRight(u.EscapedPath(), "/")
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = escaped
	return u, nil
}

func endpointURL(ep model.Endpoint, suffix string) (string, error) {
	u, err := baseURL(ep)
	if err != nil {
		return "", err
	}
	// A pasted full endpoint is accepted without adding a duplicate route.
	if strings.HasSuffix(u.Path, suffix) {
		return u.String(), nil
	}
	// Chat and transcription have independently configured URLs. Do not guess
	// by replacing the other protocol's route when it was pasted by mistake.
	if strings.HasSuffix(u.Path, "/chat/completions") || strings.HasSuffix(u.Path, "/responses") || strings.HasSuffix(u.Path, "/audio/transcriptions") || strings.HasSuffix(u.Path, "/messages") {
		return "", errors.New("Base URL 指向另一种接口，请填写 API 根路径或当前功能的完整接口地址")
	}
	if normalizeProvider(ep.Provider) == "azure" {
		if u.Path == "" {
			u.Path = "/openai/v1"
			u.RawPath = ""
		} else if strings.HasSuffix(u.Path, "/openai") {
			appendPath(u, "/v1")
		}
	} else if u.Path == "" {
		u.Path = "/v1"
		u.RawPath = ""
	}
	appendPath(u, suffix)
	return u.String(), nil
}

func appendPath(u *url.URL, suffix string) {
	escaped := u.EscapedPath()
	u.Path += suffix
	u.RawPath = escaped + suffix
}

func geminiURL(ep model.Endpoint, stream bool) (string, error) {
	u, err := baseURL(ep)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(strings.TrimSpace(ep.Model), "models/")
	if name == "" || strings.ContainsAny(name, "/?#") {
		return "", errors.New("请填写有效的 Gemini 模型名")
	}
	method := ":generateContent"
	if stream {
		method = ":streamGenerateContent"
	}
	if i := strings.Index(u.Path, "/models/"); i >= 0 {
		u.Path = u.Path[:i]
	}
	if u.Path == "" {
		u.Path = "/v1beta"
	}
	u.Path += "/models/" + name + method
	q := u.Query()
	q.Del("key") // Credentials belong in headers, never in logged request URLs.
	if stream {
		q.Set("alt", "sse")
	} else {
		q.Del("alt")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func authHeaders(req *http.Request, ep model.Endpoint) {
	if ep.APIKey == "" {
		return
	}
	switch normalizeProvider(ep.Provider) {
	case "anthropic":
		req.Header.Set("x-api-key", ep.APIKey)
	case "gemini":
		req.Header.Set("x-goog-api-key", ep.APIKey)
	case "azure":
		req.Header.Set("api-key", ep.APIKey)
	case "deepgram":
		req.Header.Set("Authorization", "Token "+ep.APIKey)
	default:
		req.Header.Set("Authorization", "Bearer "+ep.APIKey)
	}
}

type activityReader struct {
	io.Reader
	touch func()
}

func (r activityReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	if n > 0 {
		r.touch()
	}
	return n, e
}

// perform closes the body and limits unbounded/inactive responses. No automatic
// POST retries: a retry can duplicate spending and requires an explicit action.
func perform(ctx context.Context, ep model.Endpoint, address, contentType string, data []byte, streaming bool, consume func(io.Reader, string) error, onActivity func()) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	interval := 90 * time.Second
	timeoutError := errors.New("服务连续 90 秒没有返回数据，请重试或切换模型")
	if streaming {
		interval = 25 * time.Second
		timeoutError = errInactive
	}
	// Responses 推理可能较晚才发送首个事件；开始接收后仍按原流式空闲时限保护断流。
	firstResponseInterval := interval
	firstResponseError := timeoutError
	if streaming && ep.ChatProtocol == model.ChatProtocolResponses {
		firstResponseInterval = 60 * time.Second
		firstResponseError = errors.New("服务等待首个响应超过 60 秒，请重试或切换模型")
	}
	firstResponseTimer := time.AfterFunc(firstResponseInterval, func() { cancel(firstResponseError) })
	defer firstResponseTimer.Stop()
	timer := time.AfterFunc(interval, func() { cancel(timeoutError) })
	timer.Stop()
	defer timer.Stop()
	touch := func() {
		firstResponseTimer.Stop()
		timer.Reset(interval)
		if onActivity != nil {
			onActivity()
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(data))
	if err != nil {
		return errors.New("无法创建服务请求，请检查地址配置")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json, text/event-stream")
	authHeaders(req, ep)
	if normalizeProvider(ep.Provider) == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return transportError(ctx, err, ep.APIKey)
	}
	defer resp.Body.Close()
	touch()
	r := activityReader{Reader: io.LimitReader(resp.Body, maxResponseBytes+1), touch: touch}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(r, 64<<10))
		if readErr != nil {
			return transportError(ctx, readErr, ep.APIKey)
		}
		return serviceError(resp.StatusCode, body, ep.APIKey)
	}
	if err := consume(r, resp.Header.Get("Content-Type")); err != nil {
		if ctx.Err() != nil {
			return transportError(ctx, err, ep.APIKey)
		}
		var api *APIError
		if errors.As(err, &api) {
			clean := *api
			clean.Message = sanitize(api.Message, ep.APIKey)
			clean.Code = sanitize(api.Code, ep.APIKey)
			return &clean
		}
		return errors.New(sanitize(err.Error(), ep.APIKey))
	}
	return nil
}

func transportError(ctx context.Context, err error, key string) error {
	if cause := context.Cause(ctx); cause != nil {
		if errors.Is(cause, errInactive) {
			return errInactive
		}
		if errors.Is(cause, context.DeadlineExceeded) {
			return fmt.Errorf("服务请求超时：%w", context.DeadlineExceeded)
		}
		if !errors.Is(cause, context.Canceled) {
			return errors.New(sanitize(cause.Error(), key))
		}
		return fmt.Errorf("请求已取消：%w", context.Canceled)
	}
	// url.Error includes the complete URL and may expose query credentials.
	var uerr *url.Error
	if errors.As(err, &uerr) {
		err = uerr.Err
	}
	return fmt.Errorf("无法连接服务：%s", sanitize(err.Error(), key))
}

func readJSON(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return errors.New("服务返回内容过大")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return errors.New("服务返回了无法解析的 JSON，请检查 API 地址和协议")
	}
	return nil
}

package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"cue-go-remix/internal/model"
)

type Request struct{ System, Text, Image string }

// Chat executes a single request. Image, when present, is a base64 image data URL.
// onToken receives answer text only; reasoning updates merely trigger onActivity.
func Chat(ctx context.Context, ep model.Endpoint, req Request, onToken func(string), onActivity func()) (string, error) {
	ep.Provider = normalizeProvider(ep.Provider)
	if strings.TrimSpace(ep.Model) == "" {
		return "", errors.New("请先填写聊天模型名")
	}
	if strings.TrimSpace(ep.APIKey) == "" && ep.Provider != "custom" && ep.Provider != "ollama" {
		return "", errors.New("请先填写聊天服务的 API Key")
	}
	var address string
	var payload any
	var err error
	switch ep.Provider {
	case "anthropic":
		address, err = endpointURL(ep, "/messages")
		content := []any{map[string]any{"type": "text", "text": req.Text}}
		if req.Image != "" {
			mime, data, e := splitImage(req.Image)
			if e != nil {
				return "", e
			}
			content = append(content, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mime, "data": data}})
		}
		payload = map[string]any{"model": ep.Model, "max_tokens": 4096, "system": req.System, "stream": ep.Stream, "messages": []any{map[string]any{"role": "user", "content": content}}}
	case "gemini":
		address, err = geminiURL(ep, ep.Stream)
		parts := []any{map[string]any{"text": req.Text}}
		if req.Image != "" {
			mime, data, e := splitImage(req.Image)
			if e != nil {
				return "", e
			}
			parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": mime, "data": data}})
		}
		payload = map[string]any{"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": req.System}}}, "contents": []any{map[string]any{"role": "user", "parts": parts}}}
	case "openai", "custom", "azure", "cerebras", "deepseek", "groq", "minimax", "ollama":
		address, err = endpointURL(ep, "/chat/completions")
		var content any = req.Text
		if req.Image != "" {
			if _, _, e := splitImage(req.Image); e != nil {
				return "", e
			}
			content = []any{map[string]any{"type": "text", "text": req.Text}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": req.Image}}}
		}
		payload = map[string]any{"model": ep.Model, "stream": ep.Stream, "messages": []any{map[string]any{"role": "system", "content": req.System}, map[string]any{"role": "user", "content": content}}}
	default:
		return "", fmt.Errorf("不支持的聊天服务：%s", ep.Provider)
	}
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", errors.New("无法编码聊天请求")
	}
	var answer strings.Builder
	emit := func(s string) error {
		if s == "" {
			return nil
		}
		if answer.Len()+len(s) > maxResponseBytes {
			return errors.New("模型回答过长，已停止接收")
		}
		answer.WriteString(s)
		if onToken != nil {
			onToken(s)
		}
		return nil
	}
	err = perform(ctx, ep, address, "application/json", body, ep.Stream, func(r io.Reader, contentType string) error {
		reader := bufio.NewReader(r)
		// Ignore harmless leading whitespace from JSON-producing gateways.
		for {
			prefix, peekErr := reader.Peek(1)
			if peekErr != nil || !strings.ContainsRune(" \t\r\n", rune(prefix[0])) {
				break
			}
			_, _ = reader.ReadByte()
		}
		// Some gateways ignore stream:true and return JSON. Accept that explicit
		// format rather than pretending no answer was returned.
		first, _ := reader.Peek(1)
		isSSE := strings.Contains(strings.ToLower(contentType), "text/event-stream")
		if !isSSE && ep.Stream && len(first) > 0 && first[0] != '{' && first[0] != '[' && !strings.Contains(contentType, "json") {
			isSSE = true
		}
		if isSSE {
			ended := false
			err := readSSE(reader, func(event, data string) (bool, error) {
				if data == "[DONE]" {
					ended = true
					return true, nil
				}
				var value map[string]any
				if err := json.Unmarshal([]byte(data), &value); err != nil {
					return false, errors.New("服务返回了无法解析的流式数据")
				}
				if value["error"] != nil || event == "error" {
					return false, serviceError(0, []byte(data), ep.APIKey)
				}
				text, done, e := extractAnswer(ep.Provider, value, true)
				if e != nil {
					return false, e
				}
				if e = emit(text); e != nil {
					return false, e
				}
				if done {
					ended = true
				}
				return false, nil
			})
			if err != nil {
				return err
			}
			if !ended {
				return errors.New("服务的流式连接提前结束，回答可能不完整")
			}
			return nil
		}
		var value map[string]any
		if err := readJSON(reader, &value); err != nil {
			return err
		}
		if value["error"] != nil {
			raw, _ := json.Marshal(value)
			return serviceError(0, raw, ep.APIKey)
		}
		text, _, err := extractAnswer(ep.Provider, value, false)
		if err != nil {
			return err
		}
		return emit(text)
	}, onActivity)
	if err != nil {
		return answer.String(), err
	}
	if strings.TrimSpace(answer.String()) == "" {
		return "", errors.New("模型没有返回可显示的回答，请检查模型是否支持该请求")
	}
	return answer.String(), nil
}

func splitImage(image string) (string, string, error) {
	prefix, data, ok := strings.Cut(image, ",")
	if !ok || !strings.HasPrefix(prefix, "data:image/") || !strings.HasSuffix(prefix, ";base64") || data == "" {
		return "", "", errors.New("截图格式无效，需要 base64 图片")
	}
	return strings.TrimSuffix(strings.TrimPrefix(prefix, "data:"), ";base64"), data, nil
}

func readSSE(r io.Reader, handle func(event, data string) (bool, error)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var event string
	var lines []string
	dispatch := func() (bool, error) {
		if len(lines) == 0 {
			event = ""
			return false, nil
		}
		done, err := handle(event, strings.Join(lines, "\n"))
		lines = nil
		event = ""
		return done, err
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			done, err := dispatch()
			if err != nil || done {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			lines = append(lines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流式响应失败：%w", err)
	}
	_, err := dispatch()
	return err
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any            { s, _ := v.([]any); return s }
func stringValue(v any) string    { s, _ := v.(string); return s }
func boolValue(v any) bool        { b, _ := v.(bool); return b }

func contentText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	var out strings.Builder
	for _, part := range list(v) {
		m := object(part)
		if stringValue(m["type"]) == "text" || stringValue(m["type"]) == "output_text" {
			out.WriteString(stringValue(m["text"]))
		}
	}
	return out.String()
}

func extractAnswer(provider string, v map[string]any, stream bool) (string, bool, error) {
	switch provider {
	case "anthropic":
		if !stream {
			return contentText(v["content"]), true, nil
		}
		switch stringValue(v["type"]) {
		case "content_block_delta":
			return stringValue(object(v["delta"])["text"]), false, nil
		case "content_block_start":
			return stringValue(object(v["content_block"])["text"]), false, nil
		case "message_stop":
			return "", true, nil
		case "message_delta":
			reason := stringValue(object(v["delta"])["stop_reason"])
			if reason == "refusal" {
				return "", true, errors.New("模型拒绝了该请求")
			}
		}
		return "", false, nil
	case "gemini":
		if block := stringValue(object(v["promptFeedback"])["blockReason"]); block != "" {
			return "", true, fmt.Errorf("模型未接受该请求（%s）", block)
		}
		candidates := list(v["candidates"])
		if len(candidates) == 0 {
			return "", false, nil
		}
		c := object(candidates[0])
		var out strings.Builder
		for _, p := range list(object(c["content"])["parts"]) {
			part := object(p)
			if !boolValue(part["thought"]) {
				out.WriteString(stringValue(part["text"]))
				out.WriteString(stringValue(object(part["audioTranscription"])["text"]))
			}
		}
		reason := stringValue(c["finishReason"])
		if reason != "" && reason != "STOP" && reason != "MAX_TOKENS" {
			return out.String(), true, fmt.Errorf("模型停止了回答（%s）", reason)
		}
		return out.String(), reason != "", nil
	default:
		choices := list(v["choices"])
		if len(choices) == 0 {
			return "", false, nil
		}
		c := object(choices[0])
		message := object(c["message"])
		if stream {
			message = object(c["delta"])
		}
		reason := stringValue(c["finish_reason"])
		if reason == "content_filter" {
			return "", true, errors.New("模型服务拦截了该回答")
		}
		if refusal := stringValue(message["refusal"]); refusal != "" {
			return "", true, errors.New("模型拒绝了该请求")
		}
		return contentText(message["content"]), reason != "", nil
	}
}

package provider

import (
	"encoding/json"
	"errors"
	"io"

	"cue-go-remix/internal/model"
)

// responsesPayload 将系统指令、文本及可选截图转换为 Responses 请求，不创建远端会话或保存响应。
func responsesPayload(ep model.Endpoint, req Request) (any, error) {
	content := []any{map[string]any{"type": "input_text", "text": req.Text}}
	if req.Image != "" {
		if _, _, err := splitImage(req.Image); err != nil {
			return nil, err
		}
		content = append(content, map[string]any{"type": "input_image", "image_url": req.Image})
	}
	return map[string]any{
		"model": ep.Model, "stream": ep.Stream, "store": false,
		"instructions": req.System,
		"input":        []any{map[string]any{"role": "user", "content": content}},
	}, nil
}

// responsesText 提取助手消息中的正文并检查终态；忽略推理和工具数据，失败或未完成不能当作成功。
func responsesText(response map[string]any, key string) (string, error) {
	if response["error"] != nil {
		raw, _ := json.Marshal(response)
		return "", serviceError(0, raw, key)
	}
	var text string
	for _, raw := range list(response["output"]) {
		item := object(raw)
		if stringValue(item["type"]) != "message" || (item["role"] != nil && stringValue(item["role"]) != "assistant") {
			continue
		}
		for _, rawPart := range list(item["content"]) {
			part := object(rawPart)
			switch stringValue(part["type"]) {
			case "output_text":
				text += stringValue(part["text"])
			case "refusal":
				return text, errors.New("模型拒绝回答本次请求")
			}
		}
	}
	switch stringValue(response["status"]) {
	case "", "completed":
		return text, nil
	case "incomplete":
		return text, errors.New("服务返回了未完成的回答，请检查输出额度或模型限制后重试")
	default:
		return text, errors.New("服务未成功完成回答，请重试或切换模型")
	}
}

// readResponses 解析普通 JSON 或 Responses SSE；流式终态快照不重复发送已输出的正文。
// emit 负责回答长度限制；断流、失败和取消由调用方保留部分回答并报告错误。
func readResponses(reader io.Reader, streaming bool, key string, emit func(string) error) error {
	if !streaming {
		var response map[string]any
		if err := readJSON(reader, &response); err != nil {
			return err
		}
		text, err := responsesText(response, key)
		if emitErr := emit(text); emitErr != nil {
			return emitErr
		}
		return err
	}
	emitted, completed := false, false
	err := readSSE(reader, func(event, data string) (bool, error) {
		if data == "[DONE]" {
			return true, nil
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(data), &value); err != nil {
			return false, errors.New("服务返回了无法解析的 Responses 流式数据")
		}
		kind := stringValue(value["type"])
		if kind == "" {
			kind = event
		}
		if value["error"] != nil || kind == "error" {
			return false, serviceError(0, []byte(data), key)
		}
		switch kind {
		case "response.output_text.delta":
			text := stringValue(value["delta"])
			emitted = emitted || text != ""
			return false, emit(text)
		case "response.completed", "response.failed", "response.incomplete":
			text, err := responsesText(object(value["response"]), key)
			if !emitted {
				if emitErr := emit(text); emitErr != nil {
					return false, emitErr
				}
			}
			if err != nil {
				return false, err
			}
			if kind != "response.completed" {
				return false, errors.New("服务未成功完成回答，请重试或切换模型")
			}
			completed = true
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if !completed {
		return errors.New("服务的 Responses 流式连接提前结束，回答可能不完整")
	}
	return nil
}

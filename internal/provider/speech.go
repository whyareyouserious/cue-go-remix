package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"strings"

	"cue-go-remix/internal/model"
)

// WAV encodes mono signed 16-bit PCM at 16 kHz without a temporary file.
func WAV(pcm []int16) []byte {
	data := make([]byte, 44+len(pcm)*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 16000)
	binary.LittleEndian.PutUint32(data[28:], 32000)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(pcm)*2))
	for i, sample := range pcm {
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(sample))
	}
	return data
}

// Transcribe sends one bounded utterance to the explicitly selected provider.
// Silence may return an empty string; no other provider is used as a fallback.
func Transcribe(ctx context.Context, ep model.Endpoint, pcm []int16, language string) (string, error) {
	if len(pcm) == 0 {
		return "", nil
	}
	if len(pcm) > 16000*120 {
		return "", errors.New("单段音频超过两分钟，请分段转写")
	}
	ep.Provider = normalizeProvider(ep.Provider)
	if ep.APIKey == "" && ep.Provider != "custom" {
		return "", errors.New("请先填写语音服务的 API Key")
	}
	wav := WAV(pcm)
	var address, contentType string
	var body []byte
	var err error
	switch ep.Provider {
	case "openai", "custom", "groq":
		if ep.Model == "" {
			if ep.Provider == "groq" {
				ep.Model = "whisper-large-v3-turbo"
			} else {
				ep.Model = "whisper-1"
			}
		}
		address, err = endpointURL(ep, "/audio/transcriptions")
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		_ = writer.WriteField("model", ep.Model)
		_ = writer.WriteField("response_format", "json")
		if language != "" && language != "auto" {
			_ = writer.WriteField("language", language)
		}
		file, e := writer.CreateFormFile("file", "audio.wav")
		if e != nil {
			return "", e
		}
		_, _ = file.Write(wav)
		_ = writer.Close()
		body = b.Bytes()
		contentType = writer.FormDataContentType()
	case "gemini":
		if ep.Model == "" {
			return "", errors.New("请填写 Gemini 转写模型名")
		}
		address, err = geminiURL(ep, false)
		prompt := "请逐字转写音频，只返回听到的内容，不添加解释；如果没有清晰语音则返回空字符串。"
		if language != "" && language != "auto" {
			prompt += "语音语言代码：" + language + "。"
		}
		body, _ = json.Marshal(map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": prompt}, map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": base64.StdEncoding.EncodeToString(wav)}}}}}})
		contentType = "application/json"
	case "deepgram":
		address, err = endpointURL(ep, "/listen")
		if err == nil {
			u, _ := baseURL(model.Endpoint{BaseURL: address})
			q := u.Query()
			if ep.Model == "" {
				ep.Model = "nova-3"
			}
			q.Set("model", ep.Model)
			q.Set("smart_format", "true")
			if language != "" && language != "auto" {
				q.Set("language", language)
			} else {
				q.Set("detect_language", "true")
			}
			u.RawQuery = q.Encode()
			address = u.String()
		}
		contentType = "audio/wav"
		body = wav
	default:
		return "", errors.New("当前服务不支持云端语音转写，请选择 OpenAI、Groq、Gemini、Deepgram 或自定义兼容服务")
	}
	if err != nil {
		return "", err
	}
	var transcript string
	err = perform(ctx, ep, address, contentType, body, false, func(r io.Reader, _ string) error {
		var v map[string]any
		if err := readJSON(r, &v); err != nil {
			return err
		}
		if v["error"] != nil {
			raw, _ := json.Marshal(v)
			return serviceError(0, raw, ep.APIKey)
		}
		switch ep.Provider {
		case "gemini":
			var e error
			transcript, _, e = extractAnswer("gemini", v, false)
			return e
		case "deepgram":
			channels := list(object(v["results"])["channels"])
			if len(channels) > 0 {
				alts := list(object(channels[0])["alternatives"])
				if len(alts) > 0 {
					transcript = stringValue(object(alts[0])["transcript"])
				}
			}
		default:
			var ok bool
			transcript, ok = v["text"].(string)
			if !ok {
				return errors.New("语音服务返回内容缺少 text 字段，请检查转写协议与地址")
			}
		}
		return nil
	}, nil)
	return strings.TrimSpace(transcript), err
}

package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/provider"
	"cue-go-remix/internal/sherpa"
	"cue-go-remix/internal/whisper"
)

// CheckEndpoint 探测聊天或语音服务连通性；绝不持久化修改设置，语音绝不自动录音。
func (a *Runtime) CheckEndpoint(kind string, ep model.Endpoint) (model.EndpointCheckResult, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "chat" && kind != "speech" {
		return model.EndpointCheckResult{Success: false, Message: "未知的服务类型，仅支持 chat 或 speech"}, errors.New("无效的服务类型")
	}

	currentSettings := a.store.Settings()
	var currentEp model.Endpoint
	if kind == "chat" {
		currentEp = currentSettings.Chat
	} else {
		currentEp = currentSettings.Speech
	}

	// 密钥继承：若表单中留空且未勾选清除，且服务商与地址未变，继承旧密钥
	if ep.ClearKey {
		ep.APIKey = ""
	} else if ep.APIKey == "" && ep.Provider == currentEp.Provider && ep.BaseURL == currentEp.BaseURL {
		ep.APIKey = currentEp.APIKey
	}
	return a.checkResolvedEndpoint(kind, ep)
}

// checkResolvedEndpoint 只探测已完成凭据隔离的端点，不从当前激活账号再补密钥。
func (a *Runtime) checkResolvedEndpoint(kind string, ep model.Endpoint) (model.EndpointCheckResult, error) {
	ep.Provider = strings.ToLower(strings.TrimSpace(ep.Provider))
	ep.ChatProtocol = strings.ToLower(strings.TrimSpace(ep.ChatProtocol))
	if err := ep.ValidateChatProtocol(); err != nil {
		return model.EndpointCheckResult{Message: err.Error()}, nil
	}
	if kind == "speech" && ep.ChatProtocol != "" {
		return model.EndpointCheckResult{Message: "语音渠道不能设置聊天协议"}, nil
	}
	ep.BaseURL = strings.TrimSpace(ep.BaseURL)
	if ep.Provider != "local" && ep.Provider != "sherpa" && ep.BaseURL != "" {
		u, err := url.Parse(ep.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return model.EndpointCheckResult{Message: "服务地址必须为有效的 HTTP/HTTPS 地址，不能包含账号密码"}, nil
		}
	}
	currentSettings := a.store.Settings()
	a.trace("event=endpoint-check-start kind=%s provider=%s protocol=%s stream=%t", kind, logging.Provider(ep.Provider), logging.ChatProtocol(ep), ep.Stream)
	start := time.Now()

	if kind == "chat" {
		if strings.TrimSpace(ep.Model) == "" {
			return model.EndpointCheckResult{Success: false, Message: "请先填写快速模型名称"}, nil
		}
		if strings.TrimSpace(ep.APIKey) == "" && ep.Provider != "custom" && ep.Provider != "ollama" {
			return model.EndpointCheckResult{Success: false, Message: "请先填写 API Key"}, nil
		}

		// 推理模型的首个正文可能晚于 10 秒；总检测时限与接收活动超时共同约束等待。
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()

		req := provider.Request{
			System: "Respond with ok",
			Text:   "ping",
		}
		// 与正式问答使用相同协议和流式模式，兼容只开放 Responses SSE 的网关。
		_, err := provider.Chat(ctx, ep, req, func(string) {}, func() {})
		latency := time.Since(start).Milliseconds()
		if err != nil {
			a.trace("event=endpoint-check-failed kind=chat protocol=%s reason=%s category=%s", logging.ChatProtocol(ep), logging.Error(err), logging.ErrorCategory(err))
			msg := friendlyError(err)
			return model.EndpointCheckResult{Success: false, Latency: latency, Message: msg}, nil
		}
		a.trace("event=endpoint-check-success kind=chat protocol=%s latency=%d", logging.ChatProtocol(ep), latency)
		return model.EndpointCheckResult{Success: true, Latency: latency, Message: fmt.Sprintf("连接成功 (%dms)", latency)}, nil
	}

	if kind == "speech" {
		// 本地服务：必须检查选定模型与运行时完整性，绝非仅目录存在就报就绪
		if ep.Provider == "sherpa" {
			dir, err := sherpa.Verify()
			if err != nil {
				return model.EndpointCheckResult{Success: false, Message: "Sherpa 运行时或模型校验失败: " + err.Error()}, nil
			}
			return model.EndpointCheckResult{Success: true, Latency: 0, Message: fmt.Sprintf("本地 Sherpa 运行库与模型校验就绪 (%s)", dir)}, nil
		}

		if ep.Provider == "local" {
			// 1. 显式使用草稿中传入的 WhisperPath；留空则直接按默认内置查找规则查找，绝不回填失效旧配置
			targetPath := strings.TrimSpace(ep.WhisperPath)
			exe, err := whisper.RuntimePath(targetPath)
			if err != nil {
				return model.EndpointCheckResult{Success: false, Message: "未找到可用 whisper-server 可执行文件: " + err.Error()}, nil
			}
			// 2. 优先使用草稿中传入的 WhisperModel，留空使用默认 "base"
			selectedModel := strings.TrimSpace(ep.WhisperModel)
			if selectedModel == "" {
				selectedModel = currentSettings.WhisperModel
			}
			if selectedModel == "" {
				selectedModel = "base"
			}
			if err := a.models.Verify(selectedModel); err != nil {
				return model.EndpointCheckResult{Success: false, Message: fmt.Sprintf("选定模型 [%s] 校验失败或未下载: %v", selectedModel, err)}, nil
			}
			return model.EndpointCheckResult{Success: true, Latency: 0, Message: fmt.Sprintf("本地 Whisper 运行时与选定模型 [%s] 校验就绪 (%s)", selectedModel, exe)}, nil
		}

		// 云端语音服务：绝不启动麦克风硬件录音！
		if strings.TrimSpace(ep.APIKey) == "" && ep.Provider != "custom" {
			return model.EndpointCheckResult{Success: false, Message: "请先填写语音服务的 API Key"}, nil
		}

		ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
		defer cancel()

		if ep.Stream {
			// 用户开启了实时转写：执行实时流式 WebSocket 握手连通测试
			if ep.Provider != "openai" && ep.Provider != "deepgram" && ep.Provider != "gemini" {
				return model.EndpointCheckResult{Success: false, Message: fmt.Sprintf("提供商 %s 不支持实时流式转写，请关闭实时开关后使用分段转写", ep.Provider)}, nil
			}
			stream, err := provider.OpenStream(ctx, ep, "zh", func(string, bool) {}, func(error) {})
			latency := time.Since(start).Milliseconds()
			if err != nil {
				a.trace("event=endpoint-check-failed kind=speech-stream reason=%s", logging.Error(err))
				msg := friendlyError(err)
				return model.EndpointCheckResult{Success: false, Latency: latency, Message: "实时流式握手失败: " + msg}, nil
			}
			_ = stream.Close()
			a.trace("event=endpoint-check-success kind=speech-stream latency=%d", latency)
			return model.EndpointCheckResult{Success: true, Latency: latency, Message: fmt.Sprintf("实时流式握手连接成功 (%dms)", latency)}, nil
		}

		// 用户未开启实时转写：执行分段 HTTP 接口连通测试（内存 0.5s 静音 PCM，不触碰录音设备）
		silencePCM := make([]int16, 8000)
		_, err := provider.Transcribe(ctx, ep, silencePCM, "zh")
		latency := time.Since(start).Milliseconds()
		if err != nil {
			a.trace("event=endpoint-check-failed kind=speech-batch reason=%s", logging.Error(err))
			msg := friendlyError(err)
			return model.EndpointCheckResult{Success: false, Latency: latency, Message: "分段转写接口连接失败: " + msg}, nil
		}
		a.trace("event=endpoint-check-success kind=speech-batch latency=%d", latency)
		return model.EndpointCheckResult{Success: true, Latency: latency, Message: fmt.Sprintf("分段转写服务连接正常 (%dms)", latency)}, nil
	}

	return model.EndpointCheckResult{Success: false, Message: "未知的服务类型: " + kind}, nil
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	// provider 已按状态和业务原因生成脱敏提示，不能再次用通用 403 文案覆盖协议错误。
	var api *provider.APIError
	if errors.As(err, &api) {
		return api.Error()
	}
	s := err.Error()
	if strings.Contains(s, "context deadline exceeded") || strings.Contains(s, "timeout") {
		return "连接超时，请检查服务地址与网络代理"
	}
	if strings.Contains(s, "401") {
		return "API Key 无效或未授权 (HTTP 401)"
	}
	if strings.Contains(s, "403") {
		return "访问被拒绝或无模型权限 (HTTP 403)"
	}
	if strings.Contains(s, "404") {
		return "服务地址错误或模型未找到 (HTTP 404)"
	}
	if strings.Contains(s, "connection refused") {
		return "服务连接被拒绝，请确认服务已启动且地址正确"
	}
	return s
}

package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
)

// RecordUIEvent 的所有参数均受白名单约束，不能用来写入任意用户输入。
func (a *Runtime) RecordUIEvent(event, value string) {
	allowed := false
	switch event {
	case "settings-open", "settings-save":
		switch value {
		case "chat", "speech", "style", "appearance", "shortcuts", "diagnostics", "models", "access":
			allowed = true
		}
	case "provider-select":
		allowed = logging.Provider(value) != "unknown"
	case "key-focus":
		allowed = value == "chat" || value == "speech"
	case "panel-toggle":
		allowed = value == "collapsed" || value == "expanded"
	case "sidebar-toggle":
		allowed = value == "open" || value == "closed"
	}
	if allowed {
		a.trace("event=ui action=%s value=%s", event, value)
	}
}

// OpenLogsDir 在系统文件管理器中打开日志目录。
func (a *Runtime) OpenLogsDir() error {
	dir := filepath.Join(a.store.Dir(), "logs")
	_ = os.MkdirAll(dir, 0700)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

// DiagnosticsExport 白名单脱敏诊断报告；严禁携带密钥、转录正文或个人资料。
type DiagnosticsExport struct {
	AppVersion        string              `json:"appVersion"`
	Platform          string              `json:"platform"`
	Arch              string              `json:"arch"`
	GoVersion         string              `json:"goVersion"`
	UptimeSeconds     int64               `json:"uptimeSeconds"`
	ContentProtection bool                `json:"contentProtection"`
	MeetingAudio      bool                `json:"meetingAudio"`
	ChatProvider      string              `json:"chatProvider"`
	ChatModel         string              `json:"chatModel"`
	SpeechProvider    string              `json:"speechProvider"`
	SpeechModel       string              `json:"speechModel"`
	RecentErrors      []model.RecentError `json:"recentErrors"`
	RecentLogEvents   []string            `json:"recentLogEvents"`
	ExportedAt        int64               `json:"exportedAt"`
}

var (
	dateRegex  = regexp.MustCompile(`^20\d{2}/(0[1-9]|1[0-2])/(0[1-9]|[12]\d|3[01])$`)
	timeRegex  = regexp.MustCompile(`^[0-2]\d:[0-5]\d:[0-5]\d(\.\d+)?$`)
	httpCodeRe = regexp.MustCompile(`^http-\d{3}$`)
	digitsRe   = regexp.MustCompile(`^\d{1,7}$`)
)

var allowedDiagnosticEvents = map[string]bool{
	"app-launch":             true,
	"app-ready":              true,
	"settings-save-start":    true,
	"settings-save-done":     true,
	"settings-save-failed":   true,
	"endpoint-check-start":   true,
	"endpoint-check-success": true,
	"endpoint-check-failed":  true,
	"ui":                     true,
	"operation-error":        true,
	"answer-start":           true,
	"answer-done":            true,
	"answer-failed":          true,
	"listening-start":        true,
	"listening-stop":         true,
	"notes-start":            true,
	"notes-done":             true,
	"shutdown-start":         true,
	"shutdown-done":          true,
	"quit-request":           true,
}

var allowedActions = map[string]bool{
	"settings-open":   true,
	"settings-save":   true,
	"provider-select": true,
	"key-focus":       true,
	"panel-toggle":    true,
	"sidebar-toggle":  true,
}

var allowedUIValues = map[string]bool{
	"chat": true, "speech": true, "style": true, "appearance": true,
	"shortcuts": true, "diagnostics": true, "models": true, "access": true,
	"custom": true, "openai": true, "anthropic": true, "gemini": true,
	"azure": true, "deepseek": true, "groq": true, "cerebras": true,
	"minimax": true, "ollama": true, "deepgram": true, "local": true, "sherpa": true,
	"collapsed": true, "expanded": true, "open": true, "closed": true,
}

var allowedStages = map[string]bool{
	"provider": true, "storage": true, "audio": true, "link": true, "validation": true,
}

var allowedReasons = map[string]bool{
	"validation-or-storage": true, "listening": true, "cancelled": true,
	"timeout": true, "network": true, "other": true,
}

var allowedKinds = map[string]bool{
	"chat": true, "speech": true, "speech-stream": true, "speech-batch": true,
	"cancelled": true, "timeout": true, "network": true, "other": true,
}

// sanitizeDiagnosticLogLine 严格校验并清洗单行日志：
// 1. 事件名称必须严格属于 allowedDiagnosticEvents（否则整行丢弃）；
// 2. 字段只保留符合白名单枚举或格式要求的 key=value，未知字段（如 id=xxx）直接丢弃该字段，绝不抹掉整个正常失败事件；
// 3. 严格解析真实时间戳，格式不合规的时间丢弃。
func sanitizeDiagnosticLogLine(raw string) (string, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return "", false
	}

	// 先通览判断本行是否具有合法的白名单 event
	var eventName string
	for _, token := range fields {
		if strings.HasPrefix(token, "event=") {
			val := strings.TrimPrefix(token, "event=")
			if allowedDiagnosticEvents[val] {
				eventName = val
			}
			break
		}
	}
	if eventName == "" {
		return "", false // 非白名单事件行直接丢弃
	}

	var parsedDate, parsedTime string
	var pairs []string

	for i := 0; i < len(fields); i++ {
		token := fields[i]

		// 忽略 pid= 前缀
		if strings.HasPrefix(token, "pid=") {
			continue
		}

		// 严格校验真实日期：YYYY/MM/DD
		if parsedDate == "" && dateRegex.MatchString(token) {
			parsedDate = token
			continue
		}

		// 严格校验真实时间：HH:MM:SS 或 HH:MM:SS.mmmmmm
		if parsedDate != "" && parsedTime == "" && timeRegex.MatchString(token) {
			parsedTime = token
			continue
		}

		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue // 非 key=value 的杂项 token 直接丢弃，不破坏整行
		}

		k, v := parts[0], parts[1]
		switch k {
		case "event":
			pairs = append(pairs, "event="+eventName)

		case "platform":
			if v == "darwin" || v == "windows" || v == "linux" {
				pairs = append(pairs, "platform="+v)
			}

		case "action":
			if allowedActions[v] {
				pairs = append(pairs, "action="+v)
			}

		case "value":
			if allowedUIValues[v] {
				pairs = append(pairs, "value="+v)
			}

		case "kind":
			if allowedKinds[v] || httpCodeRe.MatchString(v) {
				pairs = append(pairs, "kind="+v)
			}

		case "provider":
			p := logging.Provider(v)
			if p != "unknown" {
				pairs = append(pairs, "provider="+p)
			}

		case "protocol":
			if v == "chat-completions" || v == "responses" || v == "native" || v == "unknown" {
				pairs = append(pairs, "protocol="+v)
			}

		case "category":
			if v == "protocol-mismatch" || allowedKinds[v] || httpCodeRe.MatchString(v) {
				pairs = append(pairs, "category="+v)
			}

		case "stream", "cancelled":
			if v == "true" || v == "false" {
				pairs = append(pairs, k+"="+v)
			}

		case "reason":
			if allowedReasons[v] || httpCodeRe.MatchString(v) {
				pairs = append(pairs, "reason="+v)
			}

		case "stage":
			if allowedStages[v] {
				pairs = append(pairs, "stage="+v)
			}

		case "latency", "elapsed_ms":
			if digitsRe.MatchString(v) {
				pairs = append(pairs, k+"="+v)
			}

		default:
			// 未知字段（例如 id=xxx 或其他字段）直接丢弃该字段，不影响整行事件！
			continue
		}
	}

	if len(pairs) == 0 {
		return "", false
	}

	cleanLine := strings.Join(pairs, " ")
	if parsedDate != "" && parsedTime != "" {
		cleanLine = fmt.Sprintf("%s %s %s", parsedDate, parsedTime, cleanLine)
	}
	return cleanLine, true
}

// ExportDiagnostics 导出格式化后的白名单脱敏诊断 JSON 字符串。
func (a *Runtime) ExportDiagnostics() (string, error) {
	s := a.store.Settings()
	logsPath := filepath.Join(a.store.Dir(), "logs", "cue.log")

	var logLines []string
	if file, err := os.Open(logsPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			raw := scanner.Text()
			if clean, ok := sanitizeDiagnosticLogLine(raw); ok {
				logLines = append(logLines, clean)
			}
		}
		if len(logLines) > 100 {
			logLines = logLines[len(logLines)-100:]
		}
	}
	if len(logLines) == 0 {
		logLines = []string{"(无近期脱敏日志事件)"}
	}

	report := DiagnosticsExport{
		AppVersion:        "0.1.0",
		Platform:          runtime.GOOS,
		Arch:              runtime.GOARCH,
		GoVersion:         runtime.Version(),
		UptimeSeconds:     a.UptimeSeconds(),
		ContentProtection: s.ContentProtection,
		MeetingAudio:      s.MeetingAudio,
		ChatProvider:      logging.Provider(s.Chat.Provider),
		ChatModel:         s.Chat.Model,
		SpeechProvider:    logging.Provider(s.Speech.Provider),
		SpeechModel:       s.Speech.Model,
		RecentErrors:      a.GetRecentErrors(),
		RecentLogEvents:   logLines,
		ExportedAt:        time.Now().UnixMilli(),
	}

	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

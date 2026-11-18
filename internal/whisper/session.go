package whisper

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
)

type Session struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	done     chan struct{}
	endpoint string
	mu       sync.Mutex
	once     sync.Once
}

func RuntimePath(configured string) (string, error) {
	if configured != "" {
		p, e := filepath.Abs(configured)
		if e != nil {
			return "", e
		}
		if s, e := os.Stat(p); e != nil || s.IsDir() {
			return "", errors.New("Whisper 可执行文件路径无效")
		}
		return p, nil
	}
	name := "whisper-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	for _, p := range []string{filepath.Join(dir, "whisper", name), filepath.Join(dir, "..", "Resources", "whisper", name), filepath.Join("build", "bin", "whisper-"+runtime.GOOS+"-"+runtime.GOARCH, name), filepath.Join("build", "bin", "whisper", name)} {
		if s, e := os.Stat(p); e == nil && !s.IsDir() {
			return filepath.Abs(p)
		}
	}
	if p, e := exec.LookPath(name); e == nil {
		return p, nil
	}
	return "", errors.New("未找到 whisper-server，请运行运行时准备脚本或在音频设置中指定路径")
}
func Start(ctx context.Context, settings model.Settings, modelPath string) (*Session, error) {
	exe, e := RuntimePath(settings.WhisperPath)
	if e != nil {
		return nil, e
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	base := "/cue-" + repository.ID()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, base)
	lang := settings.Language
	if lang == "" {
		lang = "auto"
	}
	args := []string{"--model", modelPath, "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--request-path", base, "--language", lang, "--no-timestamps", "--suppress-nst"}
	if settings.WhisperThreads > 0 {
		args = append(args, "--threads", strconv.Itoa(settings.WhisperThreads))
	}
	procctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procctx, exe, args...)
	cmd.Dir = filepath.Dir(exe)
	configureProcess(cmd)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		cancel()
		return nil, e
	}
	s := &Session{cmd: cmd, cancel: cancel, done: make(chan struct{}), endpoint: url + "/inference"}
	go func() { _ = cmd.Wait(); close(s.done) }()
	deadline := time.NewTimer(3 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	client := &http.Client{Timeout: time.Second}
	for {
		select {
		case <-ctx.Done():
			s.Close()
			return nil, ctx.Err()
		case <-s.done:
			cancel()
			return nil, errors.New("Whisper 进程启动失败，请确认运行时版本为 1.9.1 且模型/内存可用")
		case <-deadline.C:
			s.Close()
			return nil, errors.New("本地模型加载超时")
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", url+"/health", nil)
			res, err := client.Do(req)
			if err == nil {
				res.Body.Close()
				if res.StatusCode == 200 {
					return s, nil
				}
			}
		}
	}
}
func (s *Session) Transcribe(ctx context.Context, pcm []int16) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("response_format", "json")
	_ = w.WriteField("temperature", "0.0")
	f, e := w.CreateFormFile("file", "speech.wav")
	if e != nil {
		return "", e
	}
	_, _ = f.Write(WAV(pcm))
	_ = w.Close()
	req, e := http.NewRequestWithContext(ctx, "POST", s.endpoint, &body)
	if e != nil {
		return "", e
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("本地 Whisper 返回 HTTP %d", res.StatusCode)
	}
	var result struct {
		Text string `json:"text"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result); e != nil {
		return "", e
	}
	return strings.TrimSpace(result.Text), nil
}
func (s *Session) Close() { s.once.Do(func() { s.cancel(); <-s.done }) }
func WAV(pcm []int16) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+2*len(pcm)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(16000))
	_ = binary.Write(&b, binary.LittleEndian, uint32(32000))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pcm)*2))
	_ = binary.Write(&b, binary.LittleEndian, pcm)
	return b.Bytes()
}

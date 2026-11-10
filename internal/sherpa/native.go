// Package sherpa runs the pinned local Chinese streaming recognizer.
package sherpa

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const Version = "2.0.1"

//go:embed artifacts.json
var artifactsJSON []byte

func verifyModel(dir string) error {
	var manifest struct {
		Model struct {
			Files []struct {
				Filename string
				Bytes    int64
				SHA256   string
			}
		}
	}
	if err := json.Unmarshal(artifactsJSON, &manifest); err != nil {
		return err
	}
	for _, file := range manifest.Model.Files {
		f, err := os.Open(filepath.Join(dir, "model", file.Filename))
		if err != nil {
			return fmt.Errorf("缺少中文流式模型 %s，请重新准备完整模型包", file.Filename)
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil || n != file.Bytes || hex.EncodeToString(h.Sum(nil)) != file.SHA256 {
			return fmt.Errorf("中文流式模型 %s 校验失败，请重新准备模型包", file.Filename)
		}
	}
	return nil
}

// These layouts match sherpa-onnx 2.0.1 c-api.h on the supported 64-bit targets.
// They are passed by pointer, never as platform-dependent structs by value.
type modelConfig struct {
	Encoder, Decoder, Joiner                        *byte
	ParaformerEncoder, ParaformerDecoder            *byte
	ZipformerCTC, Tokens                            *byte
	Threads                                         int32
	Provider                                        *byte
	Debug                                           int32
	ModelType, ModelingUnit, BPEVocab, TokensBuffer *byte
	TokensBufferSize                                int32
	NemoCTC, ToneCTC                                *byte
}
type recognizerConfig struct {
	SampleRate, FeatureDim          int32
	Model                           modelConfig
	DecodingMethod                  *byte
	MaxActivePaths, EnableEndpoint  int32
	Rule1, Rule2, Rule3             float32
	HotwordsFile                    *byte
	HotwordsScore                   float32
	CTCGraph                        *byte
	CTCMaxActive                    int32
	RuleFSTs, RuleFARs              *byte
	BlankPenalty                    float32
	HotwordsBuffer                  *byte
	HotwordsBufferSize              int32
	HRDirectory, HRLexicon, HRRules *byte
}

type nativeAPI struct {
	version       func() string
	create        func(*recognizerConfig) uintptr
	destroy       func(uintptr)
	createStream  func(uintptr) uintptr
	destroyStream func(uintptr)
	accept        func(uintptr, int32, *float32, int32)
	ready         func(uintptr, uintptr) int32
	decode        func(uintptr, uintptr)
	result        func(uintptr, uintptr) *byte
	freeResult    func(*byte)
	endpoint      func(uintptr, uintptr) int32
	reset         func(uintptr, uintptr)
	finish        func(uintptr)
}

var libraries = struct {
	sync.Mutex
	loaded map[string]*nativeAPI
}{loaded: map[string]*nativeAPI{}}

// Libraries stay loaded for the process lifetime: ONNX Runtime owns global
// thread-local state. Recognizers and per-channel streams are still released.
func loadAPI(dir string) (*nativeAPI, error) {
	libraries.Lock()
	defer libraries.Unlock()
	if api := libraries.loaded[dir]; api != nil {
		return api, nil
	}
	h, err := openLibrary(dir)
	if err != nil {
		return nil, fmt.Errorf("加载本地实时语音组件失败：%w", err)
	}
	a := &nativeAPI{}
	for _, item := range []struct {
		name string
		fn   any
	}{
		{"GetVersionStr", &a.version}, {"CreateOnlineRecognizer", &a.create},
		{"DestroyOnlineRecognizer", &a.destroy}, {"CreateOnlineStream", &a.createStream},
		{"DestroyOnlineStream", &a.destroyStream}, {"OnlineStreamAcceptWaveform", &a.accept},
		{"IsOnlineStreamReady", &a.ready}, {"DecodeOnlineStream", &a.decode},
		{"GetOnlineStreamResultAsJson", &a.result}, {"DestroyOnlineStreamResultJson", &a.freeResult},
		{"OnlineStreamIsEndpoint", &a.endpoint}, {"OnlineStreamReset", &a.reset},
		{"OnlineStreamInputFinished", &a.finish},
	} {
		symbol, err := lookupSymbol(h, "SherpaOnnx"+item.name)
		if err != nil {
			return nil, fmt.Errorf("本地实时语音组件缺少接口 %s", item.name)
		}
		purego.RegisterFunc(item.fn, symbol)
	}
	if v := strings.TrimPrefix(a.version(), "v"); v != Version {
		return nil, fmt.Errorf("本地实时语音组件版本不匹配，需要 %s", Version)
	}
	libraries.loaded[dir] = a
	return a, nil
}

func RuntimeDir() (string, error) {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	for _, p := range []string{
		filepath.Join(dir, "sherpa"), filepath.Join(dir, "..", "Resources", "sherpa"),
		filepath.Join("build", "bin", "sherpa-"+runtime.GOOS+"-"+runtime.GOARCH),
	} {
		if info, err := os.Stat(filepath.Join(p, libraryName)); err == nil && !info.IsDir() {
			return filepath.Abs(p)
		}
	}
	return "", errors.New("缺少本地实时语音组件，请使用包含 Sherpa 的完整安装包，或运行 node scripts/prepare-sherpa.mjs")
}

type decoder interface {
	NewStream() (uintptr, error)
	Accept(uintptr, []float32)
	Ready(uintptr) bool
	Decode(uintptr)
	Result(uintptr) (string, error)
	Endpoint(uintptr) bool
	Reset(uintptr)
	Finish(uintptr)
	DestroyStream(uintptr)
	Close()
}

type nativeDecoder struct {
	api    *nativeAPI
	handle uintptr
}

func newDecoder(dir string) (decoder, error) {
	if err := verifyModel(dir); err != nil {
		return nil, err
	}
	var cfg recognizerConfig
	str := func(s string) *byte { return &append([]byte(s), 0)[0] }
	paths := []**byte{&cfg.Model.Encoder, &cfg.Model.Decoder, &cfg.Model.Joiner, &cfg.Model.Tokens}
	files := []string{"encoder.int8.onnx", "decoder.onnx", "joiner.int8.onnx", "tokens.txt"}
	for i, name := range files {
		path := filepath.Join(dir, "model", name)
		if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() == 0 {
			return nil, fmt.Errorf("缺少中文流式模型 %s，请重新准备完整模型包", name)
		}
		*paths[i] = str(path)
	}
	cfg.SampleRate, cfg.FeatureDim = 16000, 80
	cfg.Model.Threads, cfg.Model.Provider = 2, str("cpu")
	cfg.Model.ModelType = str("zipformer2")
	cfg.DecodingMethod, cfg.MaxActivePaths = str("modified_beam_search"), 4
	cfg.EnableEndpoint, cfg.Rule1, cfg.Rule2, cfg.Rule3 = 1, 2.4, .7, 20
	a, err := loadAPI(dir)
	if err != nil {
		return nil, err
	}
	h := a.create(&cfg)
	runtime.KeepAlive(cfg)
	if h == 0 {
		return nil, errors.New("中文流式模型加载失败，请检查模型文件和可用内存")
	}
	return &nativeDecoder{api: a, handle: h}, nil
}

func (d *nativeDecoder) NewStream() (uintptr, error) {
	s := d.api.createStream(d.handle)
	if s == 0 {
		return 0, errors.New("无法创建实时语音通道")
	}
	return s, nil
}
func (d *nativeDecoder) Accept(s uintptr, pcm []float32) {
	if len(pcm) == 0 {
		return
	}
	d.api.accept(s, 16000, &pcm[0], int32(len(pcm)))
	runtime.KeepAlive(pcm)
}
func (d *nativeDecoder) Ready(s uintptr) bool    { return d.api.ready(d.handle, s) != 0 }
func (d *nativeDecoder) Decode(s uintptr)        { d.api.decode(d.handle, s) }
func (d *nativeDecoder) Endpoint(s uintptr) bool { return d.api.endpoint(d.handle, s) != 0 }
func (d *nativeDecoder) Reset(s uintptr)         { d.api.reset(d.handle, s) }
func (d *nativeDecoder) Finish(s uintptr)        { d.api.finish(s) }
func (d *nativeDecoder) DestroyStream(s uintptr) { d.api.destroyStream(s) }
func (d *nativeDecoder) Close()                  { d.api.destroy(d.handle) }
func (d *nativeDecoder) Result(s uintptr) (string, error) {
	p := d.api.result(d.handle, s)
	if p == nil {
		return "", errors.New("实时语音组件未返回识别结果")
	}
	defer d.api.freeResult(p)
	var data []byte
	for n := 0; n < 1<<20; n++ {
		v := *(*byte)(unsafe.Add(unsafe.Pointer(p), n))
		if v == 0 {
			var result struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(data, &result) != nil {
				return "", errors.New("实时语音组件返回了无效结果")
			}
			return strings.TrimSpace(result.Text), nil
		}
		data = append(data, v)
	}
	return "", errors.New("实时语音结果超出长度限制")
}

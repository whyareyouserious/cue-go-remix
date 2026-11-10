package whisper

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cue-go-remix/internal/model"
)

//go:embed catalog.json
var catalogJSON []byte

type Artifact struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	URL      string `json:"url"`
}

func Catalog() []Artifact { var a []Artifact; _ = json.Unmarshal(catalogJSON, &a); return a }

type Manager struct {
	dir    string
	mu     sync.Mutex
	active map[string]context.CancelFunc
	used   string
	emit   func(model.Event)
	wg     sync.WaitGroup
	closed bool
}

func NewManager(dir string, emit func(model.Event)) *Manager {
	return &Manager{dir: dir, active: map[string]context.CancelFunc{}, emit: emit}
}
func (m *Manager) artifact(id string) (Artifact, error) {
	for _, a := range Catalog() {
		if a.ID == id {
			return a, nil
		}
	}
	return Artifact{}, errors.New("不支持的 Whisper 模型")
}
func (m *Manager) Path(id string) (string, error) {
	a, e := m.artifact(id)
	if e != nil {
		return "", e
	}
	return filepath.Join(m.dir, a.Filename), nil
}
func (m *Manager) List() []model.ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.ModelInfo{}
	for _, a := range Catalog() {
		p := filepath.Join(m.dir, a.Filename)
		st, e := os.Stat(p)
		v := model.ModelInfo{ID: a.ID, Bytes: a.Bytes, SHA256: a.SHA256, Installed: e == nil && st.Size() == a.Bytes, Downloading: m.active[a.ID] != nil}
		if s, e := os.Stat(p + ".part"); e == nil {
			v.Downloaded = s.Size()
		}
		if v.Installed {
			v.Downloaded = a.Bytes
		}
		out = append(out, v)
	}
	return out
}
func (m *Manager) notify() { m.emit(model.Event{Type: "models", Data: m.List()}) }
func (m *Manager) Use(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active[id] != nil {
		return errors.New("模型仍在下载")
	}
	m.used = id
	return nil
}
func (m *Manager) Release() { m.mu.Lock(); m.used = ""; m.mu.Unlock() }
func verify(path string, a Artifact) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil {
		return e
	}
	if s.Size() != a.Bytes {
		return errors.New("模型大小不匹配，请重新下载")
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return e
	}
	if hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return errors.New("模型 SHA-256 校验失败，请删除后重新下载")
	}
	return nil
}
func (m *Manager) Verify(id string) error {
	a, e := m.artifact(id)
	if e != nil {
		return e
	}
	return verify(filepath.Join(m.dir, a.Filename), a)
}
func (m *Manager) Download(parent context.Context, id string) error {
	a, e := m.artifact(id)
	if e != nil {
		return e
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("模型管理器已关闭")
	}
	if m.used == id {
		m.mu.Unlock()
		return errors.New("模型使用中，不能替换")
	}
	if m.active[id] != nil {
		m.mu.Unlock()
		return errors.New("模型已在下载")
	}
	ctx, cancel := context.WithCancel(parent)
	m.active[id] = cancel
	m.wg.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.wg.Done()
		defer cancel()
		err := m.download(ctx, a)
		m.mu.Lock()
		delete(m.active, id)
		m.mu.Unlock()
		if err != nil && !errors.Is(err, context.Canceled) {
			m.emit(model.Event{Type: "error", Text: "模型下载失败：" + err.Error()})
		}
		m.notify()
	}()
	m.notify()
	return nil
}
func (m *Manager) download(ctx context.Context, a Artifact) error {
	if e := os.MkdirAll(m.dir, 0700); e != nil {
		return e
	}
	path := filepath.Join(m.dir, a.Filename)
	part := path + ".part"
	var offset int64
	if s, e := os.Stat(part); e == nil {
		offset = s.Size()
	}
	if offset > a.Bytes {
		return errors.New("下载片段大小异常，请删除后重新下载")
	}
	if offset < a.Bytes {
		req, e := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
		if e != nil {
			return e
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		client := &http.Client{Timeout: 4 * time.Hour}
		res, e := client.Do(req)
		if e != nil {
			return e
		}
		defer res.Body.Close()
		if res.StatusCode != 200 && res.StatusCode != 206 {
			return fmt.Errorf("下载服务器 HTTP %d", res.StatusCode)
		}
		if res.StatusCode == 206 {
			if !strings.HasPrefix(res.Header.Get("Content-Range"), "bytes "+strconv.FormatInt(offset, 10)+"-") {
				return errors.New("下载服务器返回错误的续传范围")
			}
		} else {
			offset = 0
		}
		flags := os.O_WRONLY | os.O_CREATE
		if offset == 0 {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_APPEND
		}
		f, e := os.OpenFile(part, flags, 0600)
		if e != nil {
			return e
		}
		buf := make([]byte, 128*1024)
		last := time.Time{}
		for {
			n, rerr := res.Body.Read(buf)
			if n > 0 {
				if offset+int64(n) > a.Bytes {
					f.Close()
					return errors.New("下载内容超出预期模型大小")
				}
				wn, we := f.Write(buf[:n])
				offset += int64(wn)
				if we != nil {
					f.Close()
					return we
				}
				if time.Since(last) > 250*time.Millisecond {
					m.emit(model.Event{Type: "download", Data: map[string]any{"id": a.ID, "downloaded": offset, "bytes": a.Bytes}})
					last = time.Now()
				}
			}
			if rerr != nil {
				if !errors.Is(rerr, io.EOF) {
					f.Close()
					return rerr
				}
				break
			}
		}
		if e = f.Sync(); e != nil {
			f.Close()
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := verify(part, a); e != nil {
		return e
	}
	return os.Rename(part, path)
}
func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.active[id]; c != nil {
		c()
	}
}
func (m *Manager) Delete(id string) error {
	a, e := m.artifact(id)
	if e != nil {
		return e
	}
	m.mu.Lock()
	if m.used == id || m.active[id] != nil {
		m.mu.Unlock()
		return errors.New("请先停止使用或取消下载，等待任务结束后再删除")
	}
	for _, suffix := range []string{"", ".part"} {
		e = os.Remove(filepath.Join(m.dir, a.Filename) + suffix)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			m.mu.Unlock()
			return e
		}
	}
	m.mu.Unlock()
	m.notify()
	return nil
}
func (m *Manager) Import(id, source string) error {
	a, e := m.artifact(id)
	if e != nil {
		return e
	}
	m.mu.Lock()
	if m.closed || m.used == id || m.active[id] != nil {
		m.mu.Unlock()
		return errors.New("模型使用中或下载中")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.active[id] = cancel
	m.wg.Add(1)
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.active, id)
		m.mu.Unlock()
		m.notify()
		m.wg.Done()
	}()
	if e = os.MkdirAll(m.dir, 0700); e != nil {
		return e
	}
	in, e := os.Open(source)
	if e != nil {
		return e
	}
	defer in.Close()
	f, e := os.CreateTemp(m.dir, ".import-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	_, e = io.Copy(f, &contextReader{ctx: ctx, r: io.LimitReader(in, a.Bytes+1)})
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = verify(f.Name(), a); e != nil {
		return e
	}
	if e = os.Rename(f.Name(), filepath.Join(m.dir, a.Filename)); e != nil {
		return e
	}
	return nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, c := range m.active {
		c()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}

package whisper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"cue-go-remix/internal/model"
)

func TestResumeDownloadAndVerify(t *testing.T) {
	body := []byte("a model file whose hash must match")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=8-" {
			t.Errorf("错误续传范围 %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 8-%d/%d", len(body)-1, len(body)))
		w.WriteHeader(206)
		_, _ = w.Write(body[8:])
	}))
	defer srv.Close()
	dir := t.TempDir()
	m := NewManager(dir, func(model.Event) {})
	a := Artifact{ID: "test", Filename: "model.bin", Bytes: int64(len(body)), SHA256: hex.EncodeToString(sum[:]), URL: srv.URL}
	if e := os.WriteFile(filepath.Join(dir, "model.bin.part"), body[:8], 0600); e != nil {
		t.Fatal(e)
	}
	if e := m.download(context.Background(), a); e != nil {
		t.Fatal(e)
	}
	if e := verify(filepath.Join(dir, "model.bin"), a); e != nil {
		t.Fatal(e)
	}
}
func TestBadDownloadNeverPromoted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("bad")) }))
	defer srv.Close()
	dir := t.TempDir()
	m := NewManager(dir, func(model.Event) {})
	a := Artifact{ID: "bad", Filename: "bad.bin", Bytes: 3, SHA256: "wrong", URL: srv.URL}
	if e := m.download(context.Background(), a); e == nil {
		t.Fatal("未拒绝错误哈希")
	}
	if _, e := os.Stat(filepath.Join(dir, "bad.bin")); !os.IsNotExist(e) {
		t.Fatal("未校验模型不能作为可用模型")
	}
	if _, e := m.Path("../../escape"); e == nil {
		t.Fatal("未拒绝越界模型ID")
	}
}
func TestCanceledDownloadKeepsPartial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewManager(t.TempDir(), func(model.Event) {})
	if e := m.download(ctx, Artifact{Filename: "cancel.bin", Bytes: 10, URL: "http://127.0.0.1:1"}); e == nil {
		t.Fatal("取消后仍下载成功")
	}
}

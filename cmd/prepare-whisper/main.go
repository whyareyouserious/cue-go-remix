// prepare-whisper downloads a pinned runtime or builds the pinned source on macOS.
package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "准备 Whisper 失败：", err)
		os.Exit(1)
	}
}
func run() error {
	platform := flag.String("platform", runtime.GOOS, "目标系统 darwin/windows")
	arch := flag.String("arch", runtime.GOARCH, "目标架构 amd64/arm64")
	flag.Parse()
	if *platform != "darwin" && *platform != "windows" {
		return errors.New("当前运行时准备脚本支持 macOS 和 Windows")
	}
	if *platform == "windows" && *arch != "amd64" {
		return errors.New("固定 Windows 运行时仅支持 amd64")
	}
	if *platform == "darwin" && runtime.GOOS != "darwin" {
		return errors.New("macOS 运行时需要在 macOS 编译")
	}
	out, _ := filepath.Abs(filepath.Join("build", "bin", "whisper-"+*platform+"-"+*arch))
	if e := os.MkdirAll(out, 0755); e != nil {
		return e
	}
	cache, _ := filepath.Abs(filepath.Join("build", "cache", "whisper-1.9.1"))
	if e := os.MkdirAll(cache, 0755); e != nil {
		return e
	}
	url := "https://github.com/ggml-org/whisper.cpp/releases/download/v1.9.1/whisper-bin-x64.zip"
	hash := "7d8be46ecd31828e1eb7a2ecdd0d6b314feafd82163038ab6092594b0a063539"
	name := "whisper-bin-x64.zip"
	if *platform == "darwin" {
		url = "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/v1.9.1.zip"
		hash = "98a57a88ef0e733b746544f8ea25157d3265fbf0dac5c32dbb527e6ef4dbfaac"
		name = "whisper-source.zip"
	}
	archive := filepath.Join(cache, name)
	if e := download(url, archive, hash); e != nil {
		return e
	}
	source := filepath.Join(cache, *platform+"-"+*arch)
	if e := extract(archive, source); e != nil {
		return e
	}
	if *platform == "darwin" {
		source = filepath.Join(source, "whisper.cpp-1.9.1")
		build := filepath.Join(source, "build-cue")
		nativeArch := "arm64"
		if *arch == "amd64" {
			nativeArch = "x86_64"
		}
		args := []string{"-S", source, "-B", build, "-DCMAKE_BUILD_TYPE=Release", "-DBUILD_SHARED_LIBS=OFF", "-DGGML_NATIVE=OFF", "-DWHISPER_BUILD_SERVER=ON", "-DWHISPER_BUILD_TESTS=OFF", "-DWHISPER_CURL=OFF", "-DCMAKE_OSX_DEPLOYMENT_TARGET=13.3", "-DCMAKE_OSX_ARCHITECTURES=" + nativeArch}
		if e := command("cmake", args...); e != nil {
			return e
		}
		if e := command("cmake", "--build", build, "--config", "Release", "--target", "whisper-server", "-j", "4"); e != nil {
			return e
		}
		if e := copyFile(filepath.Join(build, "bin", "whisper-server"), filepath.Join(out, "whisper-server"), 0755); e != nil {
			return e
		}
		_ = copyFile(filepath.Join(source, "LICENSE"), filepath.Join(out, "LICENSE"), 0644)
	} else {
		found := false
		err := filepath.WalkDir(source, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			lower := strings.ToLower(d.Name())
			if lower == "whisper-server.exe" {
				found = true
			}
			if strings.HasSuffix(lower, ".dll") || lower == "whisper-server.exe" || strings.Contains(lower, "license") {
				return copyFile(path, filepath.Join(out, d.Name()), 0644)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !found {
			return errors.New("下载包缺少 whisper-server.exe")
		}
	}
	fmt.Println("Whisper 1.9.1 已准备：", out)
	return nil
}
func command(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
func digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func download(url, path, hash string) error {
	if sum, e := digest(path); e == nil && sum == hash {
		return nil
	}
	fmt.Println("下载固定版本：", url)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("下载返回 HTTP %d", res.StatusCode)
	}
	tmp := path + ".download"
	f, e := os.Create(tmp)
	if e != nil {
		return e
	}
	defer os.Remove(tmp)
	_, e = io.Copy(f, io.LimitReader(res.Body, 256<<20))
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	sum, e := digest(tmp)
	if e != nil {
		return e
	}
	if sum != hash {
		return fmt.Errorf("运行时校验失败：期望 %s，实际 %s；未使用下载内容", hash, sum)
	}
	return os.Rename(tmp, path)
}
func extract(archive, dest string) error {
	r, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer r.Close()
	for _, f := range r.File {
		path := filepath.Join(dest, filepath.FromSlash(f.Name))
		rel, e := filepath.Rel(dest, path)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return errors.New("归档包含越界路径")
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("归档包含不支持的符号链接")
		}
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(path, 0755); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		in, e := f.Open()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if e != nil {
			in.Close()
			return e
		}
		_, e = io.Copy(out, in)
		in.Close()
		ce := out.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	return nil
}
func copyFile(from, to string, mode os.FileMode) error {
	in, e := os.Open(from)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}

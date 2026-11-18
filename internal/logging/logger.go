package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// Logger 管理本机诊断日志和启动时轮换；由 service.Runtime 持有。
// 只记录固定事件名、枚举和数值，禁止写入设置、URL、密钥或对话正文。
type Logger struct {
	// file 以受限权限打开的本机诊断文件。
	file *os.File
	// logger 同时写入文件与标准错误，支持并发记录。
	logger *log.Logger
}

// Open 打开 dir/logs/cue.log；启动时超过 1 MiB 的旧日志轮换为 cue.previous.log，文件操作失败返回错误。
func Open(dir string) (*Logger, error) {
	dir = filepath.Join(dir, "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "cue.log")
	if info, err := os.Stat(path); err == nil && info.Size() >= 1024*1024 {
		previous := filepath.Join(dir, "cue.previous.log")
		_ = os.Remove(previous)
		if err := os.Rename(path, previous); err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Logger{file: file, logger: log.New(io.MultiWriter(file, os.Stderr), fmt.Sprintf("pid=%d ", os.Getpid()), log.LstdFlags|log.Lmicroseconds)}, nil
}

// Trace 将固定格式的诊断事件写入日志；format 和 args 不得包含用户正文或密钥。
func (l *Logger) Trace(format string, args ...any) {
	if l != nil {
		l.logger.Printf(format, args...)
	}
}

// Close 关闭日志文件；未创建日志实例时安全返回 nil。
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	return l.file.Close()
}

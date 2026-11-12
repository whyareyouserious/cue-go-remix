package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// 只写固定事件名、枚举值和数值，禁止写入设置、URL、密钥和对话原文。
type Logger struct {
	file   *os.File
	logger *log.Logger
}

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

func (l *Logger) Trace(format string, args ...any) {
	if l != nil {
		l.logger.Printf(format, args...)
	}
}
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	return l.file.Close()
}

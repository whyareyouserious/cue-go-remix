//go:build !windows

package repository

import "os"

// replaceFile 以平台原子替换方式将 from 移至 to；操作失败返回底层文件错误。
func replaceFile(from, to string) error { return os.Rename(from, to) }

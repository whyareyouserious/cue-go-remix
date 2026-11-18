package config

import (
	"os"
	"path/filepath"
)

// DataDir 优先读取 CUE_DATA_DIR，否则返回用户配置目录下的 cue-go-remix；系统目录查询失败返回错误。
func DataDir() (string, error) {
	if dir := os.Getenv("CUE_DATA_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "cue-go-remix"), nil
}

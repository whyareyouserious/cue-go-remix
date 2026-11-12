package config

import (
	"os"
	"path/filepath"
)

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

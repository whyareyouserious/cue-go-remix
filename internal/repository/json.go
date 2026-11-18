package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteJSON 将 value 以受限权限原子写入 path，并备份上一份有效 JSON；编码或文件操作失败返回错误。
func WriteJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".cue-write-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if old, e := os.ReadFile(path); e == nil && json.Valid(old) {
		if e = os.WriteFile(path+".bak", old, 0600); e != nil {
			return e
		}
	}
	return replaceFile(tmp, path)
}

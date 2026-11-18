package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// readRecover 将 path 解码到 out，主文件损坏时尝试备份；每次从默认值重新解码，避免失败数据污染恢复结果。
func readRecover[T any](path string, out *T) error {
	// 类型不匹配时反序列化可能已修改部分字段；每次从默认副本解码，防止损坏主文件污染备份恢复。
	defaults, err := json.Marshal(out)
	if err != nil {
		return err
	}
	decode := func(b []byte) error {
		var candidate T
		if err := json.Unmarshal(defaults, &candidate); err != nil {
			return err
		}
		if err := json.Unmarshal(b, &candidate); err != nil {
			return err
		}
		*out = candidate
		return nil
	}
	b, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !missing {
		err = decode(b)
		if err == nil {
			return nil
		}
	}
	backup, e := os.ReadFile(path + ".bak")
	if e != nil {
		if missing && errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if missing {
			return fmt.Errorf("主文件缺失，备份不可读：%w", e)
		}
		return fmt.Errorf("主文件损坏，备份不可读：%w", err)
	}
	if e = decode(backup); e != nil {
		return fmt.Errorf("主文件和备份均损坏：%w", e)
	}
	return nil
}

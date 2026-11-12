package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func readRecover[T any](path string, out *T) error {
	// Unmarshal can partially change a destination before reporting a type
	// mismatch. Decode a fresh copy of the defaults for each attempt so an
	// invalid main file cannot contaminate the recovered backup.
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

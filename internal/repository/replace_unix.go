//go:build !windows

package repository

import "os"

func replaceFile(from, to string) error { return os.Rename(from, to) }

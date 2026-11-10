//go:build (!darwin && !windows) || (darwin && !cgo)

package platform

import "errors"

func CursorPosition() (x, y int, err error) {
	return 0, 0, errors.New("当前平台无法读取全局鼠标位置")
}

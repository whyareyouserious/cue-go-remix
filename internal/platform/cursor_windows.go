//go:build windows

package platform

import (
	"fmt"
	"unsafe"
)

// CursorPosition returns physical screen coordinates. On Windows, callers must
// convert through Wails Screen.PhysicalToDipPoint before comparing window DIP
// bounds; do not compare these values directly on mixed-DPI displays.
func CursorPosition() (x, y int, err error) {
	var point struct{ X, Y int32 }
	ok, _, e := user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return 0, 0, fmt.Errorf("无法读取鼠标位置：%v", e)
	}
	return int(point.X), int(point.Y), nil
}

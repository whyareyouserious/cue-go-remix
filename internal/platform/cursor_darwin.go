//go:build darwin && cgo

package platform

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static int cue_cursor_position(double *x, double *y) {
    CGEventRef event = CGEventCreate(NULL);
    if (!event) return 0;
    CGPoint location = CGEventGetLocation(event);
    CFRelease(event);
    *x = location.x;
    *y = location.y;
    return 1;
}
*/
import "C"

import (
	"errors"
	"math"
)

// CursorPosition uses CoreGraphics' top-left global display coordinate system,
// matching Wails window positions (logical points, including negative screens).
func CursorPosition() (x, y int, err error) {
	var cx, cy C.double
	if C.cue_cursor_position(&cx, &cy) == 0 {
		return 0, 0, errors.New("无法读取鼠标位置")
	}
	return int(math.Round(float64(cx))), int(math.Round(float64(cy))), nil
}

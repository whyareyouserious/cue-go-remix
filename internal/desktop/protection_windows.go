package desktop

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func protectionAvailable() bool {
	session := strings.ToLower(os.Getenv("SESSIONNAME"))
	if session != "" && session != "console" {
		return false
	}
	var version struct {
		Size, Major, Minor, Build, Platform uint32
		ServicePack                         [128]uint16
	}
	version.Size = uint32(unsafe.Sizeof(version))
	r, _, _ := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion").Call(uintptr(unsafe.Pointer(&version)))
	return r == 0 && (version.Major > 10 || (version.Major == 10 && version.Build >= 19041))
}

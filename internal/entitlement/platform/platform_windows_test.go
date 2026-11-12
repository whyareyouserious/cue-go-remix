//go:build windows

package platform

import (
	"errors"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Compile-time ABI assertions also run during cross compilation. A pointer-size
// difference adds padding before the credential blob and shifts later fields.
const (
	pointerSize        = unsafe.Sizeof(uintptr(0))
	expectedSize       = 52 + (pointerSize-4)*7
	expectedBlobOffset = 28 + (pointerSize-4)*3
	expectedPersist    = 32 + (pointerSize-4)*4
	expectedAttributes = 40 + (pointerSize-4)*4
	expectedUserName   = 48 + (pointerSize-4)*6
)

var (
	_ [unsafe.Sizeof(windowsCredential{}) - expectedSize]byte
	_ [expectedSize - unsafe.Sizeof(windowsCredential{})]byte
	_ [unsafe.Offsetof(windowsCredential{}.CredentialBlob) - expectedBlobOffset]byte
	_ [expectedBlobOffset - unsafe.Offsetof(windowsCredential{}.CredentialBlob)]byte
	_ [unsafe.Offsetof(windowsCredential{}.Persist) - expectedPersist]byte
	_ [expectedPersist - unsafe.Offsetof(windowsCredential{}.Persist)]byte
	_ [unsafe.Offsetof(windowsCredential{}.Attributes) - expectedAttributes]byte
	_ [expectedAttributes - unsafe.Offsetof(windowsCredential{}.Attributes)]byte
	_ [unsafe.Offsetof(windowsCredential{}.UserName) - expectedUserName]byte
	_ [expectedUserName - unsafe.Offsetof(windowsCredential{}.UserName)]byte
)

func TestCredentialErrorMapping(t *testing.T) {
	if err := credentialError("读取", windows.ERROR_NOT_FOUND); !errors.Is(err, ErrNotFound) {
		t.Fatal("Windows 无凭据状态应返回 ErrNotFound")
	}
	if err := credentialError("保存", windows.ERROR_ACCESS_DENIED); !errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, ErrNotFound) {
		t.Fatal("Windows 权限错误应保留系统错误")
	}
}

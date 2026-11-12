//go:build windows

package platform

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	credentialTypeGeneric  = 1
	credentialPersistLocal = 2
)

var (
	credentialDLL = windows.NewLazySystemDLL("advapi32.dll")
	credRead      = credentialDLL.NewProc("CredReadW")
	credWrite     = credentialDLL.NewProc("CredWriteW")
	credDelete    = credentialDLL.NewProc("CredDeleteW")
	credFree      = credentialDLL.NewProc("CredFree")
)

// windowsCredential matches CREDENTIALW on both 32-bit and 64-bit Windows.
type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         unsafe.Pointer
	TargetAlias        *uint16
	UserName           *uint16
}

func systemMachineID() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("无法打开系统设备标识：%w", err)
	}
	defer key.Close()
	id, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("无法读取系统设备标识：%w", err)
	}
	return id, nil
}

func loadCredential() ([]byte, error) {
	target, _ := windows.UTF16PtrFromString(credentialService)
	var credential *windowsCredential
	result, _, err := credRead.Call(uintptr(unsafe.Pointer(target)), credentialTypeGeneric, 0, uintptr(unsafe.Pointer(&credential)))
	runtime.KeepAlive(target)
	if result == 0 {
		return nil, credentialError("读取", err)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(credential)))
	if credential == nil || credential.CredentialBlob == nil || credential.CredentialBlobSize == 0 || credential.CredentialBlobSize > maxCredentialBytes {
		return nil, errors.New("已保存的系统授权凭据长度无效")
	}
	return append([]byte(nil), unsafe.Slice(credential.CredentialBlob, int(credential.CredentialBlobSize))...), nil
}

func saveCredential(data []byte) error {
	target, _ := windows.UTF16PtrFromString(credentialService)
	credential := windowsCredential{
		Type:               credentialTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(data)),
		CredentialBlob:     &data[0],
		Persist:            credentialPersistLocal,
	}
	result, _, err := credWrite.Call(uintptr(unsafe.Pointer(&credential)), 0)
	runtime.KeepAlive(credential)
	runtime.KeepAlive(data)
	runtime.KeepAlive(target)
	if result == 0 {
		return credentialError("保存", err)
	}
	return nil
}

func deleteCredential() error {
	target, _ := windows.UTF16PtrFromString(credentialService)
	result, _, err := credDelete.Call(uintptr(unsafe.Pointer(target)), credentialTypeGeneric, 0)
	runtime.KeepAlive(target)
	if result == 0 {
		return credentialError("删除", err)
	}
	return nil
}

func credentialError(operation string, err error) error {
	if errors.Is(err, windows.ERROR_NOT_FOUND) {
		return ErrNotFound
	}
	return fmt.Errorf("%s系统授权凭据失败：%w", operation, err)
}

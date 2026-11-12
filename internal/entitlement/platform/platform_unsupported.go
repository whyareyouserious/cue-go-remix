//go:build (!darwin && !windows) || (darwin && !cgo)

package platform

import "errors"

var errUnsupported = errors.New("当前构建不支持系统设备标识和安全凭据存储，请使用启用 cgo 的 macOS 或 Windows 版本")

func systemMachineID() (string, error) { return "", errUnsupported }
func loadCredential() ([]byte, error)  { return nil, errUnsupported }
func saveCredential([]byte) error      { return errUnsupported }
func deleteCredential() error          { return errUnsupported }

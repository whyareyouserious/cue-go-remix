// Package platform stores authorization credentials in the operating system's
// credential store and derives Cue's device identifier without exposing its source.
package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
)

const (
	credentialService = "com.cue.remix.license"
	machineNamespace  = "com.cue.remix/license/v1:"
	// Windows generic credentials support at most 2560 bytes.
	maxCredentialBytes = 2560
)

var ErrNotFound = errors.New("未保存授权凭据")

type Credential struct {
	Mode       string `json:"mode"`
	LicenseKey string `json:"licenseKey,omitempty"`
}

// MachineID is stable across Cue reinstalls, but can change after an OS reinstall.
func MachineID() (string, error) {
	id, err := systemMachineID()
	if err != nil {
		return "", err
	}
	return hashMachineID(runtime.GOOS, id)
}

func Load() (Credential, error) {
	data, err := loadCredential()
	if err != nil {
		return Credential{}, err
	}
	return decodeCredential(data)
}

func Save(credential Credential) error {
	data, err := encodeCredential(credential)
	if err != nil {
		return err
	}
	return saveCredential(data)
}

func Delete() error { return deleteCredential() }

func hashMachineID(goos, id string) (string, error) {
	// Both supported systems provide UUIDs. Canonicalize case, surrounding
	// whitespace and optional braces before hashing the standard UUID form.
	id = strings.ToLower(strings.TrimSpace(id))
	if strings.HasPrefix(id, "{") && strings.HasSuffix(id, "}") {
		id = id[1 : len(id)-1]
	}
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return "", errors.New("系统设备标识格式无效")
	}
	value, err := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	if err != nil || len(value) != 16 {
		return "", errors.New("系统设备标识格式无效")
	}
	var nonzero byte
	for _, b := range value {
		nonzero |= b
	}
	if nonzero == 0 {
		return "", errors.New("系统设备标识为空")
	}
	digest := sha256.Sum256([]byte(machineNamespace + goos + ":" + id))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeCredential(credential Credential) (Credential, error) {
	switch credential.Mode {
	case "license":
		credential.LicenseKey = strings.TrimSpace(credential.LicenseKey)
		if credential.LicenseKey == "" {
			return Credential{}, errors.New("授权凭据缺少卡密")
		}
	case "trial":
		credential.LicenseKey = ""
	default:
		return Credential{}, errors.New("授权凭据模式无效")
	}
	return credential, nil
}

func encodeCredential(credential Credential) ([]byte, error) {
	credential, err := normalizeCredential(credential)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(credential)
	if err != nil {
		return nil, errors.New("授权凭据编码失败")
	}
	if len(data) > maxCredentialBytes {
		return nil, errors.New("授权凭据超出系统存储限制")
	}
	return data, nil
}

func decodeCredential(data []byte) (Credential, error) {
	if len(data) == 0 || len(data) > maxCredentialBytes {
		return Credential{}, errors.New("已保存的授权凭据长度无效")
	}
	var credential Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		// JSON errors can include input text; never return them with stored secrets.
		return Credential{}, errors.New("已保存的授权凭据格式无效")
	}
	return normalizeCredential(credential)
}

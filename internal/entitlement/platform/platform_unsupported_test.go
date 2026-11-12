//go:build (!darwin && !windows) || (darwin && !cgo)

package platform

import (
	"errors"
	"testing"
)

func TestUnsupportedBuildReturnsExplicitErrors(t *testing.T) {
	if id, err := MachineID(); id != "" || !errors.Is(err, errUnsupported) {
		t.Fatal("不支持的平台不应生成随机设备标识")
	}
	if _, err := Load(); !errors.Is(err, errUnsupported) {
		t.Fatal("不支持的平台应明确拒绝读取凭据")
	}
	if err := Save(Credential{Mode: "trial"}); !errors.Is(err, errUnsupported) {
		t.Fatal("不支持的平台不应降级存储凭据")
	}
	if err := Delete(); !errors.Is(err, errUnsupported) {
		t.Fatal("不支持的平台应明确拒绝删除凭据")
	}
}

//go:build darwin && cgo

package platform

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Check status mapping without calling Keychain, which could prompt the user.
func TestKeychainStatusMapping(t *testing.T) {
	if err := keychainError("读取", 0); err != nil {
		t.Fatal("Keychain 成功状态应返回 nil")
	}
	if err := keychainError("读取", -25300); !errors.Is(err, ErrNotFound) {
		t.Fatal("Keychain 无凭据状态应返回 ErrNotFound")
	}
	if err := keychainError("读取", -25293); err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "-25293") {
		t.Fatal("Keychain 权限错误应保留错误码")
	}
	if err := keychainError("读取", -25308); err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "-25308") || !strings.Contains(err.Error(), "需要访问许可") {
		t.Fatal("Keychain 非交互错误应保留错误码并提示手动处理权限")
	}
}

func TestKeychainInteractionPolicyAlwaysRestored(t *testing.T) {
	getErr := errors.New("读取策略失败")
	setErr := errors.New("禁用策略失败")
	actionErr := errors.New("凭据操作失败")
	restoreErr := errors.New("恢复策略失败")
	for _, tc := range []struct {
		name                      string
		allowed                   bool
		get, set, action, restore error
		wantCalls                 []string
	}{
		{"success", true, nil, nil, nil, nil, []string{"get", "disable", "action", "restore"}},
		{"alreadyDisabled", false, nil, nil, nil, nil, []string{"get", "disable", "action", "restore"}},
		{"getFails", true, getErr, nil, nil, nil, []string{"get"}},
		{"disableFails", true, nil, setErr, nil, nil, []string{"get", "disable", "restore"}},
		{"actionFails", true, nil, nil, actionErr, nil, []string{"get", "disable", "action", "restore"}},
		{"restoreFails", true, nil, nil, nil, restoreErr, []string{"get", "disable", "action", "restore"}},
		{"actionAndRestoreFail", true, nil, nil, actionErr, restoreErr, []string{"get", "disable", "action", "restore"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			sets := 0
			err := withInteractionDisabled(func() (bool, error) {
				calls = append(calls, "get")
				return tc.allowed, tc.get
			}, func(allowed bool) error {
				sets++
				if sets == 1 {
					calls = append(calls, "disable")
					if allowed {
						t.Fatal("凭据操作前必须禁止交互")
					}
					return tc.set
				}
				calls = append(calls, "restore")
				if allowed != tc.allowed {
					t.Fatal("必须恢复原有交互策略")
				}
				return tc.restore
			}, func() error {
				calls = append(calls, "action")
				return tc.action
			})
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("非交互策略恢复顺序错误：%v", calls)
			}
			wantError := false
			for _, expected := range []error{tc.get, tc.set, tc.action, tc.restore} {
				if expected != nil {
					wantError = true
					if !errors.Is(err, expected) {
						t.Fatalf("操作和恢复错误均须保留：%v", err)
					}
				}
			}
			if !wantError && err != nil {
				t.Fatal(err)
			}
		})
	}
}

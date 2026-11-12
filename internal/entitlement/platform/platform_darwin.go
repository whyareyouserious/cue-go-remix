//go:build darwin && cgo

package platform

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit -framework Security
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/IOKitKeys.h>
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

// The per-query flag covers modern authentication. The process-wide switch is
// also required for the legacy login keychain used by this desktop application.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static void cue_query_disallow_interaction(CFMutableDictionaryRef query) {
    CFDictionarySetValue(query, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
}
static OSStatus cue_keychain_get_interaction(Boolean *allowed) {
    return SecKeychainGetUserInteractionAllowed(allowed);
}
static OSStatus cue_keychain_set_interaction(Boolean allowed) {
    return SecKeychainSetUserInteractionAllowed(allowed);
}
#pragma clang diagnostic pop

static int cue_machine_uuid(char *buffer, size_t size) {
    io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault,
        IOServiceMatching("IOPlatformExpertDevice"));
    if (!service) return 0;
    CFTypeRef value = IORegistryEntryCreateCFProperty(service,
        CFSTR(kIOPlatformUUIDKey), kCFAllocatorDefault, 0);
    IOObjectRelease(service);
    if (!value) return 0;
    int ok = CFGetTypeID(value) == CFStringGetTypeID() &&
        CFStringGetCString((CFStringRef)value, buffer, size, kCFStringEncodingUTF8);
    CFRelease(value);
    return ok;
}

static CFMutableDictionaryRef cue_credential_query(void) {
    CFMutableDictionaryRef query = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    if (!query) return NULL;
    CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
    CFDictionarySetValue(query, kSecAttrService, CFSTR("com.cue.remix.license"));
    CFDictionarySetValue(query, kSecAttrAccount, CFSTR("authorization"));
    cue_query_disallow_interaction(query);
    return query;
}

static OSStatus cue_credential_load(void **bytes, size_t *length) {
    *bytes = NULL;
    *length = 0;
    CFMutableDictionaryRef query = cue_credential_query();
    if (!query) return errSecAllocate;
    CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
    CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
    CFTypeRef result = NULL;
    OSStatus status = SecItemCopyMatching(query, &result);
    CFRelease(query);
    if (status != errSecSuccess) return status;
    if (!result || CFGetTypeID(result) != CFDataGetTypeID()) {
        if (result) CFRelease(result);
        return errSecDecode;
    }
    CFIndex count = CFDataGetLength((CFDataRef)result);
    if (count <= 0 || count > 2560) {
        CFRelease(result);
        return errSecDecode;
    }
    *bytes = malloc((size_t)count);
    if (!*bytes) {
        CFRelease(result);
        return errSecAllocate;
    }
    memcpy(*bytes, CFDataGetBytePtr((CFDataRef)result), (size_t)count);
    *length = (size_t)count;
    CFRelease(result);
    return errSecSuccess;
}

static OSStatus cue_credential_save(const void *bytes, size_t length) {
    CFMutableDictionaryRef query = cue_credential_query();
    if (!query) return errSecAllocate;
    CFDataRef data = CFDataCreate(kCFAllocatorDefault, bytes, length);
    if (!data) {
        CFRelease(query);
        return errSecAllocate;
    }
    CFMutableDictionaryRef attributes = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    if (!attributes) {
        CFRelease(data);
        CFRelease(query);
        return errSecAllocate;
    }
    CFDictionarySetValue(attributes, kSecValueData, data);
    OSStatus status = SecItemUpdate(query, attributes);
    if (status == errSecItemNotFound) {
        CFDictionarySetValue(query, kSecValueData, data);
        status = SecItemAdd(query, NULL);
        if (status == errSecDuplicateItem) status = SecItemUpdate(query, attributes);
    }
    CFRelease(attributes);
    CFRelease(data);
    CFRelease(query);
    return status;
}

static OSStatus cue_credential_delete(void) {
    CFMutableDictionaryRef query = cue_credential_query();
    if (!query) return errSecAllocate;
    OSStatus status = SecItemDelete(query);
    CFRelease(query);
    return status;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// The legacy Keychain interaction policy is process-wide. Serialize this
// package's calls and restore the previous policy on every return path.
var keychainMu sync.Mutex

func withoutKeychainInteraction(operation string, action func() C.OSStatus) error {
	keychainMu.Lock()
	defer keychainMu.Unlock()
	return withInteractionDisabled(func() (bool, error) {
		var allowed C.Boolean
		err := keychainError("读取交互设置", C.cue_keychain_get_interaction(&allowed))
		return allowed != 0, err
	}, func(allowed bool) error {
		var value C.Boolean
		if allowed {
			value = 1
		}
		return keychainError("设置交互权限", C.cue_keychain_set_interaction(value))
	}, func() error {
		return keychainError(operation, action())
	})
}

func withInteractionDisabled(get func() (bool, error), set func(bool) error, action func() error) (err error) {
	allowed, err := get()
	if err != nil {
		return err
	}
	defer func() {
		if restoreErr := set(allowed); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
	}()
	if err = set(false); err != nil {
		return err
	}
	return action()
}

func systemMachineID() (string, error) {
	var buffer [128]C.char
	if C.cue_machine_uuid(&buffer[0], C.size_t(len(buffer))) == 0 {
		return "", errors.New("无法读取系统设备标识")
	}
	return C.GoString(&buffer[0]), nil
}

func loadCredential() ([]byte, error) {
	var data unsafe.Pointer
	var length C.size_t
	err := withoutKeychainInteraction("读取", func() C.OSStatus { return C.cue_credential_load(&data, &length) })
	defer C.free(data)
	if err != nil {
		return nil, err
	}
	return C.GoBytes(data, C.int(length)), nil
}

func saveCredential(data []byte) error {
	return withoutKeychainInteraction("保存", func() C.OSStatus {
		return C.cue_credential_save(unsafe.Pointer(&data[0]), C.size_t(len(data)))
	})
}

func deleteCredential() error {
	return withoutKeychainInteraction("删除", func() C.OSStatus { return C.cue_credential_delete() })
}

func keychainError(operation string, status C.OSStatus) error {
	if status == C.errSecSuccess {
		return nil
	}
	if status == C.errSecItemNotFound {
		return ErrNotFound
	}
	if status == C.errSecInteractionNotAllowed {
		return fmt.Errorf("%s系统授权凭据失败：钥匙串已锁定或需要访问许可，请在系统钥匙串中处理后重试（Keychain 错误 %d）", operation, int32(status))
	}
	return fmt.Errorf("%s系统授权凭据失败（Keychain 错误 %d）", operation, int32(status))
}

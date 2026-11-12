package platform

import (
	"strings"
	"testing"
)

func TestMachineDigestNormalizesUUIDAndSeparatesPlatforms(t *testing.T) {
	const id = "12345678-abcd-1234-5678-123456789abc"
	for goos, want := range map[string]string{
		"darwin":  "5a0285555d4fcd64a79f8eeea0bbd485a93abbc41162ba9bbfac8848683111ac",
		"windows": "41fb95aadf2fd0e61eeac2b226968411dcd995a409e985024e759977d6b66719",
	} {
		for _, source := range []string{id, strings.ToUpper(id), " \n{" + strings.ToUpper(id) + "}\t"} {
			got, err := hashMachineID(goos, source)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("%s 设备摘要未遵守固定命名空间或 UUID 规范化", goos)
			}
		}
	}
}

func TestMachineDigestRejectsMissingOrInvalidSource(t *testing.T) {
	for _, source := range []string{"", " \t", "invalid-source-secret", "00000000-0000-0000-0000-000000000000", "12345678-abcd-1234-5678-123456789abg", "12345678abcd-1234-5678-1234-56789abc", "{12345678-abcd-1234-5678-123456789abc"} {
		got, err := hashMachineID("darwin", source)
		if err == nil || got != "" {
			t.Fatal("无效设备标识不应生成摘要")
		}
		if source != "" && strings.Contains(err.Error(), source) {
			t.Fatal("错误泄露了原始设备标识")
		}
	}
}

func TestCredentialRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name       string
		credential Credential
		want       Credential
		wantJSON   string
	}{
		{"license", Credential{Mode: "license", LicenseKey: "  TEST-ONLY-KEY  "}, Credential{Mode: "license", LicenseKey: "TEST-ONLY-KEY"}, `{"mode":"license","licenseKey":"TEST-ONLY-KEY"}`},
		{"trial", Credential{Mode: "trial"}, Credential{Mode: "trial"}, `{"mode":"trial"}`},
		{"trialClearsKey", Credential{Mode: "trial", LicenseKey: "TEST-ONLY-OLD-KEY"}, Credential{Mode: "trial"}, `{"mode":"trial"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := encodeCredential(tc.credential)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.wantJSON {
				t.Fatal("凭据 JSON 契约错误")
			}
			got, err := decodeCredential(data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatal("凭据读取结果错误")
			}
		})
	}
}

func TestCredentialsRejectInvalidDataWithoutRevealingIt(t *testing.T) {
	const secret = "SECRET-TEST-KEY"
	for _, credential := range []Credential{
		{},
		{Mode: secret, LicenseKey: secret},
		{Mode: "license", LicenseKey: " \t"},
		{Mode: "license", LicenseKey: strings.Repeat(secret, maxCredentialBytes)},
	} {
		if _, err := encodeCredential(credential); err == nil || strings.Contains(err.Error(), secret) {
			t.Fatal("无效凭据应返回不含卡密的错误")
		}
	}
	for _, data := range []string{
		"", "null", "{}", secret,
		`{"mode":"license","licenseKey":"` + secret + `",}`,
		`{"mode":"` + secret + `","licenseKey":"` + secret + `"}`,
		`{"mode":"license","licenseKey":42}`,
		strings.Repeat(secret, maxCredentialBytes),
	} {
		if _, err := decodeCredential([]byte(data)); err == nil || strings.Contains(err.Error(), secret) {
			t.Fatal("已保存的无效凭据应返回不含卡密的错误")
		}
	}
}

func TestCredentialSizeLimitCountsEncodedBytes(t *testing.T) {
	base, err := encodeCredential(Credential{Mode: "license", LicenseKey: "a"})
	if err != nil {
		t.Fatal(err)
	}
	keySize := maxCredentialBytes - len(base) + 1
	if _, err := encodeCredential(Credential{Mode: "license", LicenseKey: strings.Repeat("a", keySize)}); err != nil {
		t.Fatal("系统凭据边界大小应允许保存")
	}
	if _, err := encodeCredential(Credential{Mode: "license", LicenseKey: strings.Repeat("a", keySize+1)}); err == nil {
		t.Fatal("超出系统凭据容量应返回错误")
	}
	if _, err := encodeCredential(Credential{Mode: "license", LicenseKey: strings.Repeat("卡", keySize)}); err == nil {
		t.Fatal("凭据大小限制必须按编码字节计算")
	}
}

//go:build integration

package entitlement

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"

	secure "cue-go-remix/internal/entitlement/platform"
)

// 仅对专门启动的本机 Java/H2 测试服务运行；不会读取系统设备或真实凭据。
func TestJavaHTTPContract(t *testing.T) {
	base := os.Getenv("CUE_JAVA_TEST_URL")
	if base == "" {
		t.Skip("需显式设置本机隔离 Java 测试服务地址 CUE_JAVA_TEST_URL")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Path != "/api" {
		t.Fatal("集成测试只允许 http://127.0.0.1:端口/api 的隔离服务")
	}
	ctx := context.Background()
	c := NewClient(base, true)
	device := "go-java-contract-fixture"
	activate := func(key, machine string) (Reply, error) {
		return c.request(ctx, "license/activate", "", map[string]string{"licenseKey": key, "machineId": machine, "clientVersion": "test"})
	}
	checkCode := func(err error, code int) {
		t.Helper()
		var api *APIError
		if !errors.As(err, &api) || api.Code != code {
			t.Fatalf("业务码应为%d，实际错误：%v", code, err)
		}
	}
	for _, key := range []string{"HTTP-FIXTURE-PRODUCT-0", "HTTP-FIXTURE-PRODUCT-1"} {
		_, err := activate(key, device)
		checkCode(err, 1006)
	}
	first, err := activate("HTTP-FIXTURE-PRODUCT-2", device)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(first, "license", true); err != nil {
		t.Fatal("Java激活协议不匹配", err)
	}
	if first.ServerTime < 1700000000000 {
		t.Fatal("Java返回的时间不是Unix毫秒")
	}
	_, err = activate("HTTP-FIXTURE-PRODUCT-2", "other-go-java-fixture")
	checkCode(err, 1007)
	second, err := activate("HTTP-FIXTURE-PRODUCT-2", device)
	if err != nil {
		t.Fatal(err)
	}
	if second.Token == first.Token || second.ExpiredAt != first.ExpiredAt {
		t.Fatal("同机恢复必须轮换Token且不延长到期")
	}
	_, err = c.request(ctx, "license/heartbeat", first.Token, map[string]string{"machineId": device})
	checkCode(err, 1202)
	pulse, err := c.request(ctx, "license/heartbeat", second.Token, map[string]string{"machineId": device})
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(pulse, "license", false); err != nil {
		t.Fatal(err)
	}
	_, err = c.request(ctx, "license/deactivate", second.Token, map[string]string{"machineId": device})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.request(ctx, "license/heartbeat", second.Token, map[string]string{"machineId": device})
	checkCode(err, 1202)

	var saved secure.Credential
	m := New(ctx, Options{BaseURL: base, AllowHTTP: true, MachineID: func() (string, error) { return device, nil }, Load: func() (secure.Credential, error) { return secure.Credential{}, secure.ErrNotFound }, Save: func(c secure.Credential) error { saved = c; return nil }, Delete: func() error { return nil }})
	defer m.Close()
	if err := m.Activate("HTTP-FIXTURE-PRODUCT-2"); err != nil {
		t.Fatal(err)
	}
	m.pulse()
	if !m.Status().Allowed || !m.Status().Saved || saved.Mode != "license" {
		t.Fatal("管理器未接受真实Java授权")
	}

	trialBody := map[string]string{"machineId": "go-java-trial-fixture", "clientVersion": "test"}
	trial, err := c.request(ctx, "trial/activate", "", trialBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(trial, "trial", true); err != nil {
		t.Fatal(err)
	}
	again, err := c.request(ctx, "trial/activate", "", trialBody)
	if err != nil {
		t.Fatal(err)
	}
	if again.ExpiredAt != trial.ExpiredAt {
		t.Fatal("试用恢复延长了到期时间")
	}
	pulse, err = c.request(ctx, "trial/heartbeat", again.Token, trialBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(pulse, "trial", false); err != nil {
		t.Fatal(err)
	}
}

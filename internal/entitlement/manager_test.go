package entitlement

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	secure "cue-go-remix/internal/entitlement/platform"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Add(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }
func fixture(t *testing.T, handler http.HandlerFunc) (*Manager, *fakeClock) {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	clock := &fakeClock{now: time.Now()}
	m := New(context.Background(), Options{BaseURL: s.URL + "/api", AllowHTTP: true, Now: clock.Now, MachineID: func() (string, error) { return "test-machine", nil }, Load: func() (secure.Credential, error) { return secure.Credential{}, secure.ErrNotFound }, Save: func(secure.Credential) error { return nil }, Delete: func() error { return nil }})
	t.Cleanup(m.Close)
	return m, clock
}
func reply(w http.ResponseWriter, mode string) {
	now := time.Now().UnixMilli()
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"token": "fixture-token", "appStatus": 2, "mode": mode, "valid": true, "serverTime": now, "expiredAt": now + 86400000}})
}
func TestNoCredentialDoesNotClaimTrialOrAuthorize(t *testing.T) {
	var calls atomic.Int32
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); reply(w, "trial") })
	if err := m.Retry(); err != nil {
		t.Fatal(err)
	}
	if m.Status().Allowed || calls.Load() != 0 {
		t.Fatal("无凭据启动不能领取试用或放行")
	}
	if _, err := m.WorkContext(); !errors.Is(err, ErrLocked) {
		t.Fatal(err)
	}
}
func TestTemporaryFailureHasBoundedGraceAndCancelsWork(t *testing.T) {
	var offline atomic.Bool
	m, clock := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if offline.Load() {
			http.Error(w, "upstream body must stay private", 503)
			return
		}
		reply(w, "license")
	})
	if err := m.Activate("CUE-test"); err != nil {
		t.Fatal(err)
	}
	work, _ := m.WorkContext()
	clock.Add(30 * time.Second)
	offline.Store(true)
	m.pulse()
	if s := m.Status(); !s.Allowed || s.State != "grace" || s.GraceSeconds != 90 {
		t.Fatal(s)
	}
	clock.Add(89 * time.Second)
	m.pulse()
	if !m.Status().Allowed {
		t.Fatal("不应提前结束宽限")
	}
	clock.Add(time.Second)
	if _, err := m.WorkContext(); err == nil {
		t.Fatal("达到120秒必须锁定")
	}
	select {
	case <-work.Done():
	default:
		t.Fatal("旧任务没有取消")
	}
	offline.Store(false)
	m.pulse()
	if !m.Status().Allowed {
		t.Fatal("联网心跳成功应恢复")
	}
	newWork, _ := m.WorkContext()
	if newWork == work || newWork.Err() != nil {
		t.Fatal("恢复必须使用新上下文，不复活旧任务")
	}
}
func TestRevocationDoesNotAutomaticallyReactivate(t *testing.T) {
	var revoked atomic.Bool
	var activations atomic.Int32
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/activate") {
			activations.Add(1)
		}
		if revoked.Load() {
			_, _ = w.Write([]byte(`{"code":1202,"message":"private"}`))
			return
		}
		reply(w, "license")
	})
	_ = m.Activate("CUE-test")
	work, _ := m.WorkContext()
	revoked.Store(true)
	m.pulse()
	if m.Status().Allowed || work.Err() == nil || m.retry {
		t.Fatal("撤销必须立即取消任务且不自动重试激活")
	}
	if activations.Load() != 1 {
		t.Fatal("自动激活绕过管理员下线")
	}
}
func TestSessionTimeoutReactivatesSameCredential(t *testing.T) {
	var calls atomic.Int32
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/heartbeat") {
			_, _ = w.Write([]byte(`{"code":1203}`))
			return
		}
		calls.Add(1)
		reply(w, "license")
	})
	_ = m.Activate("CUE-test")
	m.pulse()
	if !m.Status().Allowed || calls.Load() != 2 {
		t.Fatal("超时应重新激活同机授权")
	}
}
func TestProductAndMalformedEnvelopeNeverAuthorize(t *testing.T) {
	for _, body := range []string{`{"data":{"token":"bad"}}`, `{"code":0,"data":{"token":"bad","mode":"license","appStatus":0,"serverTime":1,"expiredAt":999999}}`, `{"code":1007,"message":"secret-card"}`} {
		t.Run(body, func(t *testing.T) {
			m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			err := m.Activate("CUE-private")
			if err == nil || m.Status().Allowed {
				t.Fatal("非法响应被接受")
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
				t.Fatal("错误泄露服务内容")
			}
		})
	}
}
func TestInvalidReplacementKeepsExistingAuthorization(t *testing.T) {
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["licenseKey"] == "wrong" {
			_, _ = w.Write([]byte(`{"code":1001}`))
			return
		}
		reply(w, "license")
	})
	_ = m.Activate("valid")
	if err := m.Activate("wrong"); err == nil {
		t.Fatal("应提示错误卡")
	}
	if !m.Status().Allowed || m.credential.LicenseKey != "valid" {
		t.Fatal("输入错误新卡不应丢弃现有授权")
	}
}
func TestForgetCancelsWorkAndDeletesCredential(t *testing.T) {
	var deleted bool
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { reply(w, "license") })
	m.opts.Delete = func() error { deleted = true; return nil }
	_ = m.Activate("CUE-test")
	work, _ := m.WorkContext()
	if err := m.Forget(); err != nil {
		t.Fatal(err)
	}
	if m.Status().Allowed || !deleted || work.Err() == nil {
		t.Fatal("清除授权必须取消任务和凭据")
	}
	if err := m.Retry(); err != nil || m.Status().Allowed {
		t.Fatal("清除后不能自动恢复")
	}
}
func TestSlowValidationCannotResurrectClosedManager(t *testing.T) {
	entered := make(chan struct{})
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/activate") {
			_, _ = io.Copy(io.Discard, r.Body)
			close(entered)
			<-r.Context().Done()
		}
	})
	done := make(chan error, 1)
	go func() { done <- m.Activate("CUE-test") }()
	<-entered
	m.Close()
	<-done
	if m.Status().Allowed {
		t.Fatal("关闭后网络结果不能恢复授权")
	}
}
func TestRememberedTrialResumesWithoutExtendingServerExpiry(t *testing.T) {
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cue/trial/activate" {
			t.Error(r.URL.Path)
		}
		reply(w, "trial")
	})
	m.opts.Load = func() (secure.Credential, error) { return secure.Credential{Mode: "trial"}, nil }
	if err := m.Retry(); err != nil {
		t.Fatal(err)
	}
	if s := m.Status(); !s.Allowed || s.Mode != "trial" || !s.Saved {
		t.Fatal(s)
	}
}
func TestCredentialsNeverFollowRedirect(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer target.Close()
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) })
	if err := m.Activate("private"); err == nil {
		t.Fatal("重定向应该拒绝")
	}
	if leaked.Load() {
		t.Fatal("凭据被转发")
	}
}

func TestCredentialStorageCannotExtendAuthorization(t *testing.T) {
	for _, remaining := range []time.Duration{10 * time.Second, 24 * time.Hour} {
		t.Run(remaining.String(), func(t *testing.T) {
			m, clock := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"token": "fixture", "appStatus": 2, "mode": "license", "serverTime": 1000, "expiredAt": 1000 + remaining.Milliseconds()}})
			})
			m.opts.Save = func(secure.Credential) error { clock.Add(121 * time.Second); return nil }
			if err := m.Activate("CUE-test"); err == nil || m.Status().Allowed {
				t.Fatal("等待保存凭据不能刷新验证时间或推迟到期")
			}
			if _, err := m.WorkContext(); err == nil {
				t.Fatal("过时的成功响应不能放行任务")
			}
		})
	}
}

func TestSameCardRejectionCancelsExistingWork(t *testing.T) {
	var frozen atomic.Bool
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if frozen.Load() {
			_, _ = io.WriteString(w, `{"code":1002}`)
			return
		}
		reply(w, "license")
	})
	_ = m.Activate("CUE-test")
	work, _ := m.WorkContext()
	frozen.Store(true)
	if err := m.Activate("CUE-test"); err == nil || m.Status().Allowed || work.Err() == nil {
		t.Fatal("当前卡明确被冻结时必须立即撤销，不能当成换卡失败保留旧授权")
	}
}

func TestExpiryPrecedesNetworkGrace(t *testing.T) {
	var offline atomic.Bool
	m, clock := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if offline.Load() {
			http.Error(w, "offline", 503)
			return
		}
		_, _ = io.WriteString(w, `{"code":0,"data":{"token":"fixture","appStatus":2,"mode":"license","serverTime":1000,"expiredAt":11000}}`)
	})
	_ = m.Activate("CUE-test")
	work, _ := m.WorkContext()
	offline.Store(true)
	clock.Add(9 * time.Second)
	m.pulse()
	if !m.Status().Allowed || m.Status().GraceSeconds != 1 {
		t.Fatal(m.Status())
	}
	clock.Add(time.Second)
	if _, err := m.WorkContext(); err == nil || work.Err() == nil {
		t.Fatal("卡到期不能继续借用网络宽限")
	}
}

func TestStorageFailureDoesNotStorePlaintextFallback(t *testing.T) {
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { reply(w, "license") })
	m.opts.Save = func(secure.Credential) error { return errors.New("private failure") }
	if err := m.Activate("CUE-test"); err != nil {
		t.Fatal(err)
	}
	s := m.Status()
	if !s.Allowed || s.Saved || s.Message == "" || strings.Contains(s.Message, "private") {
		t.Fatal(s)
	}
}

func TestSleepRequiresFreshValidationAndDoesNotReviveOldWork(t *testing.T) {
	m, clock := fixture(t, func(w http.ResponseWriter, r *http.Request) { reply(w, "license") })
	_ = m.Activate("CUE-test")
	old, _ := m.WorkContext()
	m.Revalidate()
	clock.Add(5 * time.Second)
	m.Revalidate()
	if m.Status().Allowed || old.Err() == nil {
		t.Fatal("短休眠也必须重新联网验证")
	}
	m.pulse()
	current, err := m.WorkContext()
	if err != nil || current == old || old.Err() == nil {
		t.Fatal("唤醒后须建立新授权上下文")
	}
	m.fail(&APIError{Code: 1202})
	m.Revalidate()
	m.Revalidate()
	if m.retry {
		t.Fatal("休眠不能绕过管理员撤销而自动重激活")
	}
}

func TestResponseStartedBeforeSleepCannotUnlock(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/heartbeat") {
			close(entered)
			<-release
		}
		reply(w, "license")
	})
	_ = m.Activate("CUE-test")
	done := make(chan struct{})
	go func() { m.pulse(); close(done) }()
	<-entered
	m.Revalidate()
	m.Revalidate()
	close(release)
	<-done
	if m.Status().Allowed {
		t.Fatal("休眠前发出的请求不得恢复授权")
	}
}

func TestOldFailureCannotOverrideRevalidation(t *testing.T) {
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { reply(w, "license") })
	_ = m.Activate("CUE-test")
	epoch := m.epoch
	m.Revalidate()
	m.failRequest(&APIError{Code: 1202}, epoch)
	if !m.retry || m.Status().Allowed {
		t.Fatal("旧请求错误覆盖了唤醒重新验证计划")
	}
}

func TestManualActivationWorksWhenCredentialReadIsDenied(t *testing.T) {
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { reply(w, "license") })
	m.opts.Load = func() (secure.Credential, error) { return secure.Credential{}, errors.New("access denied") }
	m.opts.Save = func(secure.Credential) error { return errors.New("access denied") }
	if err := m.Retry(); err == nil || m.Status().Allowed {
		t.Fatal("读取失败不能静默恢复")
	}
	if err := m.Activate("CUE-test"); err != nil {
		t.Fatal(err)
	}
	if !m.Status().Allowed || m.Status().Saved || m.Status().Message == "" {
		t.Fatal("手动激活应可本次使用并说明未保存")
	}
}

//go:build production

package config

import "testing"

func TestProductionDefaultUsesDeployedLicenseAPI(t *testing.T) {
	t.Setenv("CUE_LICENSE_URL", "http://127.0.0.1:1/api")
	endpoint, allowHTTP := LicenseEndpoint()
	if endpoint != "http://43.142.63.76/api" || !allowHTTP {
		t.Fatalf("默认发行包必须连接已部署入口，实际 endpoint=%q allowHTTP=%t", endpoint, allowHTTP)
	}
}

func TestProductionEndpointOverridePolicy(t *testing.T) {
	original := LicenseBaseURL
	t.Cleanup(func() { LicenseBaseURL = original })
	for _, test := range []struct {
		endpoint  string
		allowHTTP bool
	}{
		{"https://license.example.test/api", false},
		{"http://43.142.63.76/api", true},
		{"http://43.142.63.76:8088/api", false},
		{"http://other.example.test/api", false},
		{"", false},
	} {
		LicenseBaseURL = test.endpoint
		endpoint, allowHTTP := LicenseEndpoint()
		if endpoint != test.endpoint || allowHTTP != test.allowHTTP {
			t.Errorf("入口策略不匹配：endpoint=%q allowHTTP=%t", endpoint, allowHTTP)
		}
	}
}

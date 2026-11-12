//go:build production

package config

func LicenseEndpoint() (string, bool) {
	// 兼容现有部署的固定 HTTP 地址，其余发行入口仍要求 HTTPS。
	return LicenseBaseURL, LicenseBaseURL == DeployedLicenseBaseURL
}

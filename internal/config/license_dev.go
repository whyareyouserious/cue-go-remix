//go:build !production

package config

import "os"

func LicenseEndpoint() (string, bool) {
	if value := os.Getenv("CUE_LICENSE_URL"); value != "" {
		return value, true
	}
	if LicenseBaseURL != "" {
		return LicenseBaseURL, true
	}
	return DeployedLicenseBaseURL, true
}

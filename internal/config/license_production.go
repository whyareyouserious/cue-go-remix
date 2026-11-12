//go:build production

package config

func LicenseEndpoint() (string, bool) { return LicenseBaseURL, false }

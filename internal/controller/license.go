package controller

import (
	"cue-go-remix/internal/entitlement"
)

func (a *App) GetLicenseStatus() entitlement.Status { return a.runtime.GetLicenseStatus() }
func (a *App) ActivateLicense(key string) (entitlement.Status, error) {
	return a.runtime.ActivateLicense(key)
}
func (a *App) StartTrial() (entitlement.Status, error)    { return a.runtime.StartTrial() }
func (a *App) RetryLicense() (entitlement.Status, error)  { return a.runtime.RetryLicense() }
func (a *App) ForgetLicense() (entitlement.Status, error) { return a.runtime.ForgetLicense() }

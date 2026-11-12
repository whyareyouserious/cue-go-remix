package service

import (
	"cue-go-remix/internal/applink"
)

func (a *Runtime) Consent(id string, allowed bool) {
	if a.link != nil {
		a.link.Consent(id, allowed)
	}
}

func (a *Runtime) ListCallers() []applink.Caller {
	if a.link == nil {
		return []applink.Caller{}
	}
	return a.link.ListCallers()
}

func (a *Runtime) RevokeCaller(id string) error {
	if a.link == nil {
		return nil
	}
	return a.link.Revoke(id)
}

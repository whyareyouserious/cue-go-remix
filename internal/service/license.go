package service

import (
	"context"

	"cue-go-remix/internal/entitlement"
	"cue-go-remix/internal/model"
)

func (a *Runtime) GetLicenseStatus() entitlement.Status { return a.license.Status() }

func (a *Runtime) ActivateLicense(key string) (entitlement.Status, error) {
	err := a.license.Activate(key)
	return a.license.Status(), err
}

func (a *Runtime) StartTrial() (entitlement.Status, error) {
	err := a.license.Trial()
	return a.license.Status(), err
}

func (a *Runtime) RetryLicense() (entitlement.Status, error) {
	err := a.license.Retry()
	return a.license.Status(), err
}

func (a *Runtime) ForgetLicense() (entitlement.Status, error) {
	err := a.license.Forget()
	return a.license.Status(), err
}

func (a *Runtime) authorizedContext() (context.Context, error) {
	if a.license == nil {
		return nil, entitlement.ErrLocked
	}
	return a.license.WorkContext()
}

func (a *Runtime) licenseChanged(s entitlement.Status) {
	a.trace("event=license-state state=%s allowed=%t", s.State, s.Allowed)
	a.emit(model.Event{Type: "license-status", Data: s})
	if s.Allowed {
		a.mu.Lock()
		resumeNotes := !a.notesResumed && !a.closing
		a.notesResumed = true
		a.mu.Unlock()
		if resumeNotes {
			// 仅首次联网验证成功补齐上次已结束的会议；重新激活不复活旧任务。
			for _, meeting := range a.store.Meetings() {
				if meeting.EndedAt != 0 && meeting.NotesTurns < len(meeting.Turns) {
					a.makeNotes(meeting)
				}
			}
		}
	}
}
func (a *Runtime) RevalidateLicense() { a.license.Revalidate() }

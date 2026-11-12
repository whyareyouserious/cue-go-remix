package controller

import (
	"errors"

	"cue-go-remix/internal/model"
)

func (a *App) ListMeetings() []model.Meeting { return a.runtime.ListMeetings() }
func (a *App) DeleteMeeting(id string) error { return a.runtime.DeleteMeeting(id) }
func (a *App) ExportMeeting(id string) (string, error) {
	for _, meeting := range a.runtime.ListMeetings() {
		if meeting.ID != id {
			continue
		}
		path, err := a.desktop.ExportMeetingPath("cue-" + id + ".md")
		if err != nil || path == "" {
			return "", err
		}
		return a.runtime.ExportMeeting(id, path)
	}
	return "", errors.New("会议不存在")
}

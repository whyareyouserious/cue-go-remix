package controller

import (
	"errors"

	"cue-go-remix/internal/model"
)

// ListMeetings 返回按新到旧排列的会议副本，供列表和导出选择使用。
func (a *App) ListMeetings() []model.Meeting { return a.runtime.ListMeetings() }

// DeleteMeeting 按 id 删除会议；当前会议未结束、记录不存在或持久化失败时返回错误。
func (a *App) DeleteMeeting(id string) error { return a.runtime.DeleteMeeting(id) }

// ExportMeeting 按 id 导出会议为 Markdown；返回写入路径与错误，记录不存在时不创建文件。
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

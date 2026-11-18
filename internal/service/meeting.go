package service

import (
	"errors"
	"os"

	"cue-go-remix/internal/assistant"
	"cue-go-remix/internal/model"
)

// ClearTranscript 结束当前会议并清空运行态转录和页面记录；持久化失败返回错误，已保存历史保留。
func (a *Runtime) ClearTranscript() error {
	a.mu.Lock()
	live := a.listening
	a.mu.Unlock()
	if live {
		if e := a.StopListening(); e != nil {
			return e
		}
	}
	m, e := a.store.End()
	if e != nil {
		return e
	}
	if m != nil {
		a.makeNotes(*m)
	}
	a.mu.Lock()
	a.slides = []model.Slide{}
	a.mu.Unlock()
	a.status()
	return nil
}

// ListMeetings 返回按新到旧排列的会议副本，供列表和导出选择使用。
func (a *Runtime) ListMeetings() []model.Meeting {
	ms := a.store.Meetings()
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
	return ms
}

// DeleteMeeting 按 id 删除会议；当前会议未结束、记录不存在或持久化失败时返回错误。
func (a *Runtime) DeleteMeeting(id string) error { return a.store.Delete(id) }

// ExportMeeting 将 id 对应会议写入调用方选定的 path，返回该路径与写入错误；会议不存在时返回错误。
func (a *Runtime) ExportMeeting(id, path string) (string, error) {
	for _, m := range a.store.Meetings() {
		if m.ID != id {
			continue
		}
		text := "# " + m.Title + "\n\n" + m.Notes + "\n\n## 转录\n\n" + assistant.Transcript(m.Turns)
		e := os.WriteFile(path, []byte(text), 0600)
		return path, e
	}
	return "", errors.New("会议不存在")
}

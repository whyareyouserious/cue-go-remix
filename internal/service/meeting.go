package service

import (
	"errors"
	"os"

	"cue-go-remix/internal/assistant"
	"cue-go-remix/internal/model"
)

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

func (a *Runtime) ListMeetings() []model.Meeting {
	ms := a.store.Meetings()
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
	return ms
}

func (a *Runtime) DeleteMeeting(id string) error { return a.store.Delete(id) }

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

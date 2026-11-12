package repository

import (
	"errors"
	"path/filepath"
	"time"

	"cue-go-remix/internal/model"
)

func (s *Store) Meetings() []model.Meeting {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.meetings)
}

func (s *Store) Current() *model.Meeting {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.meetings {
		if m.ID == s.current {
			v := clone(m)
			return &v
		}
	}
	return nil
}

func (s *Store) persist() error { return WriteJSON(filepath.Join(s.dir, "meetings.json"), s.meetings) }

func (s *Store) Append(channel, text string) (model.Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	idx := -1
	for i := range s.meetings {
		if s.meetings[i].ID == s.current {
			idx = i
		}
	}
	if idx >= 0 {
		m := &s.meetings[idx]
		last := m.StartedAt
		if len(m.Turns) > 0 {
			last = m.Turns[len(m.Turns)-1].Time
		}
		if now-last >= 30*time.Minute.Milliseconds() {
			m.EndedAt = last
			idx = -1
		}
	}
	if idx < 0 {
		s.current = ID()
		s.meetings = append(s.meetings, model.Meeting{ID: s.current, Title: time.Now().Format("2006-01-02 15:04") + " 会议", StartedAt: now, Turns: []model.Turn{}})
		if len(s.meetings) > 50 {
			s.meetings = s.meetings[len(s.meetings)-50:]
		}
		idx = len(s.meetings) - 1
	}
	t := model.Turn{ID: ID(), Channel: channel, Text: text, Time: now}
	s.meetings[idx].Turns = append(s.meetings[idx].Turns, t)
	return t, s.persist()
}

func (s *Store) End() (*model.Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.meetings {
		if s.meetings[i].ID == s.current {
			previousEnd := s.meetings[i].EndedAt
			previousCurrent := s.current
			s.meetings[i].EndedAt = time.Now().UnixMilli()
			m := clone(s.meetings[i])
			s.current = ""
			if err := s.persist(); err != nil {
				s.meetings[i].EndedAt = previousEnd
				s.current = previousCurrent
				return nil, err
			}
			return &m, nil
		}
	}
	return nil, nil
}

func (s *Store) Notes(id, notes string, covered ...int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.meetings {
		if s.meetings[i].ID == id {
			previous := s.meetings[i].Notes
			previousCount := s.meetings[i].NotesTurns
			s.meetings[i].Notes = notes
			s.meetings[i].NotesTurns = len(s.meetings[i].Turns)
			if len(covered) > 0 {
				s.meetings[i].NotesTurns = covered[0]
			}
			if err := s.persist(); err != nil {
				s.meetings[i].Notes = previous
				s.meetings[i].NotesTurns = previousCount
				return err
			}
			return nil
		}
	}
	return errors.New("会议不存在")
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == id {
		return errors.New("请先结束当前会议再删除")
	}
	for i, m := range s.meetings {
		if m.ID == id {
			previous := s.meetings
			candidate := make([]model.Meeting, 0, len(previous)-1)
			candidate = append(candidate, previous[:i]...)
			s.meetings = append(candidate, previous[i+1:]...)
			if err := s.persist(); err != nil {
				s.meetings = previous
				return err
			}
			return nil
		}
	}
	return errors.New("会议不存在")
}

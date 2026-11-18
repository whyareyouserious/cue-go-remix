package repository

import (
	"errors"
	"path/filepath"
	"time"

	"cue-go-remix/internal/model"
)

// Meetings 返回持久化会议列表的深拷贝，调用方修改不会影响存储。
func (s *Store) Meetings() []model.Meeting {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.meetings)
}

// Current 返回当前会议副本；没有活动会议时返回 nil。
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

// persist 将当前会议列表写入 JSON；调用方必须已持有存储锁。
func (s *Store) persist() error { return WriteJSON(filepath.Join(s.dir, "meetings.json"), s.meetings) }

// Append 追加 channel 的 text 并保存；空闲超过 30 分钟时新开会议，历史最多保留 50 场，返回转录与写入错误。
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

// End 结束当前会议并持久化，返回结束后的副本；无当前会议返回 nil，写入失败恢复原状态。
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

// Notes 保存 id 的纪要 notes 及 covered 指定的覆盖条数；省略时覆盖全部转录，写入失败恢复原纪要。
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

// Delete 删除 id 对应的已结束会议；拒绝删除当前会议，持久化失败恢复原列表。
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

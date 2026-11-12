package repository

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cue-go-remix/internal/model"
)

type Store struct {
	mu       sync.Mutex
	dir      string
	settings model.Settings
	meetings []model.Meeting
	current  string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, settings: DefaultSettings(), meetings: []model.Meeting{}}
	if err := readRecover(filepath.Join(dir, "settings.json"), &s.settings); err != nil {
		return nil, fmt.Errorf("读取设置失败：%w", err)
	}
	if err := readRecover(filepath.Join(dir, "meetings.json"), &s.meetings); err != nil {
		return nil, fmt.Errorf("读取会议失败：%w", err)
	}
	if s.meetings == nil {
		s.meetings = []model.Meeting{}
	}
	now := time.Now().UnixMilli()
	for i := range s.meetings {
		m := &s.meetings[i]
		if m.EndedAt != 0 {
			continue
		}
		last := m.StartedAt
		if len(m.Turns) > 0 {
			last = m.Turns[len(m.Turns)-1].Time
		}
		if now-last < 30*time.Minute.Milliseconds() && i == len(s.meetings)-1 {
			s.current = m.ID
		} else {
			m.EndedAt = last
		}
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func ID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }

func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }

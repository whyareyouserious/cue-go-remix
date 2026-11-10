package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func DefaultSettings() model.Settings {
	return model.Settings{
		Chat:     model.Endpoint{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", SmartModel: "gpt-4o", Stream: true},
		Speech:   model.Endpoint{Provider: "openai", BaseURL: "https://api.openai.com/v1", Model: "whisper-1"},
		Language: "zh", Opacity: 0.94, ContentProtection: true, MeetingAudio: runtime.GOOS == "windows",
		SlidesInterval: 3000, WhisperModel: "base", WhisperThreads: 4,
	}
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

func readRecover[T any](path string, out *T) error {
	// Unmarshal can partially change a destination before reporting a type
	// mismatch. Decode a fresh copy of the defaults for each attempt so an
	// invalid main file cannot contaminate the recovered backup.
	defaults, err := json.Marshal(out)
	if err != nil {
		return err
	}
	decode := func(b []byte) error {
		var candidate T
		if err := json.Unmarshal(defaults, &candidate); err != nil {
			return err
		}
		if err := json.Unmarshal(b, &candidate); err != nil {
			return err
		}
		*out = candidate
		return nil
	}
	b, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !missing {
		err = decode(b)
		if err == nil {
			return nil
		}
	}
	backup, e := os.ReadFile(path + ".bak")
	if e != nil {
		if missing && errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if missing {
			return fmt.Errorf("主文件缺失，备份不可读：%w", e)
		}
		return fmt.Errorf("主文件损坏，备份不可读：%w", err)
	}
	if e = decode(backup); e != nil {
		return fmt.Errorf("主文件和备份均损坏：%w", e)
	}
	return nil
}

// WriteJSON atomically replaces one file; the previous valid version is retained.
func WriteJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".cue-write-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if old, e := os.ReadFile(path); e == nil && json.Valid(old) {
		if e = os.WriteFile(path+".bak", old, 0600); e != nil {
			return e
		}
	}
	return replaceFile(tmp, path)
}

func (s *Store) Dir() string              { return s.dir }
func (s *Store) Settings() model.Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.settings }
func Redact(v model.Settings) model.Settings {
	v.Chat.HasKey = v.Chat.APIKey != ""
	v.Chat.APIKey = ""
	v.Chat.ClearKey = false
	v.Speech.HasKey = v.Speech.APIKey != ""
	v.Speech.APIKey = ""
	v.Speech.ClearKey = false
	return v
}
func endpoint(ep, old model.Endpoint) (model.Endpoint, error) {
	ep.Provider = strings.ToLower(strings.TrimSpace(ep.Provider))
	ep.BaseURL = strings.TrimSpace(ep.BaseURL)
	if ep.BaseURL != "" {
		u, err := url.Parse(ep.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return ep, errors.New("服务地址必须为有效的 http/https 地址，不能包含用户名或密码")
		}
	}
	if ep.ClearKey {
		ep.APIKey = ""
	} else if ep.APIKey == "" && ep.Provider == old.Provider && ep.BaseURL == old.BaseURL {
		ep.APIKey = old.APIKey
	}
	ep.APIKey = strings.TrimSpace(ep.APIKey)
	ep.HasKey = ep.APIKey != ""
	ep.ClearKey = false
	if strings.ContainsAny(ep.APIKey, "\r\n") {
		return ep, errors.New("API Key 不能包含换行")
	}
	return ep, nil
}
func (s *Store) SaveSettings(v model.Settings) (model.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	v.Chat, err = endpoint(v.Chat, s.settings.Chat)
	if err != nil {
		return model.Settings{}, err
	}
	v.Speech, err = endpoint(v.Speech, s.settings.Speech)
	if err != nil {
		return model.Settings{}, err
	}
	if v.Opacity < .2 || v.Opacity > 1 {
		return model.Settings{}, errors.New("透明度须在 20% 到 100% 之间")
	}
	if len([]rune(v.AIRules)) > 2000 {
		return model.Settings{}, errors.New("回答风格最多 2000 字")
	}
	if v.SlidesInterval < 1500 {
		v.SlidesInterval = 3000
	}
	if v.SlidesInterval > 15000 {
		v.SlidesInterval = 15000
	}
	if v.WhisperThreads < 0 || v.WhisperThreads > 64 {
		return model.Settings{}, errors.New("Whisper 线程数须在 0 到 64 之间")
	}
	if v.Language == "" {
		v.Language = "zh"
	}
	// Window coordinates are maintained by the desktop process, not settings forms.
	v.WindowX = s.settings.WindowX
	v.WindowY = s.settings.WindowY
	v.WindowPositioned = s.settings.WindowPositioned
	if err = WriteJSON(filepath.Join(s.dir, "settings.json"), v); err != nil {
		return model.Settings{}, err
	}
	s.settings = v
	return Redact(v), nil
}
func (s *Store) SavePosition(x, y int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.settings
	s.settings.WindowX = x
	s.settings.WindowY = y
	s.settings.WindowPositioned = true
	if err := WriteJSON(filepath.Join(s.dir, "settings.json"), s.settings); err != nil {
		s.settings = previous
		return err
	}
	return nil
}
func ID() string         { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
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

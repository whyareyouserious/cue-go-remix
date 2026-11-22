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

// Store 管理本地 JSON 设置与会议，所有共享状态由 mu 保护，返回数据副本。
type Store struct {
	// mu 保护设置、会议列表和活动会议标识。
	mu sync.Mutex
	// dir 本机持久化根目录。
	dir string
	// settings 内部完整设置，包含密钥，向前端返回前需调用 Redact。
	settings model.Settings
	// meetings 按创建顺序保存的会议列表，最多 50 场。
	meetings []model.Meeting
	// current 当前会议 ID，空值表示没有活动会议。
	current string
}

// Open 打开 dir 下的 JSON 数据并恢复备份和活动会议；读取或恢复失败返回错误。
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, settings: DefaultSettings(), meetings: []model.Meeting{}}
	if err := readRecover(filepath.Join(dir, "settings.json"), &s.settings); err != nil {
		return nil, fmt.Errorf("读取设置失败：%w", err)
	}
	if err := migrateChannels(&s.settings); err != nil {
		return nil, fmt.Errorf("读取渠道配置失败：%w", err)
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

// Dir 返回初始化时使用的数据目录。
func (s *Store) Dir() string { return s.dir }

// ID 生成 12 字节随机数的十六进制业务标识，用于会议、转录和临时资源。
func ID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }

// clone 通过 JSON 深拷贝 v；仅用于已知可编码的项目模型。
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }

package repository

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBackupDoesNotRetainPartiallyDecodedCorruptSettings(t *testing.T) {
	dir := t.TempDir()
	// The first field is valid and would be assigned by json.Unmarshal before
	// the invalid type is returned. The older backup intentionally omits it.
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"background":"主文件残留","opacity":"invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json.bak"), []byte(`{"aiRules":"备份规则"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if got.Background != "" || got.AIRules != "备份规则" || got.Opacity != DefaultSettings().Opacity {
		t.Fatalf("备份受损坏主文件污染：%+v", got)
	}
}

func TestMissingMainFileRecoversBackup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json.bak"), []byte(`{"aiRules":"唯一备份"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Settings().AIRules != "唯一备份" {
		t.Fatal("主文件丢失时未读取备份")
	}
}

// Making the requested destination a directory produces a deterministic write
// failure even when tests run as an administrative account.
func blockFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
}

func TestPositionWriteFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Settings()
	blockFile(t, filepath.Join(dir, "settings.json"))
	if err = s.SavePosition(123, 456); err == nil {
		t.Fatal("预期写入失败")
	}
	if got := s.Settings(); !reflect.DeepEqual(got, before) {
		t.Fatal("位置保存失败仍改变内存")
	}
}

func TestMeetingMutationFailuresRollBack(t *testing.T) {
	for _, operation := range []string{"end", "notes", "delete"} {
		t.Run(operation, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Append("them", "真实会议内容"); err != nil {
				t.Fatal(err)
			}
			id := s.Current().ID
			if operation == "delete" {
				if _, err = s.End(); err != nil {
					t.Fatal(err)
				}
			}
			before := s.Meetings()
			current := s.current
			blockFile(t, filepath.Join(dir, "meetings.json"))
			switch operation {
			case "end":
				m, e := s.End()
				err = e
				if m != nil {
					t.Error("失败结束不得返回已结束会议")
				}
			case "notes":
				err = s.Notes(id, "不能保存的新纪要")
			case "delete":
				err = s.Delete(id)
			}
			if err == nil {
				t.Fatal("预期写入失败")
			}
			if !reflect.DeepEqual(before, s.Meetings()) || s.current != current {
				t.Fatalf("%s失败未回滚内存", operation)
			}
		})
	}
}

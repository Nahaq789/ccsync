package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testRemote  = "git@github.com:Nahaq789/claude-sessions.git"
	testMachine = "work-pc"
)

// t.TempDir() の下に、~/.ccsync/config.json と同じ形のパスを作る
// 例: /tmp/TestXxx123/001/.ccsync/config.json
func configPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".ccsync", "config.json")
}

func writeRaw(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSaveLoad_保存したものを読める(t *testing.T) {
	path := configPath(t)
	want := Config{Remote: testRemote, Machine: testMachine}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSave_ディレクトリがなければ作る(t *testing.T) {
	path := configPath(t) // .ccsync はまだない
	if err := Save(path, Config{Remote: testRemote, Machine: testMachine}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("ファイルができていない: %v", err)
	}
}

func TestSave_自分だけが読める権限にする(t *testing.T) {
	path := configPath(t)
	if err := Save(path, Config{Remote: testRemote, Machine: testMachine}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("ファイルの権限: got %o, want 600", got)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := di.Mode().Perm(); got != 0o700 {
		t.Errorf("ディレクトリの権限: got %o, want 700", got)
	}
}

func TestSave_JSONのキー名(t *testing.T) {
	path := configPath(t)
	if err := Save(path, Config{Remote: testRemote, Machine: testMachine}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("JSONとして読めない: %v\n%s", err, b)
	}
	if m["remote"] != testRemote || m["machine"] != testMachine {
		t.Errorf("キーは remote と machine にしてください: %s", b)
	}
}

func TestSave_上書きする(t *testing.T) {
	path := configPath(t)
	if err := Save(path, Config{Remote: testRemote, Machine: "old-pc"}); err != nil {
		t.Fatal(err)
	}
	want := Config{Remote: testRemote, Machine: "home-pc"}
	if err := Save(path, want); err != nil {
		t.Fatalf("2回目の Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoad_ファイルがなければinitを案内する(t *testing.T) {
	_, err := Load(configPath(t))
	if err == nil {
		t.Fatal("エラーになるべき")
	}
	if !strings.Contains(err.Error(), "ccsync init") {
		t.Errorf("エラーに「ccsync init」を含めてください: %v", err)
	}
}

func TestLoad_壊れたJSONはエラー(t *testing.T) {
	path := configPath(t)
	writeRaw(t, path, `{"remote": "git@github.com:Nahaq789/claude-sessions.git",`)
	if _, err := Load(path); err == nil {
		t.Error("エラーになるべき")
	}
}

func TestLoad_足りない値があればエラー(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"machine がない", `{"remote": "git@github.com:Nahaq789/claude-sessions.git"}`},
		{"remote がない", `{"machine": "work-pc"}`},
		{"machine が空", `{"remote": "git@github.com:Nahaq789/claude-sessions.git", "machine": ""}`},
		{"空のオブジェクト", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := configPath(t)
			writeRaw(t, path, tt.body)
			if _, err := Load(path); err == nil {
				t.Error("エラーになるべき")
			}
		})
	}
}

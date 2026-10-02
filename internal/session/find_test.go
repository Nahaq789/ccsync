package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testRoot = "/home/a/dev/pob"

// put は projectsDir/<dirName>/<fileName> にファイルを作り、そのパスを返す
func put(t *testing.T, pDir, dirName, fileName, content string) string {
	t.Helper()
	p := filepath.Join(pDir, dirName, fileName)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// cwdLine は cwd を持つ jsonl の1行を作る
func cwdLine(cwd string) string {
	return `{"type":"user","cwd":"` + cwd + `"}` + "\n"
}

const noCwd = `{"type":"summary","summary":"x"}` + "\n"

// find は Find を呼び、ID をキーにしたマップで結果を返す(並び順に依存しない確認用)
func find(t *testing.T, pDir string) map[string]Session {
	t.Helper()
	ss, err := Find(pDir, testRoot)
	if err != nil {
		t.Fatalf("Find がエラーを返した: %v", err)
	}
	m := map[string]Session{}
	for _, s := range ss {
		if _, dup := m[s.ID]; dup {
			t.Errorf("同じ ID が2回返っている: %s", s.ID)
		}
		m[s.ID] = s
	}
	return m
}

func TestFind_対象になるもの(t *testing.T) {
	pDir := t.TempDir()
	put(t, pDir, "-home-a-dev-pob", "root1.jsonl", cwdLine("/home/a/dev/pob"))
	put(t, pDir, "-home-a-dev-pob-server", "sub1.jsonl", cwdLine("/home/a/dev/pob/server"))
	put(t, pDir, "-home-a-dev-pob-server-api", "sub2.jsonl", cwdLine("/home/a/dev/pob/server/api"))
	put(t, pDir, "-home-a-dev-pob", "nocwd.jsonl", noCwd)

	got := find(t, pDir)

	tests := []struct {
		id, dirName, relCwd string
	}{
		{"root1", "-home-a-dev-pob", "."},
		{"sub1", "-home-a-dev-pob-server", "server"},
		{"sub2", "-home-a-dev-pob-server-api", "server/api"},
		{"nocwd", "-home-a-dev-pob", "."},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			s, ok := got[tt.id]
			if !ok {
				t.Fatalf("%s が結果に含まれていない", tt.id)
			}
			if s.RelCwd != tt.relCwd {
				t.Errorf("RelCwd = %q, want %q", s.RelCwd, tt.relCwd)
			}
			if want := filepath.Join(pDir, tt.dirName); s.Dir != want {
				t.Errorf("Dir = %q, want %q", s.Dir, want)
			}
		})
	}
	if len(got) != len(tests) {
		t.Errorf("件数 = %d, want %d: %v", len(got), len(tests), got)
	}
}

func TestFind_対象にならないもの(t *testing.T) {
	tests := []struct {
		name, dirName, fileName, content string
	}{
		{"別プロジェクト pob-old", "-home-a-dev-pob-old", "x.jsonl", cwdLine("/home/a/dev/pob-old")},
		{"別プロジェクト pob-old の配下", "-home-a-dev-pob-old-src", "x.jsonl", cwdLine("/home/a/dev/pob-old/src")},
		{"別プロジェクト pob2", "-home-a-dev-pob2", "x.jsonl", cwdLine("/home/a/dev/pob2")},
		{"無関係なプロジェクト", "-home-a-dev-other", "x.jsonl", cwdLine("/home/a/dev/other")},
		{"ルートの親", "-home-a-dev", "x.jsonl", cwdLine("/home/a/dev")},
		{"名前は完全一致だが cwd は別の場所(pob/x と pob-x の衝突)", "-home-a-dev-pob", "x.jsonl", cwdLine("/home/a/dev-pob")},
		{"cwd がなく、ディレクトリ名が前方一致", "-home-a-dev-pob-server", "x.jsonl", noCwd},
		{"jsonl 以外のファイル", "-home-a-dev-pob", "memo.txt", cwdLine("/home/a/dev/pob")},
		{"拡張子のドットがない", "-home-a-dev-pob", "memojsonl", cwdLine("/home/a/dev/pob")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pDir := t.TempDir()
			put(t, pDir, tt.dirName, tt.fileName, tt.content)
			if got := find(t, pDir); len(got) != 0 {
				t.Errorf("含まれてはいけないものが返った: %v", got)
			}
		})
	}
}

func TestFind_サブディレクトリは無視する(t *testing.T) {
	pDir := t.TempDir()
	put(t, pDir, "-home-a-dev-pob", "s1.jsonl", cwdLine("/home/a/dev/pob"))
	// <セッションID>/ の中(サブエージェントの記録など)は、セッション本体ではない
	put(t, pDir, filepath.Join("-home-a-dev-pob", "s1", "subagents"), "agent.jsonl", cwdLine("/home/a/dev/pob"))
	// 名前が .jsonl で終わるディレクトリ
	if err := os.MkdirAll(filepath.Join(pDir, "-home-a-dev-pob", "dir.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}

	got := find(t, pDir)
	if len(got) != 1 {
		t.Fatalf("件数 = %d, want 1: %v", len(got), got)
	}
	if _, ok := got["s1"]; !ok {
		t.Errorf("s1 が含まれていない: %v", got)
	}
}

func TestFind_projectsDir直下のファイルは無視する(t *testing.T) {
	pDir := t.TempDir()
	// ディレクトリではなく、同じ名前のファイルが置かれている
	if err := os.WriteFile(filepath.Join(pDir, "-home-a-dev-pob"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := find(t, pDir); len(got) != 0 {
		t.Errorf("want 0件: %v", got)
	}
}

func TestFind_IDの昇順で返る(t *testing.T) {
	pDir := t.TempDir()
	// ディレクトリの並び順と、ID の並び順が食い違うようにしておく
	put(t, pDir, "-home-a-dev-pob", "ccc.jsonl", cwdLine("/home/a/dev/pob"))
	put(t, pDir, "-home-a-dev-pob", "aaa.jsonl", cwdLine("/home/a/dev/pob"))
	put(t, pDir, "-home-a-dev-pob-server", "bbb.jsonl", cwdLine("/home/a/dev/pob/server"))

	ss, err := Find(pDir, testRoot)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range ss {
		ids = append(ids, s.ID)
	}
	want := []string{"aaa", "bbb", "ccc"}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v, want %v", ids, want)
		}
	}
}

func TestFind_ModTimeはjsonlの更新日時(t *testing.T) {
	pDir := t.TempDir()
	p := put(t, pDir, "-home-a-dev-pob", "s1.jsonl", cwdLine("/home/a/dev/pob"))
	want := time.Date(2026, 9, 30, 18, 2, 0, 0, time.UTC)
	if err := os.Chtimes(p, want, want); err != nil {
		t.Fatal(err)
	}
	s, ok := find(t, pDir)["s1"]
	if !ok {
		t.Fatal("s1 が含まれていない")
	}
	if !s.ModTime.Equal(want) {
		t.Errorf("ModTime = %v, want %v", s.ModTime, want)
	}
}

func TestFind_projectsDirが存在しない(t *testing.T) {
	ss, err := Find(filepath.Join(t.TempDir(), "nai"), testRoot)
	if err != nil {
		t.Fatalf("エラーにしない: %v", err)
	}
	if len(ss) != 0 {
		t.Errorf("want 0件: %v", ss)
	}
}

func TestFind_projectsDirが空(t *testing.T) {
	if got := find(t, t.TempDir()); len(got) != 0 {
		t.Errorf("want 0件: %v", got)
	}
}

func TestFind_読めないときはエラーを返す(t *testing.T) {
	// projectsDir としてファイルのパスを渡す(「存在しない」以外の読み込みエラー)
	p := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(p, testRoot); err == nil {
		t.Error("エラーを返すはず")
	}
}

// 要件7(優先度低):同じ ID が複数のディレクトリにある場合は、新しい方だけを残す。
// 実装したら、下の t.Skip の行を消してください。
func TestFind_同じIDは新しい方を残す(t *testing.T) {
	t.Skip("要件7を実装したら、この行を消す")

	pDir := t.TempDir()
	older := put(t, pDir, "-home-a-dev-pob", "dup.jsonl", cwdLine("/home/a/dev/pob"))
	newer := put(t, pDir, "-home-a-dev-pob-server", "dup.jsonl", cwdLine("/home/a/dev/pob/server"))
	t1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(older, t1, t1); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, t2, t2); err != nil {
		t.Fatal(err)
	}

	ss, err := Find(pDir, testRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 {
		t.Fatalf("件数 = %d, want 1: %v", len(ss), ss)
	}
	if ss[0].RelCwd != "server" {
		t.Errorf("新しい方(server)が残るはず: %v", ss[0])
	}
}

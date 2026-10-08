package syncer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nahaq789/ccsync/internal/pathnorm"
	"github.com/Nahaq789/ccsync/internal/session"
	"github.com/Nahaq789/ccsync/internal/store"
)

const (
	testRoot = "/home/a/dev/pob"
	testKey  = "github.com/nahaq789/pob"
)

// newConfig は、一時ディレクトリを使った Config を返す(git は使わない)
func newConfig(t *testing.T) Config {
	t.Helper()
	base := t.TempDir()
	return Config{
		ProjectsDir: filepath.Join(base, "projects"),
		StoreDir:    filepath.Join(base, "store"),
		Root:        testRoot,
		Key:         testKey,
		Machine:     "work-pc",
	}
}

// localBody は、手元の jsonl の中身を作る。1行目に cwd が入り、そのあとに lines が続く。
// 中には、この PC のパス(testRoot)がそのまま書かれている
func localBody(relCwd string, lines ...string) string {
	cwd := testRoot
	if relCwd != "." {
		cwd = testRoot + "/" + relCwd
	}
	body := `{"type":"user","cwd":"` + cwd + `"}` + "\n"
	for _, l := range lines {
		body += `{"type":"user","text":"` + l + `","file":"` + testRoot + `/main.go"}` + "\n"
	}
	return body
}

// putLocal は、Claude Code が保存する場所に、セッションのファイルを置く
func putLocal(t *testing.T, cfg Config, relCwd, id, body string) {
	t.Helper()
	cwd := cfg.Root
	if relCwd != "." {
		cwd = cfg.Root + "/" + relCwd
	}
	p := filepath.Join(cfg.ProjectsDir, session.EncodePath(cwd), id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// putStore は、ストアにセッションを置く。body は手元の形(パスがそのまま)で渡し、
// 中で正規化してから書く(実際の push と同じ)
func putStore(t *testing.T, cfg Config, key, relCwd, id, body string) {
	t.Helper()
	data := pathnorm.Normalize([]byte(body), cfg.Root)
	m := store.Meta{
		ID:        id,
		RelCwd:    relCwd,
		Hash:      store.Hash(data),
		Machine:   "home-pc",
		UpdatedAt: time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC),
	}
	if err := store.Write(cfg.StoreDir, key, m, data); err != nil {
		t.Fatal(err)
	}
}

func mustDiff(t *testing.T, cfg Config) []Item {
	t.Helper()
	items, err := Diff(cfg)
	if err != nil {
		t.Fatalf("Diff がエラーを返した: %v", err)
	}
	return items
}

// one は、結果が1件であることを確かめて、その1件を返す
func one(t *testing.T, items []Item) Item {
	t.Helper()
	if len(items) != 1 {
		t.Fatalf("件数 = %d, want 1: %+v", len(items), items)
	}
	return items[0]
}

func TestDiff_何もなければ空を返す(t *testing.T) {
	if items := mustDiff(t, newConfig(t)); len(items) != 0 {
		t.Errorf("want 0件: %+v", items)
	}
}

func TestDiff_手元にだけある(t *testing.T) {
	cfg := newConfig(t)
	putLocal(t, cfg, "server", "s1", localBody("server", "a"))

	it := one(t, mustDiff(t, cfg))
	if it.ID != "s1" || it.State != LocalOnly {
		t.Errorf("got ID=%q State=%d, want s1 / LocalOnly", it.ID, it.State)
	}
	if it.Local == nil {
		t.Fatal("Local が nil。手元にあるので、入っているはず")
	}
	if it.Local.ID != "s1" || it.Local.RelCwd != "server" {
		t.Errorf("Local の中身が違う: %+v", *it.Local)
	}
	if it.Remote != nil {
		t.Errorf("Remote は nil のはず(ストアにない): %+v", *it.Remote)
	}
}

func TestDiff_ストアにだけある(t *testing.T) {
	cfg := newConfig(t)
	putStore(t, cfg, cfg.Key, "server", "s1", localBody("server", "a"))

	it := one(t, mustDiff(t, cfg))
	if it.ID != "s1" || it.State != StoreOnly {
		t.Errorf("got ID=%q State=%d, want s1 / StoreOnly", it.ID, it.State)
	}
	if it.Local != nil {
		t.Errorf("Local は nil のはず(手元にない): %+v", *it.Local)
	}
	if it.Remote == nil {
		t.Fatal("Remote が nil。ストアにあるので、入っているはず")
	}
	if it.Remote.ID != "s1" || it.Remote.RelCwd != "server" || it.Remote.Machine != "home-pc" {
		t.Errorf("Remote の中身が違う: %+v", *it.Remote)
	}
}

func TestDiff_両方にある(t *testing.T) {
	tests := []struct {
		name   string
		local  string
		stored string
		want   State
	}{
		// 手元の中身にはこの PC のパスが、ストアにはプレースホルダーが入っている。
		// 正規化してから比べないと、同じ内容でも Synced にならない
		{"同じ内容", localBody(".", "a", "b"), localBody(".", "a", "b"), Synced},
		{"手元の方が続きがある", localBody(".", "a", "b", "c"), localBody(".", "a", "b"), LocalAhead},
		{"ストアの方が続きがある", localBody(".", "a"), localBody(".", "a", "b"), StoreAhead},
		{"別々に進んでいる", localBody(".", "a", "x"), localBody(".", "a", "y"), Conflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newConfig(t)
			putLocal(t, cfg, ".", "s1", tt.local)
			putStore(t, cfg, cfg.Key, ".", "s1", tt.stored)

			it := one(t, mustDiff(t, cfg))
			if it.State != tt.want {
				t.Errorf("State = %d, want %d", it.State, tt.want)
			}
			if it.Local == nil || it.Remote == nil {
				t.Errorf("両方にあるので、Local と Remote の両方が入っているはず: Local=%v Remote=%v", it.Local, it.Remote)
			}
		})
	}
}

func TestDiff_ハッシュの記録が古くても中身で判定する(t *testing.T) {
	cfg := newConfig(t)
	body := localBody(".", "a")
	putLocal(t, cfg, ".", "s1", body)

	// meta.json の Hash だけが、実際の中身と食い違っている
	data := pathnorm.Normalize([]byte(body), cfg.Root)
	m := store.Meta{ID: "s1", RelCwd: ".", Hash: "wrong", Machine: "home-pc"}
	if err := store.Write(cfg.StoreDir, cfg.Key, m, data); err != nil {
		t.Fatal(err)
	}

	if it := one(t, mustDiff(t, cfg)); it.State != Synced {
		t.Errorf("中身は同じなので Synced のはず: State = %d", it.State)
	}
}

func TestDiff_全部の状態がIDの昇順で返る(t *testing.T) {
	cfg := newConfig(t)
	// ストアにだけあるものが、ID の順では先頭に来るようにしてある
	putStore(t, cfg, cfg.Key, ".", "a-store-only", localBody(".", "a"))
	putLocal(t, cfg, ".", "f-local-only", localBody(".", "a"))

	putLocal(t, cfg, ".", "e-conflict", localBody(".", "a", "x"))
	putStore(t, cfg, cfg.Key, ".", "e-conflict", localBody(".", "a", "y"))

	putLocal(t, cfg, "server", "d-store-ahead", localBody("server", "a"))
	putStore(t, cfg, cfg.Key, "server", "d-store-ahead", localBody("server", "a", "b"))

	putLocal(t, cfg, ".", "c-local-ahead", localBody(".", "a", "b"))
	putStore(t, cfg, cfg.Key, ".", "c-local-ahead", localBody(".", "a"))

	putLocal(t, cfg, ".", "b-synced", localBody(".", "a"))
	putStore(t, cfg, cfg.Key, ".", "b-synced", localBody(".", "a"))

	want := []struct {
		id    string
		state State
	}{
		{"a-store-only", StoreOnly},
		{"b-synced", Synced},
		{"c-local-ahead", LocalAhead},
		{"d-store-ahead", StoreAhead},
		{"e-conflict", Conflict},
		{"f-local-only", LocalOnly},
	}
	items := mustDiff(t, cfg)
	if len(items) != len(want) {
		t.Fatalf("件数 = %d, want %d: %+v", len(items), len(want), items)
	}
	for i, w := range want {
		if items[i].ID != w.id || items[i].State != w.state {
			t.Errorf("[%d] got ID=%q State=%d, want ID=%q State=%d", i, items[i].ID, items[i].State, w.id, w.state)
		}
	}
}

func TestDiff_LocalとRemoteはそれぞれ自分のセッションを指す(t *testing.T) {
	cfg := newConfig(t)
	for _, id := range []string{"s1", "s2", "s3"} {
		putLocal(t, cfg, ".", id, localBody(".", id))
		putStore(t, cfg, cfg.Key, ".", id, localBody(".", id))
	}
	for _, it := range mustDiff(t, cfg) {
		if it.Local == nil || it.Remote == nil {
			t.Fatalf("%s: Local か Remote が nil", it.ID)
		}
		if it.Local.ID != it.ID {
			t.Errorf("%s の Local が、別のセッション(%s)を指している", it.ID, it.Local.ID)
		}
		if it.Remote.ID != it.ID {
			t.Errorf("%s の Remote が、別のセッション(%s)を指している", it.ID, it.Remote.ID)
		}
	}
}

func TestDiff_関係のないものは含めない(t *testing.T) {
	cfg := newConfig(t)
	putLocal(t, cfg, ".", "mine", localBody(".", "a"))

	// 別のプロジェクトの、手元のセッション
	other := filepath.Join(cfg.ProjectsDir, session.EncodePath("/home/a/dev/lovers"), "other-local.jsonl")
	if err := os.MkdirAll(filepath.Dir(other), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte(`{"cwd":"/home/a/dev/lovers"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 別のリポジトリの、ストアのセッション
	putStore(t, cfg, "github.com/nahaq789/lovers", ".", "other-store", localBody(".", "a"))

	it := one(t, mustDiff(t, cfg))
	if it.ID != "mine" {
		t.Errorf("got %q, want mine", it.ID)
	}
}

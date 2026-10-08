package syncer

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nahaq789/ccsync/internal/pathnorm"
	"github.com/Nahaq789/ccsync/internal/session"
	"github.com/Nahaq789/ccsync/internal/store"
)

// ---------- git を使うテストの準備 ----------

// isolateGit は、テスト中の git が、手元の設定(~/.gitconfig)の影響を受けないようにする
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "ccsync-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "ccsync-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRemote は、GitHub の代わりになるリポジトリを手元に作る(README.md が1つ入った状態)
func newRemote(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	gitRun(t, base, "init", "--bare", "-b", "main", remote)
	seed := filepath.Join(base, "seed")
	gitRun(t, base, "clone", remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# claude-sessions\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, seed, "add", "-A")
	gitRun(t, seed, "commit", "-m", "init")
	gitRun(t, seed, "push", "origin", "HEAD:main")
	return remote
}

// newPC は、remote を共有する 1 台の PC を作る。StoreDir は clone 済み
func newPC(t *testing.T, remote, machine string) Config {
	t.Helper()
	cfg := newConfig(t)
	cfg.Machine = machine
	if err := store.Clone(remote, cfg.StoreDir); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	return cfg
}

// remoteHead は、リモートの最新コミットの ID を返す
func remoteHead(t *testing.T, remote string) string {
	t.Helper()
	return gitRun(t, remote, "rev-parse", "main")
}

// remoteSubject は、リモートの最新コミットのメッセージを返す
func remoteSubject(t *testing.T, remote string) string {
	t.Helper()
	return gitRun(t, remote, "log", "-1", "--format=%s", "main")
}

// readStore は、別の PC として clone し直して、ストアのセッションを読む(リモートに届いたかの確認用)
func readStore(t *testing.T, remote, id string) (store.Meta, []byte) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "check")
	if err := store.Clone(remote, other); err != nil {
		t.Fatal(err)
	}
	m, data, err := store.Read(other, testKey, id)
	if err != nil {
		t.Fatalf("リモートから %s を読めない: %v", id, err)
	}
	return m, data
}

func mustPush(t *testing.T, cfg Config) Result {
	t.Helper()
	res, err := Push(cfg)
	if err != nil {
		t.Fatalf("Push がエラーを返した: %v", err)
	}
	return res
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------- Push ----------

func TestPush_手元にだけあるものを送る(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	cfg := newPC(t, remote, "work-pc")
	body := localBody("server", "a", "b")
	putLocal(t, cfg, "server", "s1", body)

	// 元のファイルの更新日時を、決まった値にしておく
	mtime := time.Date(2026, 10, 1, 12, 34, 56, 0, time.UTC)
	p := filepath.Join(cfg.ProjectsDir, session.EncodePath(cfg.Root+"/server"), "s1.jsonl")
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	res := mustPush(t, cfg)
	if !sameIDs(res.Done, []string{"s1"}) {
		t.Errorf("Done = %v, want [s1]", res.Done)
	}
	if len(res.Conflicts) != 0 {
		t.Errorf("Conflicts = %v, want []", res.Conflicts)
	}

	m, data := readStore(t, remote, "s1")
	want := pathnorm.Normalize([]byte(body), cfg.Root)
	if !bytes.Equal(data, want) {
		t.Errorf("ストアの中身が、正規化した手元の中身と違う\n got:  %q\n want: %q", data, want)
	}
	if bytes.Contains(data, []byte(cfg.Root)) {
		t.Errorf("ストアに、この PC のパスが残っている: %q", data)
	}
	if m.ID != "s1" || m.RelCwd != "server" || m.Machine != "work-pc" {
		t.Errorf("Meta の中身が違う: %+v", m)
	}
	if m.Hash != store.Hash(data) {
		t.Errorf("Meta.Hash が、中身のハッシュと一致しない")
	}
	if !m.UpdatedAt.Equal(mtime) {
		t.Errorf("UpdatedAt = %v, want %v(元のファイルの更新日時)", m.UpdatedAt, mtime)
	}
}

func TestPush_コミットメッセージにPCの名前と件数が入る(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	cfg := newPC(t, remote, "work-pc")
	putLocal(t, cfg, ".", "s1", localBody(".", "a"))
	putLocal(t, cfg, ".", "s2", localBody(".", "b"))

	mustPush(t, cfg)
	if got, want := remoteSubject(t, remote), "ccsync: work-pc から 2 件を push"; got != want {
		t.Errorf("コミットメッセージ = %q, want %q", got, want)
	}
}

func TestPush_状態ごとの扱い(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	home := newPC(t, remote, "home-pc")
	work := newPC(t, remote, "work-pc")

	// 自宅の PC で、5件を push しておく
	for _, id := range []string{"b-synced", "c-local-ahead", "d-store-ahead", "e-conflict", "f-store-only"} {
		putLocal(t, home, ".", id, localBody(".", "a"))
	}
	putLocal(t, home, ".", "d-store-ahead", localBody(".", "a", "home"))
	putLocal(t, home, ".", "e-conflict", localBody(".", "a", "home"))
	mustPush(t, home)

	// 会社の PC の手元
	putLocal(t, work, ".", "a-local-only", localBody(".", "a"))
	putLocal(t, work, ".", "b-synced", localBody(".", "a"))
	putLocal(t, work, ".", "c-local-ahead", localBody(".", "a", "work"))
	putLocal(t, work, ".", "d-store-ahead", localBody(".", "a"))
	putLocal(t, work, ".", "e-conflict", localBody(".", "a", "work"))

	res := mustPush(t, work)
	if want := []string{"a-local-only", "c-local-ahead"}; !sameIDs(res.Done, want) {
		t.Errorf("Done = %v, want %v", res.Done, want)
	}
	if want := []string{"e-conflict"}; !sameIDs(res.Conflicts, want) {
		t.Errorf("Conflicts = %v, want %v", res.Conflicts, want)
	}

	// 送ったもの
	_, data := readStore(t, remote, "c-local-ahead")
	if !bytes.Contains(data, []byte("work")) {
		t.Error("c-local-ahead が、会社の PC の内容になっていない")
	}
	// 送らなかったもの(ストアの方が新しい、競合している)は、自宅の PC の内容のまま
	for _, id := range []string{"d-store-ahead", "e-conflict"} {
		m, data := readStore(t, remote, id)
		if !bytes.Contains(data, []byte("home")) || m.Machine != "home-pc" {
			t.Errorf("%s が、上書きされている", id)
		}
	}
	// 手元にないものは、ストアに残っている
	readStore(t, remote, "f-store-only")
}

func TestPush_送るものがなければコミットしない(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	cfg := newPC(t, remote, "work-pc")
	putLocal(t, cfg, ".", "s1", localBody(".", "a"))
	mustPush(t, cfg)
	before := remoteHead(t, remote)

	res := mustPush(t, cfg) // 2回目。何も変わっていない
	if len(res.Done) != 0 || len(res.Conflicts) != 0 {
		t.Errorf("何もしないはず: %+v", res)
	}
	if after := remoteHead(t, remote); after != before {
		t.Error("送るものがないのに、コミットが作られている")
	}
}

func TestPush_最初に最新を取り込んでから送る(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	home := newPC(t, remote, "home-pc")
	work := newPC(t, remote, "work-pc") // この時点で clone したきり、取り込んでいない

	putLocal(t, home, ".", "from-home", localBody(".", "a"))
	mustPush(t, home)

	// 会社の PC は、古いストアのまま Push を呼ぶ。Push の中で取り込むので、失敗しない
	putLocal(t, work, ".", "from-work", localBody(".", "b"))
	res := mustPush(t, work)
	if !sameIDs(res.Done, []string{"from-work"}) {
		t.Errorf("Done = %v, want [from-work]", res.Done)
	}
	// 両方がリモートに残っている
	readStore(t, remote, "from-home")
	readStore(t, remote, "from-work")
}

func TestPush_何もなければ何もしない(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	cfg := newPC(t, remote, "work-pc")
	before := remoteHead(t, remote)

	res := mustPush(t, cfg)
	if len(res.Done) != 0 || len(res.Conflicts) != 0 {
		t.Errorf("何もしないはず: %+v", res)
	}
	if after := remoteHead(t, remote); after != before {
		t.Error("何もないのに、コミットが作られている")
	}
}

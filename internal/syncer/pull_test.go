package syncer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nahaq789/ccsync/internal/session"
)

// 自宅の PC は、別の PC(testRoot = /home/a/dev/pob)とは別の場所に POB を置いている
const homeRoot = "/home/b/work/pob-home"

// newHomePC は、ルートの場所が別の PC と違う、2 台目の PC を作る
func newHomePC(t *testing.T, remote string) Config {
	t.Helper()
	cfg := newPC(t, remote, "home-pc")
	cfg.Root = homeRoot
	return cfg
}

// asHome は、別の PC で書いた中身を、自宅の PC のパスに置き換えたもの(pull 後に期待する中身)
func asHome(body string) string {
	return strings.ReplaceAll(body, testRoot, homeRoot)
}

// localFile は、cfg の PC で、relCwd で始めたセッションのファイルのパスを返す
func localFile(cfg Config, relCwd, id string) string {
	cwd := cfg.Root
	if relCwd != "." {
		cwd = cfg.Root + "/" + relCwd
	}
	return filepath.Join(cfg.ProjectsDir, session.EncodePath(cwd), id+".jsonl")
}

func readText(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%s が読めない: %v", p, err)
	}
	return string(b)
}

func mustPull(t *testing.T, cfg Config) Result {
	t.Helper()
	res, err := Pull(cfg)
	if err != nil {
		t.Fatalf("Pull がエラーを返した: %v", err)
	}
	return res
}

// ---------- Pull ----------

func TestPull_ストアにだけあるものを取り込む(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newPC(t, remote, "work-pc")
	home := newHomePC(t, remote)

	body := localBody("server", "a", "b")
	putLocal(t, work, "server", "s1", body)
	mtime := time.Date(2026, 10, 1, 12, 34, 56, 0, time.UTC)
	if err := os.Chtimes(localFile(work, "server", "s1"), mtime, mtime); err != nil {
		t.Fatal(err)
	}
	mustPush(t, work)

	res := mustPull(t, home)
	if !sameIDs(res.Done, []string{"s1"}) {
		t.Errorf("Done = %v, want [s1]", res.Done)
	}
	if len(res.Conflicts) != 0 {
		t.Errorf("Conflicts = %v, want []", res.Conflicts)
	}

	// 自宅の PC の、server で始めたセッションの場所に置かれている
	p := localFile(home, "server", "s1")
	got := readText(t, p)
	if want := asHome(body); got != want {
		t.Errorf("中身が、自宅の PC のパスになっていない\n got:  %q\n want: %q", got, want)
	}
	if strings.Contains(got, testRoot) {
		t.Errorf("別の PC のパスが残っている: %q", got)
	}
	if strings.Contains(got, "\x01") {
		t.Errorf("プレースホルダーが残っている: %q", got)
	}

	// 更新日時は、別の PC の元のファイルと同じ
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("更新日時 = %v, want %v", info.ModTime(), mtime)
	}

	// 取り込んだあとは、Find で見つかり、同期済みになっている
	ss, err := session.Find(home.ProjectsDir, home.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].ID != "s1" || ss[0].RelCwd != "server" {
		t.Errorf("Find の結果 = %+v, want s1 / server", ss)
	}
	if it := one(t, mustDiff(t, home)); it.State != Synced {
		t.Errorf("取り込んだあとの状態 = %d, want Synced", it.State)
	}
}

func TestPull_ルートで始めたセッションはルートの場所に置く(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newPC(t, remote, "work-pc")
	home := newHomePC(t, remote)

	putLocal(t, work, ".", "s1", localBody(".", "a"))
	mustPush(t, work)
	mustPull(t, home)

	if _, err := os.Stat(localFile(home, ".", "s1")); err != nil {
		t.Errorf("ルートの場所に置かれていない: %v", err)
	}
}

func TestPull_ストアの方が新しければ手元を上書きする(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newPC(t, remote, "work-pc")
	home := newHomePC(t, remote)

	putLocal(t, work, "server", "s1", localBody("server", "a"))
	mustPush(t, work)
	mustPull(t, home)

	// 別の PC で続きを話して、push する
	longer := localBody("server", "a", "b", "c")
	putLocal(t, work, "server", "s1", longer)
	mustPush(t, work)

	res := mustPull(t, home)
	if !sameIDs(res.Done, []string{"s1"}) {
		t.Errorf("Done = %v, want [s1]", res.Done)
	}
	if got := readText(t, localFile(home, "server", "s1")); got != asHome(longer) {
		t.Errorf("上書きされていない\n got:  %q\n want: %q", got, asHome(longer))
	}
}

func TestPull_状態ごとの扱い(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newPC(t, remote, "work-pc")
	home := newPC(t, remote, "home-pc") // 比べやすいように、ここだけ同じルートにする

	// 別の PC で push しておく
	putLocal(t, work, ".", "a-store-only", localBody(".", "a"))
	putLocal(t, work, ".", "b-synced", localBody(".", "a"))
	putLocal(t, work, ".", "c-local-ahead", localBody(".", "a"))
	putLocal(t, work, ".", "d-store-ahead", localBody(".", "a", "work"))
	putLocal(t, work, ".", "e-conflict", localBody(".", "a", "work"))
	mustPush(t, work)

	// 自宅の PC の手元
	putLocal(t, home, ".", "b-synced", localBody(".", "a"))
	putLocal(t, home, ".", "c-local-ahead", localBody(".", "a", "home"))
	putLocal(t, home, ".", "d-store-ahead", localBody(".", "a"))
	putLocal(t, home, ".", "e-conflict", localBody(".", "a", "home"))
	putLocal(t, home, ".", "f-local-only", localBody(".", "a", "home"))

	res := mustPull(t, home)
	if want := []string{"a-store-only", "d-store-ahead"}; !sameIDs(res.Done, want) {
		t.Errorf("Done = %v, want %v", res.Done, want)
	}
	if want := []string{"e-conflict"}; !sameIDs(res.Conflicts, want) {
		t.Errorf("Conflicts = %v, want %v", res.Conflicts, want)
	}

	// 取り込んだもの
	if got := readText(t, localFile(home, ".", "d-store-ahead")); !strings.Contains(got, "work") {
		t.Error("d-store-ahead が、取り込まれていない")
	}
	// 取り込まなかったもの(手元の方が新しい、競合している、手元にだけある)は、自宅の PC の内容のまま
	for _, id := range []string{"c-local-ahead", "e-conflict", "f-local-only"} {
		got := readText(t, localFile(home, ".", id))
		if !strings.Contains(got, "home") || strings.Contains(got, "work") {
			t.Errorf("%s が、上書きされている: %q", id, got)
		}
	}
}

func TestPull_最初に最新を取り込む(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	home := newHomePC(t, remote) // 先に clone したきり、取り込んでいない
	work := newPC(t, remote, "work-pc")

	putLocal(t, work, ".", "s1", localBody(".", "a"))
	mustPush(t, work)

	res := mustPull(t, home)
	if !sameIDs(res.Done, []string{"s1"}) {
		t.Errorf("Done = %v, want [s1](Pull の中で最新を取り込んでいない)", res.Done)
	}
}

func TestPull_コミットは作らない(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newPC(t, remote, "work-pc")
	home := newHomePC(t, remote)
	putLocal(t, work, ".", "s1", localBody(".", "a"))
	mustPush(t, work)
	before := remoteHead(t, remote)

	mustPull(t, home)
	if after := remoteHead(t, remote); after != before {
		t.Error("Pull で、リモートにコミットが作られている")
	}
}

func TestPull_何もなければ何もしない(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	home := newHomePC(t, remote)

	res := mustPull(t, home)
	if len(res.Done) != 0 || len(res.Conflicts) != 0 {
		t.Errorf("何もしないはず: %+v", res)
	}
	if _, err := os.Stat(home.ProjectsDir); err == nil {
		entries, _ := os.ReadDir(home.ProjectsDir)
		if len(entries) != 0 {
			t.Errorf("何もないのに、ファイルが作られている: %v", entries)
		}
	}
}

// ---------- 往復 ----------

// 別 PC で話す → push → 自宅で pull → 自宅で続きを話す → push → 別で pull
func TestPushPull_別と自宅を往復する(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newPC(t, remote, "work-pc")
	home := newHomePC(t, remote)

	// 別 PC で話して、push
	first := localBody("server", "別で話した")
	putLocal(t, work, "server", "s1", first)
	if res := mustPush(t, work); !sameIDs(res.Done, []string{"s1"}) {
		t.Fatalf("別PC の push: Done = %v", res.Done)
	}

	// 自宅で pull
	if res := mustPull(t, home); !sameIDs(res.Done, []string{"s1"}) {
		t.Fatalf("自宅の pull: Done = %v", res.Done)
	}
	homeFile := localFile(home, "server", "s1")

	// 自宅で続きを話す(Claude Code が末尾に追記するのと同じ)
	f, err := os.OpenFile(homeFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"type":"user","text":"自宅で続きを話した","file":"` + homeRoot + `/main.go"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if res := mustPush(t, home); !sameIDs(res.Done, []string{"s1"}) {
		t.Fatalf("自宅の push: Done = %v", res.Done)
	}

	// 別 PC で pull
	if res := mustPull(t, work); !sameIDs(res.Done, []string{"s1"}) {
		t.Fatalf("別の pull: Done = %v", res.Done)
	}
	got := readText(t, localFile(work, "server", "s1"))
	want := first + `{"type":"user","text":"自宅で続きを話した","file":"` + testRoot + `/main.go"}` + "\n"
	if got != want {
		t.Errorf("別の PC の中身が違う\n got:  %q\n want: %q", got, want)
	}
	if strings.Contains(got, homeRoot) {
		t.Error("自宅の PC のパスが残っている")
	}

	// 最後は、両方とも同期済み
	for name, cfg := range map[string]Config{"別": work, "自宅": home} {
		if it := one(t, mustDiff(t, cfg)); it.State != Synced {
			t.Errorf("%s の PC の状態 = %d, want Synced", name, it.State)
		}
	}
}

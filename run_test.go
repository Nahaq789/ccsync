package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nahaq789/ccsync/internal/config"
	"github.com/Nahaq789/ccsync/internal/session"
)

// このファイルのテストは、main_test.go のヘルパー(isolateGit、gitRun、tempDir、newProject)を使う

const projectOrigin = "git@github.com:Nahaq789/pob.git"

// newStoreRemote は、GitHub の claude-sessions の代わりになるリポジトリを作る(README.md が1つ入った状態)
func newStoreRemote(t *testing.T) string {
	t.Helper()
	base := tempDir(t)
	remote := filepath.Join(base, "claude-sessions.git")
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

// pc は、1台のPCを表す
//
//	home: /tmp/.../001          (~ の代わり)
//	root: /tmp/.../002/pob      (POB の作業リポジトリ)
type pc struct {
	t    *testing.T
	home string
	root string
}

func newPC(t *testing.T) *pc {
	t.Helper()
	return &pc{t: t, home: tempDir(t), root: newProject(t, projectOrigin)}
}

// ccsync は、このPCの dir で「ccsync <args...>」を実行したときの、出力とエラーを返す
func (p *pc) ccsync(dir string, args ...string) (string, error) {
	p.t.Helper()
	var out bytes.Buffer
	err := run(args, p.home, dir, &out)
	return out.String(), err
}

func (p *pc) mustCcsync(args ...string) string {
	p.t.Helper()
	out, err := p.ccsync(p.root, args...)
	if err != nil {
		p.t.Fatalf("ccsync %s: %v\n出力:\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (p *pc) initWith(remote, machine string) {
	p.t.Helper()
	p.mustCcsync("init", "--machine", machine, remote)
}

// sessionPath は、ルートで始めたセッションの置き場所
// 例: /tmp/.../001/.claude/projects/-tmp-...-002-pob/s1.jsonl
func (p *pc) sessionPath(id string) string {
	return filepath.Join(p.home, ".claude", "projects", session.EncodePath(p.root), id+".jsonl")
}

// talk は、セッションに1行書き足す(Claude Code で会話を続けたのと同じ)
func (p *pc) talk(id, text string) {
	p.t.Helper()
	path := p.sessionPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		p.t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		p.t.Fatal(err)
	}
	defer f.Close()
	line := `{"cwd":"` + p.root + `","text":"` + text + `"}` + "\n"
	if _, err := f.WriteString(line); err != nil {
		p.t.Fatal(err)
	}
}

func (p *pc) read(id string) string {
	p.t.Helper()
	b, err := os.ReadFile(p.sessionPath(id))
	if err != nil {
		p.t.Fatalf("セッション %s を読めない: %v", id, err)
	}
	return string(b)
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("出力に %q が含まれていない\n出力:\n%s", w, out)
		}
	}
}

// ---------- 引数 ----------

func TestRun_引数がなければエラー(t *testing.T) {
	if err := run(nil, t.TempDir(), t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Error("エラーになるべき")
	}
}

func TestRun_知らないコマンドはエラー(t *testing.T) {
	if err := run([]string{"hoge"}, t.TempDir(), t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Error("エラーになるべき")
	}
}

// ---------- init ----------

func TestInit_設定を保存してストアをcloneする(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	a := newPC(t)
	a.initWith(remote, "work-pc")

	got, err := config.Load(filepath.Join(a.home, ".ccsync", "config.json"))
	if err != nil {
		t.Fatalf("設定を読めない: %v", err)
	}
	want := config.Config{Remote: remote, Machine: "work-pc"}
	if got != want {
		t.Errorf("設定: got %+v, want %+v", got, want)
	}
	if _, err := os.Stat(filepath.Join(a.home, ".ccsync", "store", ".git")); err != nil {
		t.Errorf("~/.ccsync/store に clone されていない: %v", err)
	}
}

func TestInit_引数が足りなければエラーで何も作らない(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	tests := []struct {
		name string
		args []string
	}{
		{"--machine がない", []string{"init", remote}},
		{"URL がない", []string{"init", "--machine", "work-pc"}},
		{"何もない", []string{"init"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newPC(t)
			if _, err := a.ccsync(a.root, tt.args...); err == nil {
				t.Error("エラーになるべき")
			}
			if _, err := os.Stat(filepath.Join(a.home, ".ccsync", "config.json")); err == nil {
				t.Error("設定ファイルを作ってはいけない")
			}
		})
	}
}

func TestInit_cloneできなければ設定を保存しない(t *testing.T) {
	isolateGit(t)
	a := newPC(t)
	missing := filepath.Join(tempDir(t), "no-such.git")
	if _, err := a.ccsync(a.root, "init", "--machine", "work-pc", missing); err == nil {
		t.Fatal("エラーになるべき")
	}
	if _, err := os.Stat(filepath.Join(a.home, ".ccsync", "config.json")); err == nil {
		t.Error("clone に失敗したのに設定ファイルができている(clone してから保存してください)")
	}
}

// ---------- push / pull / status の前提 ----------

func TestSync_initしていなければinitを案内する(t *testing.T) {
	isolateGit(t)
	a := newPC(t)
	for _, cmd := range []string{"push", "pull", "status"} {
		_, err := a.ccsync(a.root, cmd)
		if err == nil {
			t.Errorf("%s: エラーになるべき", cmd)
			continue
		}
		if !strings.Contains(err.Error(), "ccsync init") {
			t.Errorf("%s: エラーに「ccsync init」を含めてください: %v", cmd, err)
		}
	}
}

func TestSync_リポジトリの外ならエラー(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	a := newPC(t)
	a.initWith(remote, "work-pc")
	outside := tempDir(t)
	for _, cmd := range []string{"push", "pull", "status"} {
		if _, err := a.ccsync(outside, cmd); err == nil {
			t.Errorf("%s: エラーになるべき", cmd)
		}
	}
}

func TestSync_ストアが消えていてもcloneし直す(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	a := newPC(t)
	a.initWith(remote, "work-pc")
	if err := os.RemoveAll(filepath.Join(a.home, ".ccsync", "store")); err != nil {
		t.Fatal(err)
	}
	a.talk("s1", "hello")
	out := a.mustCcsync("push")
	mustContain(t, out, "s1")
}

// ---------- push / pull ----------

func TestPushPull_会社から自宅へ(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	work, home := newPC(t), newPC(t)
	work.initWith(remote, "work-pc")
	home.initWith(remote, "home-pc")

	work.talk("s1", "ログイン画面を作りたい")
	out := work.mustCcsync("push")
	mustContain(t, out, "push", "1件", "s1")

	out = home.mustCcsync("pull")
	mustContain(t, out, "pull", "1件", "s1")

	got := home.read("s1")
	if !strings.Contains(got, `"cwd":"`+home.root+`"`) {
		t.Errorf("自宅のパスに書き換わっていない:\n%s", got)
	}
	if strings.Contains(got, work.root) {
		t.Errorf("会社のパスが残っている:\n%s", got)
	}
}

func TestPushPull_サブディレクトリで実行しても同じ(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	work, home := newPC(t), newPC(t)
	work.initWith(remote, "work-pc")
	home.initWith(remote, "home-pc")

	work.talk("s1", "hello")
	if _, err := work.ccsync(filepath.Join(work.root, "server", "api"), "push"); err != nil {
		t.Fatalf("push: %v", err)
	}
	if _, err := home.ccsync(filepath.Join(home.root, "server"), "pull"); err != nil {
		t.Fatalf("pull: %v", err)
	}
	home.read("s1") // ルートの場所に取り込まれていること
}

func TestPushPull_対象がなくてもエラーにしない(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	a := newPC(t)
	a.initWith(remote, "work-pc")
	for _, cmd := range []string{"push", "pull"} {
		out, err := a.ccsync(a.root, cmd)
		if err != nil {
			t.Errorf("%s: %v", cmd, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Errorf("%s: 対象がないことを表示してください", cmd)
		}
	}
}

func TestPush_競合があればIDを表示してエラーにする(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	work, home := newPC(t), newPC(t)
	work.initWith(remote, "work-pc")
	home.initWith(remote, "home-pc")

	work.talk("s1", "start")
	work.mustCcsync("push")
	home.mustCcsync("pull")

	// 両方の PC で、同じセッションを別々に進めてしまった
	work.talk("s1", "会社で続き")
	home.talk("s1", "自宅で続き")
	home.talk("s2", "自宅で新しく始めた")
	work.mustCcsync("push")

	out, err := home.ccsync(home.root, "push")
	if err == nil {
		t.Fatal("競合があるのでエラーになるべき")
	}
	mustContain(t, out, "競合", "s1")
	// 競合していないものは送っている
	mustContain(t, out, "s2")
	if !strings.Contains(home.read("s1"), "自宅で続き") {
		t.Error("競合したセッションを書き換えてはいけない")
	}
}

func TestPull_競合があればエラーにする(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	work, home := newPC(t), newPC(t)
	work.initWith(remote, "work-pc")
	home.initWith(remote, "home-pc")

	work.talk("s1", "start")
	work.mustCcsync("push")
	home.mustCcsync("pull")
	work.talk("s1", "会社で続き")
	home.talk("s1", "自宅で続き")
	work.mustCcsync("push")

	out, err := home.ccsync(home.root, "pull")
	if err == nil {
		t.Fatal("競合があるのでエラーになるべき")
	}
	mustContain(t, out, "競合", "s1")
	if strings.Contains(home.read("s1"), "会社で続き") {
		t.Error("競合したセッションを上書きしてはいけない")
	}
}

// ---------- status ----------

func TestStatus_状態を表示して何も書き換えない(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	work, home := newPC(t), newPC(t)
	work.initWith(remote, "work-pc")
	home.initWith(remote, "home-pc")

	work.talk("s1", "hello")
	out := work.mustCcsync("status")
	mustContain(t, out, "github.com/nahaq789/pob", "s1", "手元だけ")

	work.mustCcsync("push")
	out = work.mustCcsync("status")
	mustContain(t, out, "s1", "同期済み")

	// 自宅の status は、push の後の最新を取り込んでから比べる
	out = home.mustCcsync("status")
	mustContain(t, out, "s1", "ストアだけ")
	if _, err := os.Stat(home.sessionPath("s1")); err == nil {
		t.Error("status で手元に書いてはいけない")
	}

	home.mustCcsync("pull")
	work.talk("s1", "続き")
	work.mustCcsync("push")
	out = home.mustCcsync("status")
	mustContain(t, out, "s1", "ストアが新しい")
	home.talk("s1", "自宅でも進めた")
	out = home.mustCcsync("status")
	mustContain(t, out, "s1", "競合")
}

func TestStatus_手元が新しい(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	a := newPC(t)
	a.initWith(remote, "work-pc")
	a.talk("s1", "hello")
	a.mustCcsync("push")
	a.talk("s1", "続き")
	out := a.mustCcsync("status")
	mustContain(t, out, "s1", "手元が新しい")
}

func TestStatus_競合があってもエラーにしない(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	work, home := newPC(t), newPC(t)
	work.initWith(remote, "work-pc")
	home.initWith(remote, "home-pc")
	work.talk("s1", "start")
	work.mustCcsync("push")
	home.mustCcsync("pull")
	work.talk("s1", "会社で続き")
	home.talk("s1", "自宅で続き")
	work.mustCcsync("push")
	if _, err := home.ccsync(home.root, "status"); err != nil {
		t.Errorf("status は見るだけなので、競合があってもエラーにしない: %v", err)
	}
}

func TestStatus_セッションがなくても表示する(t *testing.T) {
	isolateGit(t)
	remote := newStoreRemote(t)
	a := newPC(t)
	a.initWith(remote, "work-pc")
	out := a.mustCcsync("status")
	mustContain(t, out, "github.com/nahaq789/pob")
}

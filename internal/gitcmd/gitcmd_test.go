package gitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("LC_ALL", "C")
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

func newClone(t *testing.T, remote string) string {
	t.Helper()
	base := t.TempDir()
	storeDir := filepath.Join(base, "store")
	gitRun(t, base, "clone", remote, storeDir)
	return storeDir
}

// ---------- Run ----------

func TestRunGit_標準出力を前後の空白なしで返す(t *testing.T) {
	isolateGit(t)
	dir := newClone(t, newRemote(t))
	out, err := Run(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		t.Fatalf("エラーにならないはず: %v", err)
	}
	if out != "true" {
		t.Errorf("got %q, want %q", out, "true")
	}
}

func TestRunGit_標準エラーの内容は戻り値に混ぜない(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	// git clone は、成功しても「Cloning into ...」を標準エラーに出す。標準出力には何も出さない
	out, err := Run(t.TempDir(), "clone", remote, "x")
	if err != nil {
		t.Fatalf("エラーにならないはず: %v", err)
	}
	if out != "" {
		t.Errorf("標準エラーの内容が混ざっている: %q", out)
	}
}

func TestRunGit_失敗したらgitのメッセージを含むエラーを返す(t *testing.T) {
	isolateGit(t)
	out, err := Run(t.TempDir(), "no-such-command")
	if err == nil {
		t.Fatal("エラーになるはず")
	}
	if out != "" {
		t.Errorf("失敗したときの戻り値は空文字: %q", out)
	}
	if !strings.Contains(err.Error(), "is not a git command") {
		t.Errorf("git が出したメッセージが、エラーに含まれていない: %v", err)
	}
	if !strings.Contains(err.Error(), "no-such-command") {
		t.Errorf("実行したコマンドが、エラーに含まれていない: %v", err)
	}
}

func TestRunGit_指定したディレクトリで実行する(t *testing.T) {
	isolateGit(t)
	dir := newClone(t, newRemote(t))
	out, err := Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(out)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGit は、テスト中の git が、手元の設定(~/.gitconfig)の影響を受けないようにする。
// コミットに必要な名前とメールアドレスも、ここで設定する
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "ccsync-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "ccsync-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

// git は、テストの準備と確認のために、git を直接実行する(runGit とは別に用意している)
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRemote は、GitHub の代わりになるリポジトリを手元に作り、その場所を返す。
// 実際の claude-sessions と同じく、README.md が1つ入った状態にしてある
func newRemote(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	git(t, base, "init", "--bare", "-b", "main", remote)

	seed := filepath.Join(base, "seed")
	git(t, base, "clone", remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# claude-sessions\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", "-A")
	git(t, seed, "commit", "-m", "init")
	git(t, seed, "push", "origin", "HEAD:main")
	return remote
}

// newClone は、remote を Clone した storeDir を返す(1台の PC に相当する)
func newClone(t *testing.T, remote string) string {
	t.Helper()
	storeDir := filepath.Join(t.TempDir(), "store")
	if err := Clone(remote, storeDir); err != nil {
		t.Fatalf("Clone がエラーを返した: %v", err)
	}
	return storeDir
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ---------- runGit ----------

func TestRunGit_標準出力を前後の空白なしで返す(t *testing.T) {
	isolateGit(t)
	dir := newClone(t, newRemote(t))
	out, err := runGit(dir, "rev-parse", "--is-inside-work-tree")
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
	out, err := runGit(t.TempDir(), "clone", remote, "x")
	if err != nil {
		t.Fatalf("エラーにならないはず: %v", err)
	}
	if out != "" {
		t.Errorf("標準エラーの内容が混ざっている: %q", out)
	}
}

func TestRunGit_失敗したらgitのメッセージを含むエラーを返す(t *testing.T) {
	isolateGit(t)
	out, err := runGit(t.TempDir(), "no-such-command")
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
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(out)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------- Clone ----------

func TestClone_親がなくてもstoreDirにcloneする(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	parent := filepath.Join(t.TempDir(), "a", "b") // まだ存在しない
	storeDir := filepath.Join(parent, "store")

	if err := Clone(remote, storeDir); err != nil {
		t.Fatalf("Clone がエラーを返した: %v", err)
	}
	if !exists(filepath.Join(storeDir, ".git")) {
		t.Error("storeDir に .git がない")
	}
	if !exists(filepath.Join(storeDir, "README.md")) {
		t.Error("storeDir に README.md がない(clone されていない)")
	}

	// 親の中にできるのは store だけ(リポジトリ名のディレクトリは作られない)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "store" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("親の中身 = %v, want [store]", names)
	}
}

func TestClone_すでにclone済みなら何もしない(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	storeDir := newClone(t, remote)

	// 目印のファイルを置いて、2回目の Clone で消えないことを確かめる
	mark := filepath.Join(storeDir, "mark.txt")
	writeFile(t, mark, "x")

	if err := Clone(remote, storeDir); err != nil {
		t.Fatalf("2回目の Clone は、エラーにならないはず: %v", err)
	}
	if !exists(mark) {
		t.Error("2回目の Clone で、中身が作り直されている")
	}
}

func TestClone_cloneできないときはエラーを返す(t *testing.T) {
	isolateGit(t)
	storeDir := filepath.Join(t.TempDir(), "store")
	err := Clone(filepath.Join(t.TempDir(), "nai.git"), storeDir)
	if err == nil {
		t.Fatal("存在しないリポジトリなので、エラーになるはず")
	}
}

// ---------- CommitPush ----------

func TestCommitPush_変更がなければ何もしない(t *testing.T) {
	isolateGit(t)
	storeDir := newClone(t, newRemote(t))
	before := git(t, storeDir, "rev-parse", "HEAD")

	committed, err := CommitPush(storeDir, "何もないはず")
	if err != nil {
		t.Fatalf("エラーにならないはず: %v", err)
	}
	if committed {
		t.Error("変更がないので、false を返すはず")
	}
	if after := git(t, storeDir, "rev-parse", "HEAD"); after != before {
		t.Error("変更がないのに、コミットが作られている")
	}
}

func TestCommitPush_変更をコミットしてリモートに送る(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	storeDir := newClone(t, remote)
	writeFile(t, filepath.Join(storeDir, "repos", "x", "new.txt"), "hello")

	const msg = "ccsync: work-pc から 1 件を push"
	committed, err := CommitPush(storeDir, msg)
	if err != nil {
		t.Fatalf("CommitPush がエラーを返した: %v", err)
	}
	if !committed {
		t.Error("変更があるので、true を返すはず")
	}

	// 手元に、未コミットの変更が残っていない
	if st := git(t, storeDir, "status", "--porcelain"); st != "" {
		t.Errorf("未コミットの変更が残っている: %q", st)
	}
	// リモートに届いている(別の場所に clone し直して確かめる)
	other := newClone(t, remote)
	if !exists(filepath.Join(other, "repos", "x", "new.txt")) {
		t.Error("リモートに、ファイルが届いていない")
	}
	if got := git(t, other, "log", "-1", "--format=%s"); got != msg {
		t.Errorf("コミットメッセージ = %q, want %q", got, msg)
	}
}

func TestCommitPush_削除と変更も送る(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	storeDir := newClone(t, remote)
	writeFile(t, filepath.Join(storeDir, "a.txt"), "1")
	writeFile(t, filepath.Join(storeDir, "b.txt"), "1")
	if _, err := CommitPush(storeDir, "add"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(storeDir, "a.txt"), "2") // 変更
	os.Remove(filepath.Join(storeDir, "b.txt"))         // 削除
	committed, err := CommitPush(storeDir, "change")
	if err != nil || !committed {
		t.Fatalf("committed=%v err=%v", committed, err)
	}

	other := newClone(t, remote)
	if b, _ := os.ReadFile(filepath.Join(other, "a.txt")); string(b) != "2" {
		t.Errorf("変更が届いていない: %q", b)
	}
	if exists(filepath.Join(other, "b.txt")) {
		t.Error("削除が届いていない")
	}
}

func TestCommitPush_先に別のPCがpushしていたらエラーを返す(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	pcA := newClone(t, remote)
	pcB := newClone(t, remote)

	writeFile(t, filepath.Join(pcA, "a.txt"), "A")
	if _, err := CommitPush(pcA, "from A"); err != nil {
		t.Fatal(err)
	}

	// B は、最新を取り込まないまま push しようとする
	writeFile(t, filepath.Join(pcB, "b.txt"), "B")
	if _, err := CommitPush(pcB, "from B"); err == nil {
		t.Fatal("リモートが先に進んでいるので、エラーになるはず")
	}

	// Update してからやり直せば、成功する
	if err := Update(pcB); err != nil {
		t.Fatalf("Update がエラーを返した: %v", err)
	}
	writeFile(t, filepath.Join(pcB, "b.txt"), "B")
	committed, err := CommitPush(pcB, "from B")
	if err != nil || !committed {
		t.Fatalf("やり直しは成功するはず: committed=%v err=%v", committed, err)
	}
}

// ---------- Update ----------

func TestUpdate_別のPCがpushしたものを取り込む(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	pcA := newClone(t, remote)
	pcB := newClone(t, remote)

	writeFile(t, filepath.Join(pcA, "from-a.txt"), "A")
	if _, err := CommitPush(pcA, "from A"); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(pcB, "from-a.txt")) {
		t.Fatal("Update の前から、ファイルがある(テストの前提がおかしい)")
	}

	if err := Update(pcB); err != nil {
		t.Fatalf("Update がエラーを返した: %v", err)
	}
	if !exists(filepath.Join(pcB, "from-a.txt")) {
		t.Error("A が push したファイルが、取り込まれていない")
	}
}

func TestUpdate_手元に残ったものを捨ててリモートと同じ状態にする(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	storeDir := newClone(t, remote)

	// 前回の実行が途中で失敗した、という状況を作る
	writeFile(t, filepath.Join(storeDir, "README.md"), "書きかえた")               // 管理下のファイルの変更
	writeFile(t, filepath.Join(storeDir, "repos", "x", "leftover.txt"), "残骸") // 管理外のファイル
	writeFile(t, filepath.Join(storeDir, "committed.txt"), "x")
	git(t, storeDir, "add", "committed.txt")
	git(t, storeDir, "commit", "-m", "push されなかったコミット")

	if err := Update(storeDir); err != nil {
		t.Fatalf("Update がエラーを返した: %v", err)
	}

	if b, _ := os.ReadFile(filepath.Join(storeDir, "README.md")); string(b) != "# claude-sessions\n" {
		t.Errorf("README.md が、元に戻っていない: %q", b)
	}
	if exists(filepath.Join(storeDir, "repos", "x", "leftover.txt")) {
		t.Error("管理外のファイルが、残っている")
	}
	if exists(filepath.Join(storeDir, "repos")) {
		t.Error("管理外のディレクトリが、残っている")
	}
	if exists(filepath.Join(storeDir, "committed.txt")) {
		t.Error("push されなかったコミットが、残っている")
	}
	if st := git(t, storeDir, "status", "--porcelain"); st != "" {
		t.Errorf("未コミットの変更が残っている: %q", st)
	}
	if local, rem := git(t, storeDir, "rev-parse", "HEAD"), git(t, storeDir, "rev-parse", "origin/main"); local != rem {
		t.Error("手元の HEAD が、リモートと一致していない")
	}
}

func TestUpdate_cloneしていない場所ではエラーを返す(t *testing.T) {
	isolateGit(t)
	if err := Update(t.TempDir()); err == nil {
		t.Error("git のリポジトリではないので、エラーになるはず")
	}
}

// ---------- 一連の流れ ----------

// 会社の PC で書いたセッションが、自宅の PC で一覧に出てきて、読める
func TestGit_WriteからListまでの一連の流れ(t *testing.T) {
	isolateGit(t)
	remote := newRemote(t)
	work := newClone(t, remote)
	home := newClone(t, remote)

	// 会社の PC: 書いて、送る
	if err := Update(work); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, work, testKey, newMeta("s1"), testData)
	if committed, err := CommitPush(work, "ccsync: work-pc から 1 件を push"); err != nil || !committed {
		t.Fatalf("committed=%v err=%v", committed, err)
	}

	// 自宅の PC: 取り込んで、読む
	if ms, _ := List(home, testKey); len(ms) != 0 {
		t.Fatal("Update の前から、セッションがある(テストの前提がおかしい)")
	}
	if err := Update(home); err != nil {
		t.Fatal(err)
	}
	ms, err := List(home, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].ID != "s1" {
		t.Fatalf("got %v, want [s1]", ids(ms))
	}
	meta, data, err := Read(home, testKey, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !sameMeta(meta, newMeta("s1")) || string(data) != string(testData) {
		t.Error("自宅の PC で読んだ内容が、会社の PC で書いた内容と違う")
	}
}

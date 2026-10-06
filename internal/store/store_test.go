package store

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------- RepoKey ----------

func TestRepoKey(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// 同じリポジトリを指す、いろいろな書き方
		{"SSH(scp形式)", "git@github.com:Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"SSH(scp形式).git なし", "git@github.com:Nahaq789/pob", "github.com/nahaq789/pob"},
		{"SSH(scp形式)ユーザーなし", "github.com:Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"HTTPS", "https://github.com/Nahaq789/pob", "github.com/nahaq789/pob"},
		{"HTTPS .git つき", "https://github.com/Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"HTTPS 末尾スラッシュ", "https://github.com/Nahaq789/pob/", "github.com/nahaq789/pob"},
		{"HTTPS .git と末尾スラッシュ", "https://github.com/Nahaq789/pob.git/", "github.com/nahaq789/pob"},
		{"HTTP", "http://github.com/Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"ssh://", "ssh://git@github.com/Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"ssh:// ポートつき", "ssh://git@github.com:22/Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"HTTPS ポートつき", "https://github.com:443/Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"git://", "git://github.com/Nahaq789/pob.git", "github.com/nahaq789/pob"},
		{"認証情報つき", "https://user:token@github.com/Nahaq789/pob.git", "github.com/nahaq789/pob"},

		// 前後の空白と改行(git コマンドの出力には、末尾に改行がつく)
		{"末尾に改行", "git@github.com:Nahaq789/pob.git\n", "github.com/nahaq789/pob"},
		{"前後に空白", "  https://github.com/Nahaq789/pob.git  ", "github.com/nahaq789/pob"},

		// 小文字にそろえる
		{"ホストも小文字にする", "https://GitHub.com/Nahaq789/POB.git", "github.com/nahaq789/pob"},

		// 名前に記号を含む
		{"ハイフン", "git@github.com:Nahaq789/weather-report.git", "github.com/nahaq789/weather-report"},
		{"ドット", "https://github.com/Nahaq789/my.app.git", "github.com/nahaq789/my.app"},
		{"アンダースコア", "https://github.com/Nahaq789/a_b", "github.com/nahaq789/a_b"},
		{"名前が git で終わる(.git ではない)", "https://github.com/Nahaq789/legit", "github.com/nahaq789/legit"},
		{"名前が git で終わる(scp形式)", "git@github.com:Nahaq789/legit", "github.com/nahaq789/legit"},

		// GitHub 以外
		{"SSH のホストの別名", "git@github-personal:Nahaq789/pob.git", "github-personal/nahaq789/pob"},
		{"階層が深い(GitLab のサブグループ)", "git@gitlab.com:group/sub/repo.git", "gitlab.com/group/sub/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RepoKey(tt.in)
			if err != nil {
				t.Fatalf("RepoKey(%q) がエラーを返した: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("RepoKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRepoKey_エラーになるもの(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"空文字", ""},
		{"空白だけ", "   \n"},
		{"URL ではない", "not a url"},
		{"ローカルのパス", "/home/naha/dev/pob"},
		{"相対パス", "../pob"},
		{"パスがない", "https://github.com"},
		{"パスがスラッシュだけ", "https://github.com/"},
		{"scp形式でパスがない", "git@github.com:"},
		{"ホストがない", "https:///Nahaq789/pob"},
		{"空の要素を含む", "https://github.com/Nahaq789//pob"},
		{".. を含む(HTTPS)", "https://github.com/../x"},
		{".. を含む(scp形式)", "git@github.com:../x.git"},
		{".. を途中に含む", "https://github.com/Nahaq789/../../etc"},
		{". を含む", "https://github.com/./pob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RepoKey(tt.in)
			if err == nil {
				t.Errorf("RepoKey(%q) はエラーになるはずが、%q を返した", tt.in, got)
			}
		})
	}
}

// SSH と HTTPS、大文字と小文字など、書き方が違っても同じキーになる
func TestRepoKey_書き方が違っても同じキーになる(t *testing.T) {
	urls := []string{
		"git@github.com:Nahaq789/pob.git",
		"https://github.com/nahaq789/pob",
		"https://github.com/Nahaq789/Pob.git/",
		"ssh://git@github.com:22/NAHAQ789/pob.git\n",
	}
	first, err := RepoKey(urls[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range urls[1:] {
		got, err := RepoKey(u)
		if err != nil {
			t.Fatalf("RepoKey(%q): %v", u, err)
		}
		if got != first {
			t.Errorf("RepoKey(%q) = %q, want %q", u, got, first)
		}
	}
}

// ---------- Hash / Write / Read / List 共通 ----------

const testKey = "github.com/nahaq789/pob"

// 正規化済みの本体を模したデータ。プレースホルダーの制御文字を含み、全体としては JSON ではない
var testData = []byte("{\"cwd\":\"\x01CCSYNC_ROOT\x01\"}\n{\"type\":\"user\",\"text\":\"こんにちは\"}\n")

func newMeta(id string) Meta {
	return Meta{
		ID:        id,
		RelCwd:    "server",
		Hash:      Hash(testData),
		Machine:   "work-pc",
		UpdatedAt: time.Date(2026, 9, 30, 9, 2, 3, 0, time.UTC),
	}
}

func sameMeta(a, b Meta) bool {
	return a.ID == b.ID && a.RelCwd == b.RelCwd && a.Hash == b.Hash &&
		a.Machine == b.Machine && a.UpdatedAt.Equal(b.UpdatedAt)
}

func mustWrite(t *testing.T, storeDir, key string, m Meta, data []byte) {
	t.Helper()
	if err := Write(storeDir, key, m, data); err != nil {
		t.Fatalf("Write がエラーを返した: %v", err)
	}
}

// countFiles は dir 以下にあるファイルの数を返す(dir がなければ 0)
func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// ---------- Hash ----------

func TestHash(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}
	for _, tt := range tests {
		if got := Hash([]byte(tt.in)); got != tt.want {
			t.Errorf("Hash(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if Hash([]byte("a")) == Hash([]byte("b")) {
		t.Error("中身が違えば、ハッシュも違うはず")
	}
}

// ---------- Write / Read ----------

func TestWriteRead_書いたものを読むと同じものが返る(t *testing.T) {
	storeDir := t.TempDir()
	want := newMeta("s1")
	mustWrite(t, storeDir, testKey, want, testData)

	got, data, err := Read(storeDir, testKey, "s1")
	if err != nil {
		t.Fatalf("Read がエラーを返した: %v", err)
	}
	if !sameMeta(got, want) {
		t.Errorf("Meta が違う\n got:  %+v\n want: %+v", got, want)
	}
	if !bytes.Equal(data, testData) {
		t.Errorf("本体が違う\n got:  %q\n want: %q", data, testData)
	}
}

func TestWrite_決まった場所に2つのファイルを書く(t *testing.T) {
	storeDir := t.TempDir()
	mustWrite(t, storeDir, testKey, newMeta("s1"), testData)

	dir := filepath.Join(storeDir, "repos", "github.com", "nahaq789", "pob", "s1")

	// session.jsonl は、渡したデータそのまま
	data, err := os.ReadFile(filepath.Join(dir, "session.jsonl"))
	if err != nil {
		t.Fatalf("session.jsonl がない: %v", err)
	}
	if !bytes.Equal(data, testData) {
		t.Errorf("session.jsonl の中身が違う: %q", data)
	}

	// meta.json は、決まったキーを持つ JSON
	raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatalf("meta.json がない: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("meta.json が JSON として読めない: %v\n%s", err, raw)
	}
	for _, k := range []string{"id", "rel_cwd", "hash", "machine", "updated_at"} {
		if _, ok := m[k]; !ok {
			t.Errorf("meta.json にキー %q がない: %s", k, raw)
		}
	}
	if m["id"] != "s1" || m["rel_cwd"] != "server" || m["machine"] != "work-pc" {
		t.Errorf("meta.json の値が違う: %s", raw)
	}

	if n := countFiles(t, storeDir); n != 2 {
		t.Errorf("ファイルの数 = %d, want 2", n)
	}
}

func TestWrite_本人だけが読める権限で書く(t *testing.T) {
	storeDir := t.TempDir()
	mustWrite(t, storeDir, testKey, newMeta("s1"), testData)

	dir := filepath.Join(storeDir, "repos", testKey, "s1")
	for _, p := range []string{dir, filepath.Join(dir, "session.jsonl"), filepath.Join(dir, "meta.json")} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s の権限が %o。本人以外に読み書きを許している", filepath.Base(p), perm)
		}
	}
}

func TestWrite_すでにあれば上書きする(t *testing.T) {
	storeDir := t.TempDir()
	mustWrite(t, storeDir, testKey, newMeta("s1"), testData)

	newData := []byte("短くなったデータ\n")
	m2 := newMeta("s1")
	m2.Machine = "home-pc"
	m2.Hash = Hash(newData)
	mustWrite(t, storeDir, testKey, m2, newData)

	got, data, err := Read(storeDir, testKey, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, newData) {
		t.Errorf("本体が上書きされていない: %q", data)
	}
	if got.Machine != "home-pc" || got.Hash != Hash(newData) {
		t.Errorf("Meta が上書きされていない: %+v", got)
	}
	if n := countFiles(t, storeDir); n != 2 {
		t.Errorf("ファイルの数 = %d, want 2", n)
	}
}

func TestWrite_不正なIDでは何も書かない(t *testing.T) {
	for _, id := range []string{"", ".", "..", "a/b", "../x", "/abs"} {
		t.Run("ID="+id, func(t *testing.T) {
			storeDir := t.TempDir()
			if err := Write(storeDir, testKey, newMeta(id), testData); err == nil {
				t.Errorf("ID %q はエラーになるはず", id)
			}
			if n := countFiles(t, storeDir); n != 0 {
				t.Errorf("エラーなのに、ファイルが %d 個書かれている", n)
			}
		})
	}
}

func TestWrite_ドットを含むIDは書ける(t *testing.T) {
	storeDir := t.TempDir()
	for _, id := range []string{"session.v2", "a..b", ".hidden"} {
		if err := Write(storeDir, testKey, newMeta(id), testData); err != nil {
			t.Errorf("ID %q は書けるはず: %v", id, err)
		}
	}
}

func TestRead_エラーになるもの(t *testing.T) {
	storeDir := t.TempDir()
	mustWrite(t, storeDir, testKey, newMeta("ok"), testData)

	// meta.json がない
	mustWrite(t, storeDir, testKey, newMeta("nometa"), testData)
	os.Remove(filepath.Join(storeDir, "repos", testKey, "nometa", "meta.json"))
	// session.jsonl がない
	mustWrite(t, storeDir, testKey, newMeta("nodata"), testData)
	os.Remove(filepath.Join(storeDir, "repos", testKey, "nodata", "session.jsonl"))
	// meta.json が壊れている
	mustWrite(t, storeDir, testKey, newMeta("broken"), testData)
	os.WriteFile(filepath.Join(storeDir, "repos", testKey, "broken", "meta.json"), []byte("{こわれている"), 0o600)

	tests := []struct{ name, id string }{
		{"存在しないセッション", "nai"},
		{"meta.json がない", "nometa"},
		{"session.jsonl がない", "nodata"},
		{"meta.json が壊れている", "broken"},
		{"ID が空", ""},
		{"ID が ..", ".."},
		{"ID が .", "."},
		{"ID が / を含む", "ok/../ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := Read(storeDir, testKey, tt.id); err == nil {
				t.Errorf("Read(%q) はエラーになるはず", tt.id)
			}
		})
	}

	// 正常なものは、読める
	if _, _, err := Read(storeDir, testKey, "ok"); err != nil {
		t.Errorf("正常なセッションが読めない: %v", err)
	}
}

func TestRead_別のリポジトリのセッションは読めない(t *testing.T) {
	storeDir := t.TempDir()
	mustWrite(t, storeDir, testKey, newMeta("s1"), testData)
	if _, _, err := Read(storeDir, "github.com/nahaq789/lovers", "s1"); err == nil {
		t.Error("別の key では、見つからないはず")
	}
}

// ---------- List ----------

func ids(ms []Meta) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func sameStrings(a, b []string) bool {
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

func TestList_IDの昇順で返る(t *testing.T) {
	storeDir := t.TempDir()
	for _, id := range []string{"ccc", "aaa", "bbb"} {
		mustWrite(t, storeDir, testKey, newMeta(id), testData)
	}
	ms, err := List(storeDir, testKey)
	if err != nil {
		t.Fatalf("List がエラーを返した: %v", err)
	}
	if want := []string{"aaa", "bbb", "ccc"}; !sameStrings(ids(ms), want) {
		t.Errorf("got %v, want %v", ids(ms), want)
	}
}

func TestList_Metaの中身がそのまま返る(t *testing.T) {
	storeDir := t.TempDir()
	want := newMeta("s1")
	mustWrite(t, storeDir, testKey, want, testData)
	ms, err := List(storeDir, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || !sameMeta(ms[0], want) {
		t.Errorf("got %+v, want %+v", ms, want)
	}
}

func TestList_まだ何もないときは空を返す(t *testing.T) {
	t.Run("ストアが空", func(t *testing.T) {
		ms, err := List(t.TempDir(), testKey)
		if err != nil {
			t.Fatalf("エラーにしない: %v", err)
		}
		if len(ms) != 0 {
			t.Errorf("want 0件: %v", ms)
		}
	})
	t.Run("別のリポジトリのセッションしかない", func(t *testing.T) {
		storeDir := t.TempDir()
		mustWrite(t, storeDir, "github.com/nahaq789/lovers", newMeta("s1"), testData)
		ms, err := List(storeDir, testKey)
		if err != nil {
			t.Fatalf("エラーにしない: %v", err)
		}
		if len(ms) != 0 {
			t.Errorf("別の key のセッションが混ざっている: %v", ids(ms))
		}
	})
}

func TestList_壊れたものは読み飛ばす(t *testing.T) {
	storeDir := t.TempDir()
	keyDir := filepath.Join(storeDir, "repos", testKey)
	mustWrite(t, storeDir, testKey, newMeta("ok1"), testData)
	mustWrite(t, storeDir, testKey, newMeta("ok2"), testData)

	// ディレクトリではないもの
	if err := os.WriteFile(filepath.Join(keyDir, "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// meta.json がないディレクトリ
	if err := os.MkdirAll(filepath.Join(keyDir, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	// meta.json がなく、session.jsonl だけがあるディレクトリ
	mustWrite(t, storeDir, testKey, newMeta("nometa"), testData)
	os.Remove(filepath.Join(keyDir, "nometa", "meta.json"))
	// meta.json が壊れているディレクトリ
	mustWrite(t, storeDir, testKey, newMeta("broken"), testData)
	os.WriteFile(filepath.Join(keyDir, "broken", "meta.json"), []byte("{こわれている"), 0o600)
	// meta.json の ID が、ディレクトリ名と違う
	mustWrite(t, storeDir, testKey, newMeta("mismatch"), testData)
	raw, _ := json.Marshal(newMeta("zzz"))
	os.WriteFile(filepath.Join(keyDir, "mismatch", "meta.json"), raw, 0o600)

	ms, err := List(storeDir, testKey)
	if err != nil {
		t.Fatalf("壊れたものがあっても、エラーにしない: %v", err)
	}
	if want := []string{"ok1", "ok2"}; !sameStrings(ids(ms), want) {
		t.Errorf("got %v, want %v", ids(ms), want)
	}
}

func TestList_一覧できないときはエラーを返す(t *testing.T) {
	// key の場所に、ディレクトリではなくファイルがある(「存在しない」以外のエラー)
	storeDir := t.TempDir()
	p := filepath.Join(storeDir, "repos", testKey)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := List(storeDir, testKey); err == nil {
		t.Error("エラーを返すはず")
	}
}

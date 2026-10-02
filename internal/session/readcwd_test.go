package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp はテスト用の一時ファイルを作り、そのパスを返す
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadCwd(t *testing.T) {
	long := strings.Repeat("x", 200_000) // 1行が 64KB を超えるケース用

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"1行目に cwd", `{"type":"user","cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"1行目に cwd がなく、2行目にある", `{"type":"summary","summary":"x"}` + "\n" + `{"type":"user","cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"最初に見つかったものを返す", `{"cwd":"/first"}` + "\n" + `{"cwd":"/second"}` + "\n", "/first"},
		{"サブディレクトリ", `{"cwd":"/home/a/dev/pob/server"}` + "\n", "/home/a/dev/pob/server"},
		{"最後の行に改行がない", `{"type":"summary"}` + "\n" + `{"cwd":"/home/a/dev/pob"}`, "/home/a/dev/pob"},
		{"壊れた行は読み飛ばす", "これはJSONではない\n" + `{"cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"空行は読み飛ばす", "\n\n" + `{"cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"cwd が空文字の行は読み飛ばす", `{"cwd":""}` + "\n" + `{"cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"入れ子の中の cwd は対象外", `{"message":{"cwd":"/nested"}}` + "\n" + `{"cwd":"/top"}` + "\n", "/top"},
		{"本文に cwd という文字列があっても惑わされない", `{"message":{"content":"\"cwd\":\"/fake\""}}` + "\n" + `{"cwd":"/real"}` + "\n", "/real"},
		{"とても長い行のあとにある", `{"type":"user","text":"` + long + `"}` + "\n" + `{"cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"とても長い行の中にある", `{"text":"` + long + `","cwd":"/home/a/dev/pob"}` + "\n", "/home/a/dev/pob"},
		{"cwd がどこにもない", `{"type":"summary"}` + "\n" + `{"type":"user"}` + "\n", ""},
		{"空のファイル", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadCwd(writeTemp(t, tt.content))
			if err != nil {
				t.Fatalf("エラーにならないはず: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadCwd_FileNotFound(t *testing.T) {
	_, err := ReadCwd(filepath.Join(t.TempDir(), "nai.jsonl"))
	if err == nil {
		t.Fatal("ファイルがないときはエラーを返す")
	}
}

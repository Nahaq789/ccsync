package pathnorm

import (
	"strings"
	"testing"
)

// テストは Placeholder の具体的な値に依存しないように書いている。
// 期待値の中の "<R>" は Placeholder に置き換えてから比較する。
func r(s string) string { return strings.ReplaceAll(s, "<R>", Placeholder) }

const rootA = "/home/a/dev/pob"
const rootB = "/home/b/x/pob2"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cwd がルートそのもの", `{"cwd":"/home/a/dev/pob"}`, `{"cwd":"<R>"}`},
		{"ルート配下のファイル", `"/home/a/dev/pob/main.go"`, `"<R>/main.go"`},
		{"深い階層", `"/home/a/dev/pob/server/api/handler.go"`, `"<R>/server/api/handler.go"`},
		{"末尾がスラッシュ", `"/home/a/dev/pob/"`, `"<R>/"`},
		{"コマンド中", `cd /home/a/dev/pob && ls`, `cd <R> && ls`},
		{"文字列全体がルート", `/home/a/dev/pob`, `<R>`},
		{"1行に2回", `"/home/a/dev/pob/a.go","/home/a/dev/pob/b.go"`, `"<R>/a.go","<R>/b.go"`},
		{"複数行", "{\"cwd\":\"/home/a/dev/pob\"}\n{\"cwd\":\"/home/a/dev/pob\"}\n", "{\"cwd\":\"<R>\"}\n{\"cwd\":\"<R>\"}\n"},
		{"日本語の本文中", `"/home/a/dev/pob を開いて"`, `"<R> を開いて"`},
		{"コロン区切り(エラー出力など)", `/home/a/dev/pob/main.go:12:3: undefined`, `<R>/main.go:12:3: undefined`},

		// 置き換えてはいけないもの
		{"後ろに - が続く", `"/home/a/dev/pob-old/x"`, `"/home/a/dev/pob-old/x"`},
		{"後ろに数字が続く", `"/home/a/dev/pob2"`, `"/home/a/dev/pob2"`},
		{"後ろに . が続く", `"/home/a/dev/pob.bak"`, `"/home/a/dev/pob.bak"`},
		{"後ろに _ が続く", `"/home/a/dev/pob_v1"`, `"/home/a/dev/pob_v1"`},
		{"前に別のパスがある", `"/mnt/home/a/dev/pob"`, `"/mnt/home/a/dev/pob"`},
		{"前に英数字がある", `x/home/a/dev/pob`, `x/home/a/dev/pob`},
		{"ルートの親", `"/home/a/dev"`, `"/home/a/dev"`},
		{"無関係なパス", `"/usr/local/go/bin"`, `"/usr/local/go/bin"`},
		{"空のデータ", ``, ``},

		// 置き換えるものと置き換えないものが混在
		{"混在", `"/home/a/dev/pob-old" "/home/a/dev/pob/x" "/home/a/dev/pob2"`, `"/home/a/dev/pob-old" "<R>/x" "/home/a/dev/pob2"`},
		{"直前の一致を誤って飛ばさない", `/home/a/dev/pob2 /home/a/dev/pob`, `/home/a/dev/pob2 <R>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(Normalize([]byte(tt.in), rootA))
			if want := r(tt.want); got != want {
				t.Errorf("\n in:   %q\n got:  %q\n want: %q", tt.in, got, want)
			}
		})
	}
}

func TestNormalize_EmptyRoot(t *testing.T) {
	in := `{"cwd":"/home/a/dev/pob"}`
	if got := string(Normalize([]byte(in), "")); got != in {
		t.Errorf("root が空なら何もしない: got %q", got)
	}
}

func TestDenormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cwd", `{"cwd":"<R>"}`, `{"cwd":"/home/b/x/pob2"}`},
		{"配下のファイル", `"<R>/main.go"`, `"/home/b/x/pob2/main.go"`},
		{"複数", `<R>/a <R>/b`, `/home/b/x/pob2/a /home/b/x/pob2/b`},
		{"プレースホルダーなし", `"/home/a/dev/pob-old/x"`, `"/home/a/dev/pob-old/x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(Denormalize([]byte(r(tt.in)), rootB))
			if got != tt.want {
				t.Errorf("\n got:  %q\n want: %q", got, tt.want)
			}
		})
	}
}

// A で正規化して B で戻すと、ルートだけが B のものに変わり、それ以外は変わらない
func TestRoundTrip_AtoB(t *testing.T) {
	in := `{"cwd":"/home/a/dev/pob","f":"/home/a/dev/pob/main.go","old":"/home/a/dev/pob-old/x","m":"/mnt/home/a/dev/pob"}`
	want := `{"cwd":"/home/b/x/pob2","f":"/home/b/x/pob2/main.go","old":"/home/a/dev/pob-old/x","m":"/mnt/home/a/dev/pob"}`
	got := string(Denormalize(Normalize([]byte(in), rootA), rootB))
	if got != want {
		t.Errorf("\n got:  %q\n want: %q", got, want)
	}
}

// 同じルートで正規化して戻すと、元と完全に一致する
func TestRoundTrip_Identity(t *testing.T) {
	inputs := []string{
		`{"cwd":"/home/a/dev/pob"}`,
		`/home/a/dev/pob-old /home/a/dev/pob /home/a/dev/pob2`,
		// 会話の本文にプレースホルダー風の文字列が出てきても壊れない
		`{"text":"{{ROOT}} と {{CCSYNC_ROOT}} と $ROOT と __ROOT__"}`,
		`{"text":"ccsync はパスを <ROOT> に置き換えます"}`,
		"",
	}
	for _, in := range inputs {
		got := string(Denormalize(Normalize([]byte(in), rootA), rootA))
		if got != in {
			t.Errorf("\n in:  %q\n got: %q", in, got)
		}
	}
}

// 正規化した結果には、元のルートが(境界付きでは)残らない
func TestNormalize_NoRootLeft(t *testing.T) {
	in := `{"cwd":"/home/a/dev/pob","a":"/home/a/dev/pob/x","b":"cd /home/a/dev/pob"}`
	got := string(Normalize([]byte(in), rootA))
	if strings.Contains(got, rootA) {
		t.Errorf("ルートが残っている: %q", got)
	}
}

// プレースホルダーは、通常のテキストに出てこない形であること
func TestPlaceholder_IsUnlikelyInText(t *testing.T) {
	if Placeholder == "" {
		t.Fatal("Placeholder が空")
	}
	if strings.Contains(Placeholder, "/") {
		t.Error("Placeholder に / を含めると、パスの一部と区別しにくくなる")
	}
	hasCtrl := false
	for _, c := range Placeholder {
		if c < 0x20 {
			hasCtrl = true
		}
	}
	if !hasCtrl {
		t.Log("ヒント: 制御文字を含めない場合、会話の本文と偶然一致する可能性が残ります")
	}
}

package store

import "testing"

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

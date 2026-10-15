# ccsync

Claude Code CLI のセッション(会話履歴)を、プライベートな GitHub リポジトリ経由で複数の PC 間で同期するツールです。

```bash
# PC-A(作業の終わりに)
cd ~/dev/pob
ccsync push

# PC-B
cd ~/dev/pob
ccsync pull
claude --resume      # いつもの CLI で続きから
```

## 特徴

- **リポジトリ単位で同期**:今いる git リポジトリのセッションだけを扱います。ほかのプロジェクトの会話は送りません。リポジトリのサブディレクトリ(`server/` など)で始めた会話も含みます。
- **パスが違っても大丈夫**:会話の中に書かれたリポジトリのパスを、取り込む PC のパスに書き換えます。ユーザー名やフォルダ構成が PC ごとに違っていても続きから使えます。
- **SSH / HTTPS の違いを吸収**:リポジトリは `origin` の URL から `github.com/owner/repo` の形で識別します。`git@github.com:...` と `https://github.com/...` は同じリポジトリとして扱います。
- **競合を検知**:両方の PC で同じセッションを進めてしまった場合は、上書きせずにスキップして知らせます。

## 動作環境

- Linux / macOS / WSL(Windows ネイティブは対象外)
- Go 1.26 以上
- git と、GitHub に SSH で接続できる設定(`ssh -T git@github.com` が通ること)

## インストール

```bash
go install github.com/Nahaq789/ccsync@latest
```

`~/go/bin/ccsync` に入ります。`ccsync` が見つからない場合は、`~/go/bin` を PATH に追加してください(zsh なら `~/.zshrc`、bash なら `~/.bashrc`)。

```bash
export PATH="$HOME/go/bin:$PATH"
```

Claude Code を動かしている環境と同じ側(WSL なら WSL の中)に入れてください。

入ったバージョンは `ccsync --version` で確かめられます。

## セットアップ

### 1. ストアにする GitHub リポジトリを作る

GitHub で **プライベート** リポジトリを 1 つ作ります(例:`claude-sessions`)。

空のリポジトリは clone できないため、作るときに README を追加するなどして、コミットを 1 つ以上入れておいてください。

### 2. 各 PC で初期設定する

```bash
# PC-A
ccsync init --machine work-pc git@github.com:Nahaq789/claude-sessions.git

# PC-B
ccsync init --machine home-pc git@github.com:Nahaq789/claude-sessions.git
```

- `--machine` は、その PC の名前です。ストアの記録とコミットメッセージに残るので、知られて困らない名前を付けてください。
- `--machine` は URL より前に書いてください(後ろに書くと読まれません)。

設定は `~/.ccsync/config.json` に保存され、ストアは `~/.ccsync/store` に clone されます。

## 使い方

同期したいリポジトリの中(サブディレクトリでも可)で実行します。

| コマンド | 内容 |
| --- | --- |
| `ccsync status` | 手元とストアの状態を一覧表示する(何も書き換えない) |
| `ccsync push` | 手元の方が新しいセッションを、ストアに送る |
| `ccsync pull` | ストアの方が新しいセッションを、手元に取り込む |
| `ccsync --version`(`ccsync version` でも可) | インストールされている ccsync のバージョンを表示する |

いずれも、最初に GitHub の最新を取り込んでから比べます。

### version の表示

```
$ ccsync --version
ccsync v1.1.0
```

### status の表示

```
github.com/nahaq789/pob
  1a2b3c4d-...  手元が新しい
  9f8e7d6c-...  ストアが新しい
```

| 状態 | 意味 | 何をすればよいか |
| --- | --- | --- |
| 同期済み | 手元とストアが同じ | 何もしなくてよい |
| 手元だけ | この PC にしかない | `push` |
| ストアだけ | ストアにしかない(もう一方の PC で始めた) | `pull` |
| 手元が新しい | この PC で続きを話した | `push` |
| ストアが新しい | もう一方の PC で続きを話した | `pull` |
| 競合 | 両方の PC で別々に続きを話した | 下の「競合したとき」を参照 |

### push / pull の表示

```
push: 2件 1a2b3c4d-..., 9f8e7d6c-...
競合: 1件(両方のPCで進んでいるため、スキップしました) 5e6f7a8b-...
ccsync: 競合が 1 件あります
```

### 競合したとき

競合したセッションは、push でも pull でも書き換えずにスキップし、ほかのセッションだけを処理します。競合が 1 件でもあれば、終了コード 1 で終わります(`status` は見るだけなので、競合があっても 0 です)。

競合を上書きする機能はありません。どちらか一方の PC で、そのセッションのファイル(`~/.claude/projects/<フォルダ>/<セッションID>.jsonl`)を退避するか削除してから、もう一度 `pull` してください。

### 終了コード

| 終了コード | 意味 |
| --- | --- |
| 0 | 成功 |
| 1 | エラー、または push / pull で競合があった |

## 運用の注意

- **push / pull の前に Claude Code を閉じてください。** 開いたままだと、その後も会話ファイルへの書き込みが続きます。
- **同じセッションを 2 台で同時に進めないでください。** 競合になります。
- `pull` は、ストアの方が新しいセッションを手元に上書きします。手元でしか進めていない変更がある場合は競合として検出されるので、消えることはありません。
- `claude --resume` の一覧に出ない場合は、`claude --resume <セッションID>` で直接開けます。

## セキュリティ上の注意

- **ストアには会話の全文が入ります。** 会話に貼ったソースコードや秘密情報が含まれる可能性があるので、ストアのリポジトリは必ずプライベートにしてください。
- **公開したくないリポジトリでは使わないでください。** リポジトリのコードや、それについての会話が個人の GitHub に上がることになります。個人開発のリポジトリだけで使ってください。
- マシン名はストアの `meta.json` とコミットメッセージに記録されます。

## 制限

- Claude Code の保存形式(`~/.claude/projects/` 配下)は公式に公開された仕様ではありません。Claude Code の更新で動かなくなる可能性があります。
- 同期するのは会話本体(`<セッションID>.jsonl`)だけです。
- リポジトリは `origin` の URL で識別するため、`origin` が設定されていないリポジトリでは使えません。

## 仕組み

### データの置き場所

| 場所 | 中身 |
| --- | --- |
| `~/.claude/projects/<作業ディレクトリをエンコードした名前>/<セッションID>.jsonl` | Claude Code の会話 |
| `~/.ccsync/config.json` | この PC の設定(ストアの URL、マシン名) |
| `~/.ccsync/store/` | ストアの clone |
| ストアの `repos/<github.com/owner/repo>/<セッションID>/` | `meta.json` と `session.jsonl` |

### パスの書き換え

ストアに入れるときに、会話の中のリポジトリのルートパス(例:`/home/naha/dev/pob`)を、制御文字を含む目印に置き換えます。取り込むときは、その目印を取り込む PC のルートパスに戻します。`/home/naha/dev/pob-old` のような、ルートで始まる別のパスは置き換えません。

### 新旧の判定

Claude Code の会話ファイルは、後ろに追記されていくだけで、途中が書き換わることはありません。この性質を使って、手元とストアの内容を比べて判定します。

| 比べた結果 | 判定 |
| --- | --- |
| 同じ | 同期済み |
| 手元の先頭部分がストアと一致する | 手元が新しい |
| ストアの先頭部分が手元と一致する | ストアが新しい |
| どちらでもない | 競合 |

前回の同期の記録は持たず、毎回内容から判定します。

### package 構成

| package | 役割 |
| --- | --- |
| `internal/pathnorm` | パスを目印に置き換える / 戻す |
| `internal/session` | 手元から、このリポジトリの会話を探す |
| `internal/store` | ストアの読み書きと、GitHub との同期 |
| `internal/gitcmd` | git コマンドの実行 |
| `internal/syncer` | 手元とストアを比べて、push / pull を行う |
| `internal/config` | この PC の設定の保存と読み込み |
| `main.go` | コマンドの受け付けと表示 |

## 開発

```bash
go test ./...
```

テストは、手元に作った bare リポジトリを GitHub の代わりにして、2 台の PC を模した往復まで確かめます。ネットワークには接続しません。

# muxflow

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**tmuxのコマンドを覚えなくても、tmuxを使いこなせる。**

`tmux new-session -s myproject` や `tmux attach-session -t myproject`——そんなコマンドを毎回調べていませんか？  
muxflow は、[lazygit](https://github.com/jesseduffield/lazygit) が git を直感的にしたように、**tmux をキーボード操作だけで扱える TUI ツール**です。

プロジェクト一覧からセッションを起動・アタッチしたり、PC再起動後にまとめて自動起動したりできます。  
ウィンドウ・ペインの分割レイアウトや、よく使う起動コマンド（`docker compose up`、`npm run dev` など）を登録しておけば、`Enter` 一発でいつでも同じ環境が立ち上がります。

[Bubble Tea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss) で実装されています。

---

## 前提条件

**tmux** が必要です。先にインストールしてください。

```bash
# Ubuntu / Debian
sudo apt install tmux

# macOS (Homebrew)
brew install tmux
```

---

## インストール

[GitHub Releases](https://github.com/masaway/muxflow/releases/latest) からプラットフォームに合ったアーカイブをダウンロードして展開し、PATHの通った場所に移動します。

**Linux (amd64)**

```bash
curl -L https://github.com/masaway/muxflow/releases/latest/download/muxflow_linux_amd64.tar.gz | tar xz
mv muxflow ~/.local/bin/
```

**Linux (arm64)**

```bash
curl -L https://github.com/masaway/muxflow/releases/latest/download/muxflow_linux_arm64.tar.gz | tar xz
mv muxflow ~/.local/bin/
```

**macOS (Apple Silicon)**

```bash
curl -L https://github.com/masaway/muxflow/releases/latest/download/muxflow_darwin_arm64.tar.gz | tar xz
mv muxflow /usr/local/bin/
```

**macOS (Intel)**

```bash
curl -L https://github.com/masaway/muxflow/releases/latest/download/muxflow_darwin_amd64.tar.gz | tar xz
mv muxflow /usr/local/bin/
```

**Windows**

> Windows では tmux が使えないため、**WSL（Windows Subsystem for Linux）** 上での利用を推奨します。WSL 内では上記の Linux 手順でインストールしてください。

### go install / ソースビルド

```bash
# go install
go install github.com/masaway/muxflow@latest

# ソースビルド
git clone https://github.com/masaway/muxflow
cd muxflow
go build -o muxflow .
mv muxflow ~/.local/bin/
```

> Go 1.24.2 以上が必要です。

---

## 更新

### コマンドで更新する（推奨）

```bash
muxflow --update
```

最新バージョンを確認し、新しいバージョンがあれば確認後に自動でバイナリを更新します。

### 手動で更新する

インストール時と同じコマンドを再実行すると上書きインストールできます。

```bash
# 例: Linux (amd64)
curl -L https://github.com/masaway/muxflow/releases/latest/download/muxflow_linux_amd64.tar.gz | tar xz
mv muxflow ~/.local/bin/
```

### バージョン確認

```bash
muxflow --version
```

---

## 起動

```bash
muxflow
```

> `command not found` と表示される場合は、インストール先がPATHに含まれているか確認してください。
>
> ```bash
> # ~/.local/bin をPATHに追加する場合（Linux）
> echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
> ```

---

## 初回起動

起動すると、プロジェクト一覧は空の状態でTUIが開きます。まずプロジェクトを登録しましょう。

### 1. スキャンディレクトリを設定する

初回起動時はスキャンディレクトリが未設定のため、**設定画面が自動で開きます**。スキャンしたいディレクトリ（例: `~/work`）を入力して `Enter` で保存してください。

### 2. tmux 推奨設定を適用する（任意）

スキャンディレクトリ設定後、**tmux 推奨設定画面**が開きます。ツール制作者がおすすめする `~/.tmux.conf` を1ステップで適用できます。

| 主な設定内容 | |
|---|---|
| Prefix | `Ctrl+S` |
| ペイン移動 | `Prefix + h / j / k / l` |
| ペイン分割 | `Prefix + \` (横) / `-` (縦) |
| ステータスバー | セッション名・現在パス・git branch 表示 |
| muxflow 起動 | `Prefix + m` で新ウィンドウに muxflow を開く |
| マウス操作 | 有効 |

既存の `~/.tmux.conf` がある場合は **バックアップ（`.tmux.conf.bak`）してから上書き**するか、上書きのみかを選べます。バックアップがある場合は適用後に元の設定に戻すこともできます。

設定不要な場合は `q` または `Esc` でスキップできます。

### 3. プロジェクトをスキャンして追加する

設定画面を閉じると、続けてスキャン画面が開きます。指定したディレクトリ配下のGitリポジトリが一覧表示されます。

1. `j` / `k` でカーソル移動
2. `Space` で追加したいプロジェクトを選択
3. `Enter` で登録

> メイン画面に戻った後、不要なプロジェクトは `X` キーでリスト末尾の非表示セクションへ移動できます。

### 4. セッションを起動・アタッチする

`Enter` キーでセッションを起動し、そのままアタッチします。まずはここまで試してみてください。

> スキャン画面は後からでもメイン画面で `s` キーを押すと開けます。

### 5. 自動起動を設定する（任意）

PC再起動後にセッションをまとめて立ち上げたい場合は、プロジェクト一覧で `a` キーを押して `★` を付けておきます。

---

## 起動コマンドを登録する

使い慣れてきたら、プロジェクトごとに起動コマンドを登録しておくのがおすすめです。  
メイン画面でプロジェクトを選択して `e` キーを押すと、**いつでも**エディタ画面を開けます。

**これが muxflow の一番の使いどころです。**  
`docker compose up` や `npm run dev` といったコマンドを登録しておくと、次回からセッション起動と同時に自動実行されます。「このプロジェクトの起動コマンドなんだっけ？」を考える必要がなくなります。

FEとBEが別れているプロジェクトでも、ウィンドウを分けてそれぞれのコマンドを登録しておけば、`Enter` 一発で両方まとめて起動できます。

### コマンドの実行モード

ペインごとに **実行するかどうか** を選べます。

| Execute | 動作 |
|---------|------|
| ON | セッション起動時にコマンドを自動実行する |
| OFF | コマンドをターミナルに入力した状態で止める（実行はしない） |

OFF にしておくと「コマンドは出てくるが Enter は自分で押す」状態になるので、起動前に引数を確認・変更したい場面でも使えます。

### 設定例：FE + BE を1セッションで起動

```
ウィンドウ: dev
  ペイン1  dir: ./backend   command: docker compose up   Execute: ON
  ペイン2  dir: ./frontend  command: npm run dev          Execute: ON
```

エディタ画面の操作方法は[キーバインド一覧](documents/keybindings.md)を参照してください。

---

## 2回目以降の起動

### tmuxセッションがゼロの状態で起動した場合

`★` のついたプロジェクトが自動で一括起動します。その後、アタッチ先をTUIで選択してください。

### tmuxセッションがある状態で起動した場合

プロジェクト一覧が表示されます。起動中のセッションは `Enter` でアタッチ、停止中のセッションは `Enter` で起動してアタッチします。

---

## 詳細ドキュメント

- [キーバインド一覧](documents/keybindings.md)
- [設定ファイル](documents/configuration.md)

---

## Contributing

バグ報告・機能提案・PRはいつでも歓迎です。詳しくは [CONTRIBUTING.md](CONTRIBUTING.md) をご覧ください。

## License

[MIT](LICENSE)

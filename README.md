# lazyprj

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

tmuxセッションをTUIで管理するツールです。プロジェクト一覧からセッションを起動・アタッチしたり、PC再起動後にまとめて自動起動したりできます。

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

[GitHub Releases](https://github.com/masaway/lazyprj/releases/latest) からプラットフォームに合ったアーカイブをダウンロードして展開し、PATHの通った場所に移動します。

**Linux (amd64)**

```bash
curl -L https://github.com/masaway/lazyprj/releases/latest/download/lazyprj_linux_amd64.tar.gz | tar xz
mv lazyprj ~/.local/bin/
```

**Linux (arm64)**

```bash
curl -L https://github.com/masaway/lazyprj/releases/latest/download/lazyprj_linux_arm64.tar.gz | tar xz
mv lazyprj ~/.local/bin/
```

**macOS (Apple Silicon)**

```bash
curl -L https://github.com/masaway/lazyprj/releases/latest/download/lazyprj_darwin_arm64.tar.gz | tar xz
mv lazyprj /usr/local/bin/
```

**macOS (Intel)**

```bash
curl -L https://github.com/masaway/lazyprj/releases/latest/download/lazyprj_darwin_amd64.tar.gz | tar xz
mv lazyprj /usr/local/bin/
```

**Windows**

> Windows では tmux が使えないため、**WSL（Windows Subsystem for Linux）** 上での利用を推奨します。WSL 内では上記の Linux 手順でインストールしてください。

### go install / ソースビルド

```bash
# go install
go install github.com/masaway/lazyprj@latest

# ソースビルド
git clone https://github.com/masaway/lazyprj
cd lazyprj
go build -o lazyprj .
mv lazyprj ~/.local/bin/
```

> Go 1.24.2 以上が必要です。

---

## 起動

```bash
lazyprj
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

### 2. プロジェクトをスキャンして追加する

設定画面を閉じると、続けてスキャン画面が開きます。指定したディレクトリ配下のGitリポジトリが一覧表示されます。

1. `j` / `k` でカーソル移動
2. `Space` で追加したいプロジェクトを選択
3. `Enter` で登録

### 3. セッションを起動・アタッチする

`Enter` キーでセッションを起動し、そのままアタッチします。まずはここまで試してみてください。

### 4. 自動起動を設定する（任意）

PC再起動後にセッションをまとめて立ち上げたい場合は、プロジェクト一覧で `a` キーを押して `★` を付けておきます。

---

## 起動コマンドを登録する

使い慣れてきたら、プロジェクトごとに起動コマンドを登録しておくのがおすすめです。  
メイン画面でプロジェクトを選択して `e` キーを押すと、**いつでも**エディタ画面を開けます。

**これが lazyprj の一番の使いどころです。**  
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

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## ビルドと実行

```bash
# ビルド
go build -o lazyprj .

# 実行（リポジトリルートから）
./lazyprj

# 依存関係の更新
go mod tidy
```

## ブランチ戦略

GitHub Flow を採用。`main` への直pushは禁止（ブランチ保護設定済み）。

```bash
git switch -c feature/xxx
# 実装後
git push origin feature/xxx
gh pr create
gh pr merge --delete-branch
git switch main && git pull
```

## アーキテクチャ概要

tmuxセッションをTUIで管理するGoアプリケーション。[Bubble Tea](https://github.com/charmbracelet/bubbletea) の Model-Update-View パターンで実装されている。

### 画面遷移

`internal/ui/app.go` の `App` struct が Bubble Tea のルートモデル。`currentScreen` フィールドで画面を切り替える：

- `screenMain` → `App`（メイン画面、プロジェクト一覧＋詳細）
- `screenEditor` → `EditorModel`（ウィンドウ・ペイン編集）
- `screenScan` → `ScanModel`（未登録プロジェクトのスキャン）
- `screenSetup` → `SetupModel`（スキャンディレクトリ設定、初回起動時に自動遷移）
- `screenQuickstart` → `QuickstartModel`（新規セッション作成）

### アタッチの遅延実行

tmuxのアタッチは `tea.Quit` 後に実行する必要があるため、`App.pendingAttach` にセッション名を保存し、`main.go` で `p.Run()` の戻り値から `PendingAttach()` を呼び出して実行する。

### 設定ファイル

`internal/config/config.go` の `GetConfigPath()` が `~/.config/lazyprj/config.json` を返す。`XDG_CONFIG_HOME` が設定されている場合は `$XDG_CONFIG_HOME/lazyprj/config.json` を使用。保存も同パスに行われる。

初回起動時（ファイルが存在しない場合）は `ScanDirectory` が空の `Config` を返し、`app.go` が `screenSetup` へ自動遷移する。

### モジュール構成

| パッケージ | 役割 |
|-----------|------|
| `internal/config` | 設定のロード・保存・データ構造・プロジェクトスキャン |
| `internal/tmux` | tmuxコマンド操作（セッション作成・停止・アタッチ） |
| `internal/ui` | Bubble Tea の全画面モデル、スタイル定義 |

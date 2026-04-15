# Contributing to lazyprj

バグ報告・機能提案・プルリクエスト、どれも歓迎です。

---

## バグ報告・機能提案

[Issues](https://github.com/masaway/lazyprj/issues) からテンプレートを選んで投稿してください。

---

## プルリクエスト

### 開発環境のセットアップ

```bash
git clone https://github.com/masaway/lazyprj
cd lazyprj
go mod tidy
```

### ビルドと動作確認

```bash
go build -o lazyprj .
./lazyprj
```

### PRを送る前に

- `go build` が通ること
- 既存の動作を壊していないこと（手動で一通り操作して確認）
- コミットメッセージは変更内容が伝わるように書いてください

### アーキテクチャについて

[CLAUDE.md](CLAUDE.md) にコード構成の概要があります。実装の参考にしてください。

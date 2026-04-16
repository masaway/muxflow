# tmux ソケット分離 (`-L` オプション)

`-L` オプションを使うと、tmux サーバーと設定ファイルをまるごと分離した状態で lazyprj を起動できます。

---

## 使い方

```bash
./lazyprj -L <ソケット名>
```

### 例

```bash
# 通常起動（デフォルト）
./lazyprj

# "demo" という名前のソケットで起動
./lazyprj -L demo
```

---

## `-L` を指定したときの挙動

| 項目 | 通常起動 | `-L demo` 指定時 |
|------|----------|-----------------|
| tmux サーバー | デフォルトソケット | `lazyprj-demo` ソケット |
| 設定ファイル | `~/.config/lazyprj/config.json` | `~/.config/lazyprj/config-demo.json` |

tmux サーバーと設定ファイルの両方が分離されるため、通常の作業環境に一切影響しません。

---

## デモ・スクリーンショット用途

README 等に掲載する画像を撮影するとき、通常の作業セッションを汚さずにクリーンな状態を作れます。

### 初回起動フロー（setup 画面から）のデモ

```bash
# 設定ファイルが存在しない状態で起動 → setup 画面から始まる
./lazyprj -L demo
```

`config-demo.json` が存在しない場合、自動的に setup 画面へ遷移します。
通常の `config.json` はそのまま保持されます。

### セッションがゼロの状態からのデモ

```bash
# demo ソケットには tmux サーバーが存在しないため、全プロジェクトが停止中で表示される
./lazyprj -L demo
```

lazyprj 上でセッションを起動する操作から順にデモできます。

---

## デモ環境のリセット

```bash
# tmux サーバーを停止（セッションをすべて破棄）
tmux -L demo kill-server

# 設定ファイルを削除（初回起動フローに戻す）
rm ~/.config/lazyprj/config-demo.json
```

両方実行すれば完全にまっさらな状態に戻ります。

---

## 複数環境の使い分け

ソケット名ごとに独立した環境を持てるため、用途に応じて使い分けられます。

```bash
./lazyprj -L work     # 仕事用（config-work.json + work ソケット）
./lazyprj -L personal # 個人用（config-personal.json + personal ソケット）
./lazyprj -L demo     # デモ用（config-demo.json + demo ソケット）
```

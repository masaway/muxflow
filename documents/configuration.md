# 設定ファイル

設定ファイルは初回起動時に自動作成されます。

| パス | 説明 |
|------|------|
| `~/.config/muxflow/config.json` | 設定ファイル |

`XDG_CONFIG_HOME` が設定されている場合は `$XDG_CONFIG_HOME/muxflow/config.json` を使用します。

---

## 設定の構造

```json
{
  "projects": [
    {
      "name": "myapp",
      "path": "~/work/myapp",
      "auto_start": true,
      "description": "メインの開発プロジェクト",
      "windows": [
        {
          "name": "dev",
          "layout": "even-horizontal",
          "panes": [
            { "dir": ".", "command": "", "execute": false },
            { "dir": ".", "command": "npm run dev", "execute": true }
          ]
        }
      ]
    }
  ],
  "hidden_projects": [
    {
      "name": "old-project",
      "path": "/home/user/work/old-project"
    }
  ],
  "skipped_paths": [
    "/home/user/work/archived"
  ],
  "settings": {
    "scan_directory": "/home/user/work"
  }
}
```

### プロジェクトフィールド

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `name` | string | tmuxセッション名 |
| `path` | string | プロジェクトのルートディレクトリ |
| `auto_start` | bool | 起動時に自動起動するか（省略時: false） |
| `description` | string | 説明文（省略可） |
| `windows` | array | ウィンドウ・ペイン構成 |

### 非表示プロジェクト

`hidden_projects` にはメイン画面で `X` キーにより非表示にしたプロジェクトが保存されます。  
リスト末尾の `▶ 非表示 (N)` セクションから確認でき、`X` キーで通常リストに復元できます。

### スキップ済みパス（レガシー）

`skipped_paths` は以前のバージョンとの互換性のために残されています。新規エントリは追加されません。  
ここに記録されたパスは非表示セクションに表示され、`X` キーで復元できます。

---

## tmuxレイアウト

エディタ画面でウィンドウのレイアウトを選択できます（プレビュー付き）。

| レイアウト | 説明 |
|-----------|------|
| `even-horizontal` | ペインを左右に均等分割 |
| `even-vertical` | ペインを上下に均等分割 |
| `main-horizontal` | 上に広いメイン、下に小ペイン |
| `main-vertical` | 左に広いメイン、右に小ペイン |
| `tiled` | グリッド状に配置 |

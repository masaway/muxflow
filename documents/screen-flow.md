# lazyprj 画面フロー図

## 全体画面遷移

オーバーレイ画面（Editor / Scan / Quickstart）は閉じると常に Main に戻る。

```mermaid
flowchart TD
    START([起動]) --> LOAD[設定ファイル読み込み]
    LOAD -->|ScanDirectory が空| SETUP[Setup 画面\nスキャンディレクトリ設定]
    LOAD -->|設定済み| MAIN[Main 画面\nプロジェクト一覧]

    SETUP -->|完了 / スキップ| MAIN

    MAIN --> EDITOR[Editor 画面\n※オーバーレイ]
    MAIN --> SCAN[Scan 画面\n※オーバーレイ]
    MAIN --> SETUP
    MAIN --> QUICKSTART[Quickstart 画面\n※オーバーレイ]
    MAIN -->|セッションアタッチ| TMUX([tmux アタッチ\n※アプリ終了])
```

---

## Main 画面

プロジェクト一覧と詳細を表示するメイン画面。

```mermaid
flowchart LR
    subgraph MAIN_SCREEN[Main 画面]
        direction TB
        LEFT[プロジェクトリスト\n左パネル]
        RIGHT[プロジェクト詳細\n右パネル]
        STATUSBAR[ステータスバー]
        KEYHINTS[キーヒント]
    end
```

---

## Setup 画面（フルスクリーン）

初回起動時または `S` キーで表示。スキャンディレクトリのパスを入力する。

```mermaid
flowchart TD
    SETUP_START([Setup 画面表示]) --> INPUT[パス入力フォーム]
    INPUT -->|保存| SAVE[設定保存]
    INPUT -->|スキップ\n既存設定がある場合のみ| SKIP[スキップ]
    SAVE --> MAIN([Main 画面へ])
    SKIP --> MAIN
```

**表示条件:**
- 初回起動時（`ScanDirectory` が空）→ 自動遷移
- Main 画面で `S` キー押下

---

## Editor 画面（オーバーレイ）

Main 画面の上に重なって表示される。プロジェクトのウィンドウ・ペイン構成を編集する。

ウィンドウリストとペインリストは Tab で相互に切り替え可能。

```mermaid
flowchart LR
    START([Editor 画面表示]) --> WIN_LIST[ウィンドウリスト]
    WIN_LIST -->|切り替え| PANE_LIST[ペインリスト]

    WIN_LIST -->|追加 / 編集| WIN_FORM[ウィンドウフォーム]
    WIN_FORM -->|確定 / キャンセル| WIN_LIST

    PANE_LIST -->|追加 / 編集| PANE_FORM[ペインフォーム]
    PANE_FORM -->|確定 / キャンセル| PANE_LIST

    WIN_LIST -->|保存 / 閉じる| MAIN([Main 画面へ])
    PANE_LIST -->|保存 / 閉じる| MAIN
```

---

## Scan 画面（オーバーレイ）

Main 画面の上に重なって表示される。スキャンディレクトリ配下の未登録プロジェクトを一覧表示し、登録するプロジェクトを選択する。

```mermaid
flowchart TD
    SCAN_START([Scan 画面表示]) --> LOADING[スキャン中...]
    LOADING --> LIST[プロジェクト一覧]

    LIST -->|スキップ済み表示切替| LIST
    LIST -->|登録| SAVE[設定に追加保存]
    LIST -->|閉じる| MAIN([Main 画面へ])

    SAVE --> MAIN
```

---

## Quickstart 画面（オーバーレイ）

Main 画面の上に重なって表示される。任意ディレクトリを指定して即座にtmuxセッションを作成する。

```mermaid
flowchart TD
    QS_START([Quickstart 画面表示]) --> DIR_INPUT[ディレクトリパス入力]
    DIR_INPUT -->|次へ| NAME_INPUT[セッション名入力]
    NAME_INPUT -->|戻る| DIR_INPUT
    NAME_INPUT -->|作成| CREATE[セッション作成 & 設定に追加]
    DIR_INPUT -->|閉じる| MAIN([Main 画面へ])
    NAME_INPUT -->|閉じる| MAIN
    CREATE --> MAIN
```

---

## オーバーレイ一覧

```mermaid
flowchart TD
    subgraph MAIN_BASE[Main 画面（ベース）]
        BASE[プロジェクト一覧 + 詳細]
    end

    subgraph OVERLAYS[オーバーレイ層]
        OV1[Editor 画面\nウィンドウ/ペイン編集]
        OV2[Scan 画面\n新規プロジェクト登録]
        OV3[Quickstart 画面\nセッション即時作成]
        OV4[Help ダイアログ\nキー一覧]
        OV5[確認ダイアログ\nセッション再起動確認]
        OV6[Sync 確認ダイアログ\n設定変更の同期確認]
    end

    subgraph FULLSCREEN[フルスクリーン]
        FS1[Setup 画面\nスキャンディレクトリ設定]
    end

    MAIN_BASE --> OV1
    MAIN_BASE --> OV2
    MAIN_BASE --> OV3
    MAIN_BASE --> OV4
    MAIN_BASE --> OV5
    MAIN_BASE --> OV6
    MAIN_BASE -.->|置き換え| FS1
```

> **オーバーレイ**: Main 画面がバックグラウンドに透けて見える状態でダイアログが前面表示される
>
> **フルスクリーン**: Main 画面を完全に置き換えて表示される（Setup のみ）

---

## 起動フロー

```mermaid
sequenceDiagram
    participant User as ユーザー
    participant App as lazyprj
    participant Config as 設定ファイル
    participant Tmux as tmux

    User->>App: 起動
    App->>Config: 設定読み込み
    alt ScanDirectory が未設定
        App-->>User: Setup 画面表示
        User->>App: パス入力 + Enter
        App->>Config: ScanDirectory 保存
    end
    App-->>User: Main 画面表示
    alt 初回起動 & tmuxセッションなし & AutoStart プロジェクトあり
        App->>Tmux: AutoStart プロジェクトを一括起動
    end
    User->>App: Enter/s（セッション選択）
    alt セッション未起動
        App->>Tmux: セッション作成
    end
    App->>Tmux: アタッチ（アプリ終了後）
    Tmux-->>User: tmux セッションへ
```

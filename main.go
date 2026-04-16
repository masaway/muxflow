package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/creativeprojects/go-selfupdate"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/masaway/lazyprj/internal/config"
	"github.com/masaway/lazyprj/internal/tmux"
	"github.com/masaway/lazyprj/internal/ui"
)

var version = "dev"

func selfUpdate() {
	if version == "dev" {
		fmt.Println("開発ビルドのため更新をスキップします。")
		return
	}
	fmt.Println("最新バージョンを確認中...")
	latest, found, err := selfupdate.DetectLatest(context.Background(), selfupdate.ParseSlug("masaway/lazyprj"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "バージョン確認エラー:", err)
		os.Exit(1)
	}
	if !found || latest.LessOrEqual(version) {
		fmt.Println("すでに最新バージョンです:", version)
		return
	}
	fmt.Printf("新しいバージョンが見つかりました: %s → %s\n", version, latest.Version())
	fmt.Print("更新しますか？ [y/N]: ")
	var input string
	fmt.Scanln(&input)
	if input != "y" && input != "Y" {
		fmt.Println("更新をキャンセルしました。")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "実行ファイルパス取得エラー:", err)
		os.Exit(1)
	}
	if err := selfupdate.UpdateTo(context.Background(), latest.AssetURL, latest.AssetName, exe); err != nil {
		fmt.Fprintln(os.Stderr, "更新エラー:", err)
		os.Exit(1)
	}
	fmt.Printf("更新完了: %s\n", latest.Version())
}

func main() {
	var socketName string
	var showVersion bool
	var doUpdate bool
	flag.StringVar(&socketName, "L", "", "使用する tmux ソケット名（例: lazyprj-demo）")
	flag.BoolVar(&showVersion, "version", false, "バージョンを表示して終了")
	flag.BoolVar(&doUpdate, "update", false, "最新バージョンに更新して終了")
	flag.Parse()

	if showVersion {
		fmt.Println("lazyprj", version)
		os.Exit(0)
	}

	if doUpdate {
		selfUpdate()
		os.Exit(0)
	}

	if socketName != "" {
		tmux.SetSocket(socketName)
		config.SetSocket(socketName)
	}

	app := ui.New()

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	result, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}

	// アタッチが必要な場合は終了後に実行
	if finalApp, ok := result.(*ui.App); ok {
		if name := finalApp.PendingAttach(); name != "" {
			if attachErr := tmux.AttachOrSwitch(name); attachErr != nil {
				fmt.Fprintf(os.Stderr, "アタッチエラー (%s): %v\n", name, attachErr)
				os.Exit(1)
			}
		}
	}
}

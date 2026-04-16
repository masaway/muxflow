package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/masaway/lazyprj/internal/config"
	"github.com/masaway/lazyprj/internal/tmux"
	"github.com/masaway/lazyprj/internal/ui"
)

func main() {
	var socketName string
	flag.StringVar(&socketName, "L", "", "使用する tmux ソケット名（例: lazyprj-demo）")
	flag.Parse()

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

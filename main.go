package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/masaway/muxflow/internal/config"
	"github.com/masaway/muxflow/internal/tmux"
	"github.com/masaway/muxflow/internal/ui"
)

var version = "dev"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/repos/masaway/muxflow/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func findAsset(rel *githubRelease) (url, name string, found bool) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, goos) && strings.Contains(lower, goarch) {
			return a.BrowserDownloadURL, a.Name, true
		}
	}
	return "", "", false
}

func selfUpdate() {
	if version == "dev" {
		fmt.Println("開発ビルドのため更新をスキップします。")
		return
	}
	fmt.Println("最新バージョンを確認中...")
	rel, err := fetchLatestRelease()
	if err != nil {
		fmt.Fprintln(os.Stderr, "バージョン確認エラー:", err)
		os.Exit(1)
	}
	latestTag := strings.TrimPrefix(rel.TagName, "v")
	latestVer, err := semver.NewVersion(latestTag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "バージョン解析エラー:", err)
		os.Exit(1)
	}
	currentVer, err := semver.NewVersion(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "現在バージョン解析エラー:", err)
		os.Exit(1)
	}
	if !latestVer.GreaterThan(currentVer) {
		fmt.Println("すでに最新バージョンです:", version)
		return
	}
	assetURL, assetName, found := findAsset(rel)
	if !found {
		fmt.Fprintln(os.Stderr, "このプラットフォーム向けのアセットが見つかりません")
		os.Exit(1)
	}
	fmt.Printf("新しいバージョンが見つかりました: %s → %s\n", version, latestTag)
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
	if err := selfupdate.UpdateTo(context.Background(), assetURL, assetName, exe); err != nil {
		fmt.Fprintln(os.Stderr, "更新エラー:", err)
		os.Exit(1)
	}
	fmt.Printf("更新完了: %s\n", latestTag)
}

func main() {
	var socketName string
	var showVersion bool
	var doUpdate bool
	flag.StringVar(&socketName, "L", "", "使用する tmux ソケット名（例: muxflow-demo）")
	flag.BoolVar(&showVersion, "version", false, "バージョンを表示して終了")
	flag.BoolVar(&doUpdate, "update", false, "最新バージョンに更新して終了")
	flag.Parse()

	if showVersion {
		fmt.Println("muxflow", version)
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

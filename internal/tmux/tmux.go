package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/masaway/muxflow/internal/config"
)

var socket string

// SetSocket は使用する tmux ソケット名を設定する（例: "muxflow-demo"）。
// 空文字の場合はデフォルトソケットを使用する。
func SetSocket(s string) {
	socket = s
}

// socketArgs は tmux コマンドに -L <socket> を付加するための引数を返す
func socketArgs(args []string) []string {
	if socket == "" {
		return args
	}
	return append([]string{"-L", socket}, args...)
}

func run(args ...string) (int, string) {
	cmd := exec.Command("tmux", socketArgs(args)...)
	out, _ := cmd.CombinedOutput()
	err := cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), strings.TrimSpace(string(out))
		}
		return 1, ""
	}
	return 0, strings.TrimSpace(string(out))
}

func runCmd(args ...string) (int, string) {
	cmd := exec.Command("tmux", socketArgs(args)...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), ""
		}
		return 1, ""
	}
	return 0, strings.TrimSpace(string(out))
}

// ListSessions は現在のtmuxセッション一覧を返す
func ListSessions() map[string]bool {
	code, out := runCmd("list-sessions", "-F", "#{session_name}")
	if code != 0 {
		return map[string]bool{}
	}
	sessions := map[string]bool{}
	for _, s := range strings.Split(out, "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			sessions[s] = true
		}
	}
	return sessions
}

// SessionExists は指定セッションが存在するか確認する
func SessionExists(name string) bool {
	return ListSessions()[name]
}

// KillSession は指定セッションを停止する
func KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

var shellProcesses = map[string]bool{
	"bash": true, "zsh": true, "sh": true, "fish": true,
	"tcsh": true, "ksh": true, "dash": true, "csh": true,
}

// ListActiveProcesses はセッション内で実行中のプロセス名一覧を返す（シェルを除く）
func ListActiveProcesses(sessionName string) ([]string, error) {
	code, out := runCmd("list-panes", "-t", sessionName, "-s", "-F", "#{pane_current_command}")
	if code != 0 {
		return nil, fmt.Errorf("list-panes failed")
	}
	seen := map[string]bool{}
	var result []string
	for _, cmd := range strings.Split(out, "\n") {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" || shellProcesses[cmd] || seen[cmd] {
			continue
		}
		seen[cmd] = true
		result = append(result, cmd)
	}
	return result, nil
}

func resolveDir(projectPath, paneDir string) string {
	paneDir = config.ExpandPath(paneDir)
	if filepath.IsAbs(paneDir) {
		return paneDir
	}
	return filepath.Join(projectPath, paneDir)
}

// joinCommand は複数行コマンドを "; " でつないで1行にする
func joinCommand(cmd string) string {
	var parts []string
	for _, line := range strings.Split(cmd, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "; ")
}

// shellQuote はシェルコマンド内でパスを安全に使えるようシングルクォートでエスケープする
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// CreateSession はプロジェクト設定からtmuxセッションを作成する
func CreateSession(project *config.Project, killExisting bool) (bool, error) {
	name := project.Name
	path := config.ExpandPath(project.Path)

	// プロジェクトルートの存在確認
	if _, err := os.Stat(path); err != nil {
		return false, fmt.Errorf("ディレクトリが見つかりません: %s", path)
	}

	if SessionExists(name) {
		if killExisting {
			if err := KillSession(name); err != nil {
				return false, err
			}
		} else {
			return false, nil // スキップ（既に起動中）
		}
	}

	project.MigrateFromCommands()

	if !project.HasWindows() {
		runCmd("new-session", "-d", "-s", name, "-c", path)
		return true, nil
	}

	firstWindow := true
	for winIdx, window := range project.Windows {
		winTarget := fmt.Sprintf("%s:%d", name, winIdx)

		// pane 0 のディレクトリ（new-window に使う）
		firstPaneDir := path
		if len(window.Panes) > 0 && window.Panes[0].Dir != "" {
			firstPaneDir = resolveDir(path, window.Panes[0].Dir)
		}

		if firstWindow {
			runCmd("new-session", "-d", "-s", name, "-c", firstPaneDir, "-n", window.Name)
			firstWindow = false
		} else {
			runCmd("new-window", "-t", name, "-n", window.Name, "-c", firstPaneDir)
		}

		for paneIdx, pane := range window.Panes {
			paneDir := resolveDir(path, pane.Dir)

			if paneIdx > 0 {
				runCmd("split-window", "-t", winTarget, "-c", paneDir)
				// 分割後すぐにレイアウトを適用することで、ペインが小さくなりすぎて
				// 次の split-window が失敗するのを防ぐ
				runCmd("select-layout", "-t", winTarget, window.Layout)
			}

			paneTarget := fmt.Sprintf("%s.%d", winTarget, paneIdx)

			if pane.Command != "" {
				cmd := joinCommand(pane.Command)
				if pane.Execute {
					runCmd("send-keys", "-t", paneTarget, cmd, "Enter")
				} else {
					runCmd("send-keys", "-t", paneTarget, cmd)
				}
			}
		}

		runCmd("select-layout", "-t", winTarget, window.Layout)
		runCmd("select-pane", "-t", fmt.Sprintf("%s.0", winTarget))
	}

	runCmd("select-window", "-t", fmt.Sprintf("%s:0", name))
	return true, nil
}

// InspectSession は実行中セッションのウィンドウ・ペイン構成を取得する。
func InspectSession(name string) []config.Window {
	code, out := runCmd("list-windows", "-t", name, "-F", "#{window_index}|#{window_name}|#{window_layout}")
	if code != 0 {
		return nil
	}

	var windows []config.Window
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		winIdxStr, winName, layout := parts[0], parts[1], parts[2]

		winIdx := 0
		fmt.Sscanf(winIdxStr, "%d", &winIdx)

		if layout == "" {
			layout = "even-horizontal"
		}

		target := fmt.Sprintf("%s:%s", name, winIdxStr)
		code2, paneOut := runCmd("list-panes", "-t", target, "-F", "#{pane_current_path}")
		var panes []config.Pane
		if code2 == 0 {
			for _, p := range strings.Split(paneOut, "\n") {
				p = strings.TrimSpace(p)
				if p != "" {
					panes = append(panes, config.Pane{Dir: p})
				}
			}
		}
		if len(panes) == 0 {
			panes = []config.Pane{{Dir: "."}}
		}
		windows = append(windows, config.Window{
			Name:   winName,
			Layout: layout,
			Panes:  panes,
		})
	}
	return windows
}

// IsInsideTmux は現在tmuxセッション内で実行されているかを返す
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// SwitchClient はtmuxのswitch-clientを実行する（tmux内専用）
func SwitchClient(name string) error {
	return exec.Command("tmux", socketArgs([]string{"switch-client", "-t", name})...).Run()
}

// AttachOrSwitch はtmux内ならswitch-client、外ならattach-sessionを実行する
func AttachOrSwitch(name string) error {
	if IsInsideTmux() {
		return SwitchClient(name)
	}
	cmd := exec.Command("tmux", socketArgs([]string{"attach-session", "-t", name})...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

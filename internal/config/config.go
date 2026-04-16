package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Pane はtmuxペインの設定
type Pane struct {
	Dir     string `json:"dir"`
	Command string `json:"command,omitempty"`
	Execute bool   `json:"execute"`
}

// Window はtmuxウィンドウの設定
type Window struct {
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Panes  []Pane `json:"panes"`
}

// Commands は旧フォーマットのコマンド設定
type Commands struct {
	Startup string `json:"startup,omitempty"`
	Dev     string `json:"dev,omitempty"`
}

// Project はプロジェクト設定
type Project struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	AutoStart   bool     `json:"auto_start,omitempty"`
	Description string   `json:"description,omitempty"`
	Commands    Commands `json:"commands,omitempty"`
	Windows     []Window `json:"windows,omitempty"`
}

// Settings はグローバル設定
type Settings struct {
	ScanDirectory            string `json:"scan_directory"`
	DefaultLayout            string `json:"default_layout"`
	AutoAttach               bool   `json:"auto_attach"`
	KillExisting             bool   `json:"kill_existing"`
	CreateDevPane            bool   `json:"create_dev_pane"`
	DevPaneSize              string `json:"dev_pane_size"`
	EnableParentDirectory    bool   `json:"enable_parent_directory"`
	EnableScriptDirectory    bool   `json:"enable_script_directory"`
	EnableClaudeInBottomPane bool   `json:"enable_claude_in_bottom_pane"`
}

// Config は設定ファイル全体
type Config struct {
	Projects     []Project `json:"projects"`
	SkippedPaths []string  `json:"skipped_paths,omitempty"`
	Settings     Settings  `json:"settings"`
}

func (p *Project) HasWindows() bool {
	return len(p.Windows) > 0
}

// MigrateFromCommands は旧フォーマットから新フォーマットへ移行
func (p *Project) MigrateFromCommands() {
	if p.HasWindows() {
		return
	}
	panes := []Pane{{Dir: "."}}
	if p.Commands.Dev != "" {
		panes = append(panes, Pane{Dir: ".", Command: p.Commands.Dev})
	}
	w := Window{
		Name:   "dev",
		Layout: "even-horizontal",
		Panes:  panes,
	}
	if p.Commands.Startup != "" {
		w.Panes[0].Command = p.Commands.Startup
		w.Panes[0].Execute = true
	}
	p.Windows = []Window{w}
}

// configDir は設定ディレクトリ (~/.config/lazyprj/) を返す
func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazyprj")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lazyprj")
}

var configSocket string

// SetSocket はソケット名を設定する。空でない場合、config ファイル名が
// config-<socket>.json になる。
func SetSocket(s string) {
	configSocket = s
}

// GetConfigPath は設定ファイルのパスを返す。
// ソケット名が指定されている場合は config-<socket>.json を使用する。
func GetConfigPath() string {
	name := "config.json"
	if configSocket != "" {
		name = "config-" + configSocket + ".json"
	}
	return filepath.Join(configDir(), name)
}

// Load は設定ファイルを読み込む
func Load() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{
			Settings: Settings{DefaultLayout: "even-vertical"},
		}, nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save は設定を ~/.config/lazyprj/config.json に保存する
func Save(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetConfigPath(), data, 0o644)
}

// ExpandPath はチルダ展開を行う
func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

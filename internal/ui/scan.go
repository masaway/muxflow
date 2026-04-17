package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/masaway/muxflow/internal/config"
)

// ScanModel は新規プロジェクトスキャン画面
type ScanModel struct {
	width, height int
	cfg           *config.Config

	projects []config.ScannedProject
	cursor   int
	selected map[int]bool

	status      string
	statusIsErr bool
	done        bool
	loading     bool
}

// ─── メッセージ ───────────────────────────────────────────────────────────────

type scanLoadedMsg struct {
	newProjects []config.ScannedProject
	err         error
}

type scanSavedMsg struct{ err error }

// ─── コンストラクタ ───────────────────────────────────────────────────────────

func NewScanner(cfg *config.Config) *ScanModel {
	return &ScanModel{
		cfg:      cfg,
		selected: map[int]bool{},
		loading:  true,
	}
}

func (m *ScanModel) Init() tea.Cmd { return nil }

func (m *ScanModel) IsDone() bool { return m.done }

func (m *ScanModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

// LoadCmd はスキャンを非同期で実行するコマンドを返す
func (m *ScanModel) LoadCmd() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		result, err := config.ScanNewProjects(cfg)
		return scanLoadedMsg{newProjects: result.New, err: err}
	}
}

// ─── カーソル計算ヘルパー ────────────────────────────────────────────────────

// totalItems はカーソルが動ける全アイテム数を返す
func (m *ScanModel) totalItems() int {
	return len(m.projects)
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m *ScanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("スキャンエラー: %s", msg.err)
			m.statusIsErr = true
		} else {
			m.projects = msg.newProjects
			m.updateStatus()
		}

	case scanSavedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("保存エラー: %s", msg.err)
			m.statusIsErr = true
		} else {
			m.done = true
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *ScanModel) updateStatus() {
	if len(m.projects) == 0 {
		m.status = "新規プロジェクトは見つかりませんでした"
	} else {
		m.status = fmt.Sprintf("%d 件の未登録プロジェクトを発見", len(m.projects))
	}
}

func (m *ScanModel) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q":
		m.done = true

	case "j", "down":
		if m.cursor < m.totalItems()-1 {
			m.cursor++
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case " ":
		if m.cursor < len(m.projects) {
			m.selected[m.cursor] = !m.selected[m.cursor]
			if m.cursor < len(m.projects)-1 {
				m.cursor++
			}
		}

	case "a":
		if len(m.selected) == len(m.projects) {
			m.selected = map[int]bool{}
		} else {
			for i := range m.projects {
				m.selected[i] = true
			}
		}

	case "enter":
		if len(m.selected) == 0 {
			m.status = "プロジェクトが選択されていません"
			m.statusIsErr = true
			break
		}
		return m, m.saveCmd()
	}
	return m, nil
}

func (m *ScanModel) saveCmd() tea.Cmd {
	for i, p := range m.projects {
		if m.selected[i] {
			proj := config.Project{Name: p.Name, Path: p.Path}
			for j, hp := range m.cfg.HiddenProjects {
				if hp.Path == p.Path {
					proj = hp
					m.cfg.HiddenProjects = append(m.cfg.HiddenProjects[:j], m.cfg.HiddenProjects[j+1:]...)
					break
				}
			}
			m.cfg.Projects = append(m.cfg.Projects, proj)
		}
	}
	cfg := m.cfg
	return func() tea.Msg {
		if err := config.Save(cfg); err != nil {
			return scanSavedMsg{err: err}
		}
		return scanSavedMsg{}
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *ScanModel) View() string {
	dialogW := m.width - 12
	dialogH := m.height - 6
	if dialogW > 110 {
		dialogW = 110
	}
	if dialogW < 54 {
		dialogW = 54
	}
	if dialogH > 30 {
		dialogH = 30
	}
	if dialogH < 12 {
		dialogH = 12
	}
	return m.buildScanDialog(dialogW, dialogH)
}

func (m *ScanModel) buildScanDialog(w, h int) string {
	innerW := w - 2
	innerH := h - 2

	// status(1) + keyhints(2) = 3 行分確保
	listH := innerH - 3
	if listH < 2 {
		listH = 2
	}

	var listContent string
	if m.loading {
		listContent = styleDim.Render("スキャン中...")
	} else if len(m.projects) == 0 {
		scanDir := m.cfg.Settings.ScanDirectory
		if scanDir == "" {
			home, _ := os.UserHomeDir()
			scanDir = home
		}
		listContent = styleDim.Render(fmt.Sprintf(
			"未登録のプロジェクトが見つかりませんでした\n\nスキャン先: %s\n\n設定ファイルの scan_directory を確認してください",
			config.ExpandPath(scanDir),
		))
	} else {
		listContent = m.renderProjectList(innerW, listH)
	}

	// listContent を listH 行に揃えて status/keys を常に下端に固定する
	lines := strings.Split(listContent, "\n")
	for len(lines) < listH {
		lines = append(lines, "")
	}
	listContent = strings.Join(lines[:listH], "\n")

	title := "スキャン"
	if len(m.selected) > 0 {
		title = fmt.Sprintf("スキャン  %d 件選択", len(m.selected))
	}

	status := m.renderScanStatusW(innerW)
	keys := m.renderScanKeysW(innerW)

	content := lipgloss.JoinVertical(lipgloss.Left, listContent, status, keys)
	return panelBorderColored(content, w, h, 0, title, colorYellow, colorYellow)
}

func (m *ScanModel) renderScanStatusW(w int) string {
	if m.statusIsErr {
		return styleStatusErr.Width(w).Render(m.status)
	}
	return styleStatusOk.Width(w).Render(m.status)
}

func (m *ScanModel) renderScanKeysW(w int) string {
	type hint struct{ key, desc string }
	hints := []hint{
		{"Space", "選択"}, {"a", "全選択"}, {"Enter", "追加"}, {"Esc", "戻る"},
	}
	var parts []string
	for _, h := range hints {
		parts = append(parts, styleKeyDesc.Render(h.desc)+styleKeySep.Render(": ")+styleKeyName.Render(h.key))
	}
	sep := styleKeySep.Render("  |  ")
	topSep := styleKeyBarSep.Render(strings.Repeat("─", w))
	line := styleKeyBarPad.Render(strings.Join(parts, sep))
	return topSep + "\n" + line
}

func (m *ScanModel) renderProjectList(totalW, innerH int) string {
	var lines []string
	lineIdx := 0

	// 通常プロジェクト
	for i, p := range m.projects {
		if lineIdx >= innerH {
			break
		}

		check := styleDim.Render("[ ]")
		if m.selected[i] {
			check = styleGreenBold.Render("[✓]")
		}

		path := p.Path
		maxPathW := totalW - 26
		if maxPathW > 0 && len(path) > maxPathW {
			path = "…" + path[len(path)-maxPathW+1:]
		}

		if i == m.cursor {
			rawName := fmt.Sprintf("%-20s", p.Name)
			checkRaw := "[ ]"
			if m.selected[i] {
				checkRaw = "[✓]"
			}
			lines = append(lines, styleSelectedItem.Width(totalW).Render(
				fmt.Sprintf("%s %s  %s", checkRaw, rawName, path),
			))
		} else {
			line := fmt.Sprintf("%s %-20s  %s", check, styleNormal.Render(p.Name), styleDim.Render(path))
			lines = append(lines, line)
		}
		lineIdx++
	}

	return strings.Join(lines, "\n")
}

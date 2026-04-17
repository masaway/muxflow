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
	skipped  []config.ScannedProject
	cursor   int
	selected map[int]bool

	showSkipped bool

	status      string
	statusIsErr bool
	done        bool
	loading     bool
}

// ─── メッセージ ───────────────────────────────────────────────────────────────

type scanLoadedMsg struct {
	newProjects     []config.ScannedProject
	skippedProjects []config.ScannedProject
	err             error
}

type scanSavedMsg struct{ err error }

type scanSkipMsg struct{ err error }

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
		return scanLoadedMsg{newProjects: result.New, skippedProjects: result.Skipped, err: err}
	}
}

// ─── カーソル計算ヘルパー ────────────────────────────────────────────────────

// totalItems はカーソルが動ける全アイテム数を返す
func (m *ScanModel) totalItems() int {
	n := len(m.projects)
	if len(m.skipped) > 0 {
		n++ // スキップ済みヘッダー
		if m.showSkipped {
			n += len(m.skipped)
		}
	}
	return n
}

// isHeader はカーソル位置がスキップ済みヘッダーかどうか
func (m *ScanModel) isHeader(idx int) bool {
	return len(m.skipped) > 0 && idx == len(m.projects)
}

// skippedIdx はカーソル位置に対応する m.skipped のインデックスを返す（-1: 対象外）
func (m *ScanModel) skippedIdx(cursorPos int) int {
	if !m.showSkipped || len(m.skipped) == 0 {
		return -1
	}
	si := cursorPos - len(m.projects) - 1
	if si >= 0 && si < len(m.skipped) {
		return si
	}
	return -1
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
			m.skipped = msg.skippedProjects
			m.updateStatus()
		}

	case scanSavedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("保存エラー: %s", msg.err)
			m.statusIsErr = true
		} else {
			m.done = true
		}

	case scanSkipMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("保存エラー: %s", msg.err)
			m.statusIsErr = true
		} else {
			m.updateStatus()
			m.statusIsErr = false
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *ScanModel) updateStatus() {
	if len(m.projects) == 0 && len(m.skipped) == 0 {
		m.status = "新規プロジェクトは見つかりませんでした"
	} else if len(m.skipped) > 0 {
		m.status = fmt.Sprintf("%d 件の未登録プロジェクトを発見  /  スキップ済み: %d 件", len(m.projects), len(m.skipped))
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
		if m.isHeader(m.cursor) {
			m.showSkipped = !m.showSkipped
			break
		}
		if m.cursor < len(m.projects) {
			m.selected[m.cursor] = !m.selected[m.cursor]
			if m.cursor < len(m.projects)-1 {
				m.cursor++
			}
		}

	case "a":
		// 全選択/全解除（通常プロジェクトのみ）
		if len(m.selected) == len(m.projects) {
			m.selected = map[int]bool{}
		} else {
			for i := range m.projects {
				m.selected[i] = true
			}
		}

	case "x":
		if m.isHeader(m.cursor) {
			m.showSkipped = !m.showSkipped
			break
		}
		if si := m.skippedIdx(m.cursor); si >= 0 {
			// スキップ解除
			return m, m.unskipCmd(si)
		}
		if m.cursor < len(m.projects) {
			// スキップに追加
			return m, m.skipCmd(m.cursor)
		}

	case "enter":
		if m.isHeader(m.cursor) {
			m.showSkipped = !m.showSkipped
			break
		}
		if len(m.selected) == 0 {
			m.status = "プロジェクトが選択されていません"
			m.statusIsErr = true
			break
		}
		return m, m.saveCmd()
	}
	return m, nil
}

func (m *ScanModel) skipCmd(idx int) tea.Cmd {
	p := m.projects[idx]
	// 選択状態をずらす
	newSelected := map[int]bool{}
	for i, sel := range m.selected {
		if i < idx && sel {
			newSelected[i] = true
		} else if i > idx && sel {
			newSelected[i-1] = true
		}
	}
	m.selected = newSelected
	// projects から取り出して skipped へ
	m.projects = append(m.projects[:idx], m.projects[idx+1:]...)
	m.skipped = append(m.skipped, p)
	if m.cursor >= m.totalItems() && m.cursor > 0 {
		m.cursor--
	}
	// config に保存
	m.cfg.SkippedPaths = append(m.cfg.SkippedPaths, p.Path)
	cfg := m.cfg
	return func() tea.Msg {
		return scanSkipMsg{err: config.Save(cfg)}
	}
}

func (m *ScanModel) unskipCmd(si int) tea.Cmd {
	p := m.skipped[si]
	// skipped から取り出して projects へ
	m.skipped = append(m.skipped[:si], m.skipped[si+1:]...)
	m.projects = append(m.projects, p)
	// SkippedPaths から削除
	for i, sp := range m.cfg.SkippedPaths {
		if sp == p.Path {
			m.cfg.SkippedPaths = append(m.cfg.SkippedPaths[:i], m.cfg.SkippedPaths[i+1:]...)
			break
		}
	}
	// カーソルがずれないよう調整
	if m.cursor >= m.totalItems() && m.cursor > 0 {
		m.cursor--
	}
	cfg := m.cfg
	return func() tea.Msg {
		return scanSkipMsg{err: config.Save(cfg)}
	}
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
	} else if len(m.projects) == 0 && len(m.skipped) == 0 {
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
		{"Space", "選択"}, {"a", "全選択"}, {"x", "スキップ"}, {"Enter", "追加"}, {"Esc", "戻る"},
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

	// スキップ済みセクション
	if len(m.skipped) > 0 && lineIdx < innerH {
		arrow := "▶"
		if m.showSkipped {
			arrow = "▼"
		}
		headerText := fmt.Sprintf("%s スキップ済み (%d)", arrow, len(m.skipped))
		headerCursorIdx := len(m.projects)

		if m.cursor == headerCursorIdx {
			lines = append(lines, styleSelectedItem.Width(totalW).Render(headerText))
		} else {
			lines = append(lines, styleDimBold.Render(headerText))
		}
		lineIdx++

		if m.showSkipped {
			for si, p := range m.skipped {
				if lineIdx >= innerH {
					break
				}
				cursorPos := len(m.projects) + 1 + si

				path := p.Path
				maxPathW := totalW - 26
				if maxPathW > 0 && len(path) > maxPathW {
					path = "…" + path[len(path)-maxPathW+1:]
				}

				skipMark := styleDim.Render("[S]")

				if m.cursor == cursorPos {
					lines = append(lines, styleSelectedItem.Width(totalW).Render(
						fmt.Sprintf("%s %-20s  %s", "[S]", p.Name, path),
					))
				} else {
					line := fmt.Sprintf("%s %-20s  %s", skipMark, styleDim.Render(p.Name), styleDim.Render(path))
					lines = append(lines, line)
				}
				lineIdx++
			}
		}
	}

	return strings.Join(lines, "\n")
}

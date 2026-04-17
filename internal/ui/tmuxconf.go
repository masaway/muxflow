package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/masaway/muxflow/internal/tmuxconf"
)

type tmuxConfState int

const (
	tmuxConfStateSelect  tmuxConfState = iota // 初期選択画面
	tmuxConfStateConfirm                      // 既存ファイルあり確認ダイアログ
	tmuxConfStateDone                         // 適用完了
	tmuxConfStatePreview                      // 設定内容プレビュー
)

// TmuxConfModel は tmux 推奨設定画面
type TmuxConfModel struct {
	width, height int
	state         tmuxConfState
	done          bool

	confirmCursor int // 0=backup+apply, 1=overwrite, 2=cancel
	doneCursor    int // 0=continue, 1=restore
	wasBackedUp   bool

	previewScroll int
	previewLines  []string

	status      string
	statusIsErr bool
}

type tmuxConfAppliedMsg struct {
	backedUp bool
	err      error
}

type tmuxConfRestoredMsg struct {
	err error
}

func NewTmuxConfModel() *TmuxConfModel {
	lines := strings.Split(strings.TrimRight(tmuxconf.RecommendedConfig, "\n"), "\n")
	return &TmuxConfModel{
		previewLines: lines,
	}
}

func (m *TmuxConfModel) Init() tea.Cmd { return nil }

func (m *TmuxConfModel) IsDone() bool { return m.done }

func (m *TmuxConfModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

func (m *TmuxConfModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tmuxConfAppliedMsg:
		if msg.err != nil {
			m.status = "エラー: " + msg.err.Error()
			m.statusIsErr = true
			m.state = tmuxConfStateSelect
		} else {
			m.wasBackedUp = msg.backedUp
			m.doneCursor = 0
			m.state = tmuxConfStateDone
			m.status = ""
		}
		return m, nil

	case tmuxConfRestoredMsg:
		if msg.err != nil {
			m.status = "復元エラー: " + msg.err.Error()
			m.statusIsErr = true
		} else {
			m.done = true
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case tmuxConfStateSelect:
			return m.updateSelect(msg)
		case tmuxConfStateConfirm:
			return m.updateConfirm(msg)
		case tmuxConfStateDone:
			return m.updateDone(msg)
		case tmuxConfStatePreview:
			return m.updatePreview(msg)
		}
	}
	return m, nil
}

func (m *TmuxConfModel) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if tmuxconf.Exists() {
			m.confirmCursor = 0
			m.state = tmuxConfStateConfirm
		} else {
			return m, applyTmuxConfCmd(false)
		}
	case "p":
		m.previewScroll = 0
		m.state = tmuxConfStatePreview
	case "q", "esc", "ctrl+c":
		m.done = true
	}
	return m, nil
}

func (m *TmuxConfModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	choices := 3
	switch msg.String() {
	case "j", "down":
		m.confirmCursor = (m.confirmCursor + 1) % choices
	case "k", "up":
		m.confirmCursor = (m.confirmCursor - 1 + choices) % choices
	case "enter":
		switch m.confirmCursor {
		case 0:
			return m, applyTmuxConfCmd(true)
		case 1:
			return m, applyTmuxConfCmd(false)
		case 2:
			m.state = tmuxConfStateSelect
		}
	case "esc":
		m.state = tmuxConfStateSelect
	case "ctrl+c":
		m.done = true
	}
	return m, nil
}

func (m *TmuxConfModel) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	choices := 1
	if m.wasBackedUp {
		choices = 2
	}
	switch msg.String() {
	case "j", "down":
		m.doneCursor = (m.doneCursor + 1) % choices
	case "k", "up":
		m.doneCursor = (m.doneCursor - 1 + choices) % choices
	case "enter":
		if m.doneCursor == 1 && m.wasBackedUp {
			return m, restoreTmuxConfCmd()
		}
		m.done = true
	case "esc":
		m.done = true
	case "ctrl+c":
		m.done = true
	}
	return m, nil
}

func (m *TmuxConfModel) updatePreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := len(m.previewLines) - (m.height - 6)
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch msg.String() {
	case "j", "down":
		if m.previewScroll < maxScroll {
			m.previewScroll++
		}
	case "k", "up":
		if m.previewScroll > 0 {
			m.previewScroll--
		}
	case "q", "esc":
		m.state = tmuxConfStateSelect
	case "ctrl+c":
		m.done = true
	}
	return m, nil
}

func applyTmuxConfCmd(backup bool) tea.Cmd {
	return func() tea.Msg {
		backedUp := backup && tmuxconf.Exists()
		err := tmuxconf.Apply(backup)
		return tmuxConfAppliedMsg{backedUp: backedUp, err: err}
	}
}

func restoreTmuxConfCmd() tea.Cmd {
	return func() tea.Msg {
		return tmuxConfRestoredMsg{err: tmuxconf.Restore()}
	}
}

func (m *TmuxConfModel) View() string {
	panelH := m.height - 3
	var body string

	switch m.state {
	case tmuxConfStateSelect:
		body = m.viewSelect()
	case tmuxConfStateConfirm:
		body = m.viewConfirm()
	case tmuxConfStateDone:
		body = m.viewDone()
	case tmuxConfStatePreview:
		body = m.viewPreview()
	}

	var title string
	if m.state == tmuxConfStatePreview {
		title = "設定内容"
	} else {
		title = "tmux 推奨設定"
	}

	panel := panelBorder(body, m.width, panelH, 0, title, true)

	var statusLine string
	if m.status != "" {
		if m.statusIsErr {
			statusLine = styleStatusErr.Width(m.width).Render(m.status)
		} else {
			statusLine = styleStatusOk.Width(m.width).Render(m.status)
		}
	} else {
		statusLine = styleStatusOk.Width(m.width).Render("")
	}

	return lipgloss.JoinVertical(lipgloss.Left, panel, statusLine, m.renderKeys())
}

func (m *TmuxConfModel) viewSelect() string {
	desc := styleNormal.Render("ツール制作者おすすめの tmux 設定を適用できます。")
	featTitle := styleDim.Render("主な設定:")
	features := []string{
		"  ・Prefix: Ctrl+S",
		"  ・hjkl でペイン移動 / リサイズ",
		"  ・ステータスバー (git branch 表示)",
		"  ・PREFIX+m で muxflow 起動",
		"  ・マウスサポート有効",
		"  ・スクロール履歴: 50,000行",
		"  ・\\ / - でペイン分割",
	}
	var featureLines []string
	for _, f := range features {
		featureLines = append(featureLines, styleDim.Render(f))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		desc,
		"",
		featTitle,
		strings.Join(featureLines, "\n"),
	)
}

func (m *TmuxConfModel) viewConfirm() string {
	warning := styleYellow.Render("~/.tmux.conf が既に存在します")

	type choice struct {
		label string
		sub   string
	}
	choices := []choice{
		{"バックアップして適用", "(.tmux.conf.bak に退避)"},
		{"上書き（バックアップなし）", ""},
		{"キャンセル", ""},
	}

	var lines []string
	lines = append(lines, "", warning, "")
	for i, c := range choices {
		cursor := "  "
		if i == m.confirmCursor {
			cursor = styleGreen.Render("▶ ")
		}
		label := styleNormal.Render(c.label)
		if i == m.confirmCursor {
			label = styleGreenBold.Render(c.label)
		}
		lines = append(lines, cursor+label)
		if c.sub != "" {
			lines = append(lines, "   "+styleDim.Render(c.sub))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m *TmuxConfModel) viewDone() string {
	ok := styleGreen.Render("✓ ~/.tmux.conf を適用しました")

	var lines []string
	lines = append(lines, "", ok, "")

	if m.wasBackedUp {
		lines = append(lines,
			styleDim.Render("元の設定は ~/.tmux.conf.bak に保存されています。"),
			"",
		)
	}

	type choice struct{ label string }
	choices := []choice{{"そのまま続ける"}}
	if m.wasBackedUp {
		choices = append(choices, choice{"元の設定に戻す"})
	}

	for i, c := range choices {
		cursor := "  "
		if i == m.doneCursor {
			cursor = styleGreen.Render("▶ ")
		}
		label := styleNormal.Render(c.label)
		if i == m.doneCursor {
			label = styleGreenBold.Render(c.label)
		}
		lines = append(lines, cursor+label, "")
	}

	return strings.Join(lines, "\n")
}

func (m *TmuxConfModel) viewPreview() string {
	innerH := m.height - 6
	if innerH < 1 {
		innerH = 1
	}

	end := m.previewScroll + innerH
	if end > len(m.previewLines) {
		end = len(m.previewLines)
	}

	visible := m.previewLines[m.previewScroll:end]
	innerW := m.width - 4

	var rendered []string
	for _, line := range visible {
		if lipgloss.Width(line) > innerW {
			line = line[:innerW]
		}
		rendered = append(rendered, styleDim.Render(line))
	}
	return strings.Join(rendered, "\n")
}

func (m *TmuxConfModel) renderKeys() string {
	var hints []struct{ key, desc string }

	switch m.state {
	case tmuxConfStateSelect:
		hints = []struct{ key, desc string }{
			{"Enter", "適用"},
			{"p", "内容を表示"},
			{"q / Esc", "スキップ"},
		}
	case tmuxConfStateConfirm:
		hints = []struct{ key, desc string }{
			{"j/k", "移動"},
			{"Enter", "決定"},
			{"Esc", "戻る"},
		}
	case tmuxConfStateDone:
		hints = []struct{ key, desc string }{
			{"Enter", "決定"},
			{"Esc", "そのまま続ける"},
		}
	case tmuxConfStatePreview:
		hints = []struct{ key, desc string }{
			{"j/k", "スクロール"},
			{"q / Esc", "戻る"},
		}
	}

	var parts []string
	for _, h := range hints {
		parts = append(parts, styleKeyDesc.Render(h.desc)+styleKeySep.Render(": ")+styleKeyName.Render(h.key))
	}
	sep := styleKeySep.Render("  |  ")
	topSep := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width))
	line := lipgloss.NewStyle().Padding(0, 1).Render(strings.Join(parts, sep))
	return topSep + "\n" + line
}

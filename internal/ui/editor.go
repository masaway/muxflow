package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/masaway/muxflow/internal/config"
)

var windowLayouts = []string{
	"even-horizontal",
	"even-vertical",
	"main-horizontal",
	"main-vertical",
	"tiled",
}

func isNamedLayout(layout string) bool {
	for _, l := range windowLayouts {
		if l == layout {
			return true
		}
	}
	return false
}

type editorPanel int

const (
	editorPanelWindows editorPanel = iota
	editorPanelPanes
)

type editorFormMode int

const (
	editorFormNone editorFormMode = iota
	editorFormPane
	editorFormWindow
)

// EditorModel はウィンドウ/ペイン編集画面
type EditorModel struct {
	width, height int
	project       config.Project
	cfg           *config.Config
	projectIdx    int

	panel      editorPanel
	winCursor  int
	paneCursor int

	formMode editorFormMode
	paneForm paneFormState
	winForm  windowFormState

	status      string
	statusIsErr bool
	done        bool
	showHelp    bool
}

type paneFormState struct {
	dirOptions  []string
	dirCursor   int
	dirInput    textinput.Model
	dirInputMode bool
	cmd         textinput.Model
	execute     bool
	focus       int // 0=dir, 1=cmd, 2=execute
	isNew       bool
}

type windowFormState struct {
	name      textinput.Model
	layoutIdx int
	focus     int // 0=name, 1=layout
	isNew     bool
}

// ─── メッセージ ───────────────────────────────────────────────────────────────

type editorSavedMsg struct{ err error }

// ─── コンストラクタ ───────────────────────────────────────────────────────────

func NewEditor(cfg *config.Config, projectIdx int) *EditorModel {
	p := cfg.Projects[projectIdx]
	p.MigrateFromCommands()
	return &EditorModel{
		project:    p,
		cfg:        cfg,
		projectIdx: projectIdx,
	}
}

func (m *EditorModel) Init() tea.Cmd { return nil }

func (m *EditorModel) IsDone() bool { return m.done }

func (m *EditorModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

func listProjectDirs(projPath string) []string {
	expanded := config.ExpandPath(projPath)
	dirs := []string{"."}
	entries, err := os.ReadDir(expanded)
	if err != nil {
		return dirs
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirs = append(dirs, e.Name())
	}
	return dirs
}

func newPaneForm(p config.Pane, proj config.Project, isNew bool) paneFormState {
	opts := listProjectDirs(proj.Path)
	cursor := 0
	target := p.Dir
	if target == "" {
		target = "."
	}
	found := false
	for i, d := range opts {
		if d == target {
			cursor = i
			found = true
			break
		}
	}

	dirInput := textinput.New()
	dirInput.Placeholder = "例: src/api"
	dirInput.Width = 38

	// リストに存在しないパスは最初からテキスト入力モードで開く
	dirInputMode := false
	if !found {
		dirInputMode = true
		dirInput.SetValue(target)
		dirInput.CursorEnd()
		dirInput.Focus()
	}

	cmd := textinput.New()
	cmd.Placeholder = "例: npm run dev"
	cmd.SetValue(p.Command)
	cmd.Width = 38

	return paneFormState{dirOptions: opts, dirCursor: cursor, dirInput: dirInput, dirInputMode: dirInputMode, cmd: cmd, execute: p.Execute, focus: 0, isNew: isNew}
}

func newWindowForm(w config.Window, isNew bool) windowFormState {
	name := textinput.New()
	name.Placeholder = "例: dev"
	name.SetValue(w.Name)
	name.Width = 28
	name.Focus()

	layoutIdx := 0
	for i, l := range windowLayouts {
		if l == w.Layout {
			layoutIdx = i
			break
		}
	}
	return windowFormState{name: name, layoutIdx: layoutIdx, focus: 0, isNew: isNew}
}

// ─── ヘルパー ─────────────────────────────────────────────────────────────────

func (m *EditorModel) selectedWindow() *config.Window {
	if m.winCursor >= len(m.project.Windows) {
		return nil
	}
	return &m.project.Windows[m.winCursor]
}

func (m *EditorModel) selectedPane() *config.Pane {
	w := m.selectedWindow()
	if w == nil || m.paneCursor >= len(w.Panes) {
		return nil
	}
	return &w.Panes[m.paneCursor]
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m *EditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 保存完了メッセージ
	if saved, ok := msg.(editorSavedMsg); ok {
		if saved.err != nil {
			m.status = fmt.Sprintf("保存エラー: %s", saved.err)
			m.statusIsErr = true
		} else {
			m.status = "✓ 保存しました (personal.json)"
			m.statusIsErr = false
		}
		return m, nil
	}

	// フォーム表示中はフォームに渡す
	if m.formMode != editorFormNone {
		return m.updateForm(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		return m.handleKey(key)
	}
	return m, nil
}

func (m *EditorModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			if m.formMode == editorFormPane && m.paneForm.dirInputMode && m.paneForm.focus == 0 {
				m.paneForm.dirInputMode = false
				m.paneForm.dirInput.Blur()
				return m, nil
			}
			m.formMode = editorFormNone
			return m, nil

		case "enter", "w", "ctrl+s":
			// テキスト入力フォーカス中は w をそのままテキスト入力に渡す
			if key.String() == "w" {
				if m.formMode == editorFormPane && (m.paneForm.dirInputMode || m.paneForm.focus == 1) {
					break
				}
				if m.formMode == editorFormPane && m.paneForm.focus == 2 {
					// execute フィールドは通常保存
				} else if m.formMode == editorFormWindow && m.winForm.focus == 0 {
					break
				}
			}
			if m.formMode == editorFormPane {
				return m.commitPaneForm()
			}
			return m.commitWindowForm()

		case "e":
			if m.formMode == editorFormPane && m.paneForm.focus == 0 && !m.paneForm.dirInputMode {
				current := "."
				if len(m.paneForm.dirOptions) > 0 {
					current = m.paneForm.dirOptions[m.paneForm.dirCursor]
				}
				m.paneForm.dirInput.SetValue(current)
				m.paneForm.dirInput.CursorEnd()
				m.paneForm.dirInput.Focus()
				m.paneForm.dirInputMode = true
				return m, nil
			}

		case "tab":
			if m.formMode == editorFormPane && m.paneForm.dirInputMode {
				// dirInputMode 中は Tab で次フィールドへ（値は保持したまま）
				m.paneForm.dirInput.Blur()
				m.cyclePaneFormFocus(true)
				return m, nil
			}
			if m.formMode == editorFormPane {
				m.cyclePaneFormFocus(true)
			} else {
				m.cycleWindowFormFocus(true)
			}
			return m, nil

		case "shift+tab":
			if m.formMode == editorFormPane && m.paneForm.dirInputMode {
				// dirInputMode 中は Shift+Tab で前フィールドへ（値は保持したまま）
				m.paneForm.dirInput.Blur()
				m.cyclePaneFormFocus(false)
				return m, nil
			}
			if m.formMode == editorFormPane {
				m.cyclePaneFormFocus(false)
			} else {
				m.cycleWindowFormFocus(false)
			}
			return m, nil

		case " ":
			if m.formMode == editorFormPane && m.paneForm.focus == 2 {
				m.paneForm.execute = !m.paneForm.execute
				return m, nil
			}

		case "left", "h":
			if m.formMode == editorFormWindow && m.winForm.focus == 1 {
				m.winForm.layoutIdx = (m.winForm.layoutIdx - 1 + len(windowLayouts)) % len(windowLayouts)
				return m, nil
			}

		case "right", "l":
			if m.formMode == editorFormWindow && m.winForm.focus == 1 {
				m.winForm.layoutIdx = (m.winForm.layoutIdx + 1) % len(windowLayouts)
				return m, nil
			}

		case "j", "down":
			if m.formMode == editorFormPane && m.paneForm.focus == 0 && !m.paneForm.dirInputMode {
				n := len(m.paneForm.dirOptions)
				if n > 0 && m.paneForm.dirCursor < n-1 {
					m.paneForm.dirCursor++
				}
				return m, nil
			}

		case "k", "up":
			if m.formMode == editorFormPane && m.paneForm.focus == 0 && !m.paneForm.dirInputMode {
				if m.paneForm.dirCursor > 0 {
					m.paneForm.dirCursor--
				}
				return m, nil
			}
		}
	}

	// テキスト入力に転送
	var cmd tea.Cmd
	if m.formMode == editorFormPane {
		if m.paneForm.dirInputMode && m.paneForm.focus == 0 {
			m.paneForm.dirInput, cmd = m.paneForm.dirInput.Update(msg)
		} else if m.paneForm.focus == 1 {
			m.paneForm.cmd, cmd = m.paneForm.cmd.Update(msg)
		}
	} else {
		if m.winForm.focus == 0 {
			m.winForm.name, cmd = m.winForm.name.Update(msg)
		}
	}
	return m, cmd
}

func (m *EditorModel) cyclePaneFormFocus(fwd bool) {
	n := 3 // dir, cmd, execute
	if fwd {
		m.paneForm.focus = (m.paneForm.focus + 1) % n
	} else {
		m.paneForm.focus = (m.paneForm.focus - 1 + n) % n
	}
	m.paneForm.cmd.Blur()
	m.paneForm.dirInput.Blur()
	if m.paneForm.focus == 1 {
		m.paneForm.cmd.Focus()
	} else if m.paneForm.focus == 0 && m.paneForm.dirInputMode {
		m.paneForm.dirInput.Focus()
	}
}

func (m *EditorModel) cycleWindowFormFocus(fwd bool) {
	n := 2 // name, layout
	if fwd {
		m.winForm.focus = (m.winForm.focus + 1) % n
	} else {
		m.winForm.focus = (m.winForm.focus - 1 + n) % n
	}
	m.winForm.name.Blur()
	if m.winForm.focus == 0 {
		m.winForm.name.Focus()
	}
}

func (m *EditorModel) commitPaneForm() (tea.Model, tea.Cmd) {
	w := m.selectedWindow()
	if w == nil {
		m.formMode = editorFormNone
		return m, nil
	}
	dir := "."
	if m.paneForm.dirInputMode {
		if v := strings.TrimSpace(m.paneForm.dirInput.Value()); v != "" {
			dir = v
		}
	} else if len(m.paneForm.dirOptions) > 0 {
		dir = m.paneForm.dirOptions[m.paneForm.dirCursor]
	}
	pane := config.Pane{
		Dir:     dir,
		Command: strings.TrimSpace(m.paneForm.cmd.Value()),
		Execute: m.paneForm.execute,
	}
	if m.paneForm.isNew {
		w.Panes = append(w.Panes, pane)
		m.paneCursor = len(w.Panes) - 1
	} else {
		w.Panes[m.paneCursor] = pane
	}
	m.formMode = editorFormNone
	return m, m.saveCmd()
}

func (m *EditorModel) commitWindowForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.winForm.name.Value())
	if name == "" {
		name = "window"
	}
	layout := windowLayouts[m.winForm.layoutIdx]
	if m.winForm.isNew {
		m.project.Windows = append(m.project.Windows, config.Window{
			Name:   name,
			Layout: layout,
			Panes:  []config.Pane{{Dir: "."}},
		})
		m.winCursor = len(m.project.Windows) - 1
		m.paneCursor = 0
	} else {
		m.project.Windows[m.winCursor].Name = name
		m.project.Windows[m.winCursor].Layout = layout
	}
	m.formMode = editorFormNone
	return m, m.saveCmd()
}

func (m *EditorModel) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		switch key.String() {
		case "?", "esc", "q":
			m.showHelp = false
		}
		return m, nil
	}

	switch key.String() {
	case "?":
		m.showHelp = true

	case "esc":
		m.done = true

	case "w", "ctrl+s":
		return m, m.saveCmd()

	case "tab":
		if m.panel == editorPanelWindows {
			m.panel = editorPanelPanes
		} else {
			m.panel = editorPanelWindows
		}

	case "h":
		m.panel = editorPanelWindows

	case "l":
		m.panel = editorPanelPanes

	case "1":
		m.panel = editorPanelWindows

	case "2":
		m.panel = editorPanelPanes

	case "j", "down":
		if m.panel == editorPanelWindows {
			if m.winCursor < len(m.project.Windows)-1 {
				m.winCursor++
				m.paneCursor = 0
			}
		} else {
			w := m.selectedWindow()
			if w != nil && m.paneCursor < len(w.Panes)-1 {
				m.paneCursor++
			}
		}

	case "k", "up":
		if m.panel == editorPanelWindows {
			if m.winCursor > 0 {
				m.winCursor--
				m.paneCursor = 0
			}
		} else {
			if m.paneCursor > 0 {
				m.paneCursor--
			}
		}

	case "a":
		if m.panel == editorPanelWindows {
			m.winForm = newWindowForm(config.Window{
				Name:   fmt.Sprintf("window%d", len(m.project.Windows)),
				Layout: "even-horizontal",
			}, true)
			m.formMode = editorFormWindow
		} else {
			w := m.selectedWindow()
			if w == nil {
				break
			}
			m.paneForm = newPaneForm(config.Pane{Dir: "."}, m.project, true)
			m.formMode = editorFormPane
		}

	case "e":
		if m.panel == editorPanelWindows {
			w := m.selectedWindow()
			if w != nil {
				m.winForm = newWindowForm(*w, false)
				m.formMode = editorFormWindow
			}
		} else {
			p := m.selectedPane()
			if p != nil {
				m.paneForm = newPaneForm(*p, m.project, false)
				m.formMode = editorFormPane
			}
		}

	case "d":
		if m.panel == editorPanelWindows {
			if len(m.project.Windows) > 1 {
				m.project.Windows = append(
					m.project.Windows[:m.winCursor],
					m.project.Windows[m.winCursor+1:]...,
				)
				if m.winCursor >= len(m.project.Windows) {
					m.winCursor = len(m.project.Windows) - 1
				}
				m.paneCursor = 0
				m.status = "ウィンドウを削除しました"
				return m, m.saveCmd()
			} else {
				m.status = "最後のウィンドウは削除できません"
			}
		} else {
			w := m.selectedWindow()
			if w != nil {
				if len(w.Panes) > 1 {
					w.Panes = append(w.Panes[:m.paneCursor], w.Panes[m.paneCursor+1:]...)
					if m.paneCursor >= len(w.Panes) {
						m.paneCursor = len(w.Panes) - 1
					}
					m.status = "ペインを削除しました"
					return m, m.saveCmd()
				} else {
					m.status = "最後のペインは削除できません"
				}
			}
		}
	}
	return m, nil
}

func (m *EditorModel) saveCmd() tea.Cmd {
	// メモリ更新（mainゴルーチン内）
	m.cfg.Projects[m.projectIdx] = m.project
	cfg := m.cfg
	return func() tea.Msg {
		if err := config.Save(cfg); err != nil {
			return editorSavedMsg{err: err}
		}
		return editorSavedMsg{}
	}
}

// truncateStr は表示幅単位で文字列を切り詰め、超過時は "..." を付ける
func truncateStr(s string, maxW int) string {
	if runewidth.StringWidth(s) <= maxW {
		return s
	}
	if maxW <= 3 {
		w := 0
		var buf []rune
		for _, r := range s {
			rw := runewidth.RuneWidth(r)
			if w+rw > maxW {
				break
			}
			buf = append(buf, r)
			w += rw
		}
		return string(buf)
	}
	w := 0
	var buf []rune
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > maxW-3 {
			break
		}
		buf = append(buf, r)
		w += rw
	}
	return string(buf) + "..."
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *EditorModel) View() string {
	dialogW := m.width - 12
	dialogH := m.height - 6
	if dialogW > 100 {
		dialogW = 100
	}
	if dialogW < 54 {
		dialogW = 54
	}
	if dialogH > 28 {
		dialogH = 28
	}
	if dialogH < 12 {
		dialogH = 12
	}

	dialog := m.buildEditorDialog(dialogW, dialogH)

	if m.formMode != editorFormNone {
		var box string
		if m.formMode == editorFormPane {
			box = m.renderPaneFormBox()
		} else {
			box = m.renderWindowFormBox()
		}
		return overlayCenter(dialog, box, dialogW, dialogH)
	}

	if m.showHelp {
		return overlayCenter(dialog, m.renderEditorHelp(), dialogW, dialogH)
	}

	return dialog
}

func (m *EditorModel) buildEditorDialog(w, h int) string {
	innerW := w - 2
	innerH := h - 2

	// status(1) + keyhints(2) = 3 行分確保
	panelH := innerH - 3
	if panelH < 4 {
		panelH = 4
	}

	leftW := innerW / 2
	if leftW < 22 {
		leftW = 22
	}
	rightW := innerW - leftW

	left := m.renderWindowPanel(leftW, panelH)
	right := m.renderPanePanel(rightW, panelH)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	statusBar := m.renderEditorStatus(innerW)
	keyHints := m.renderEditorKeyHints(innerW)

	content := lipgloss.JoinVertical(lipgloss.Left, panels, statusBar, keyHints)

	return panelBorderColored(content, w, h, 0, "編集: "+m.project.Name, colorYellow, colorYellow)
}

func (m *EditorModel) renderEditorStatus(width int) string {
	if m.statusIsErr {
		return styleStatusErr.Width(width).Render(m.status)
	}
	return styleStatusOk.Width(width).Render(m.status)
}

func (m *EditorModel) renderEditorKeyHints(width int) string {
	type hint struct{ key, desc string }
	hints := []hint{
		{"a", "追加"}, {"e", "編集"}, {"d", "削除"},
		{"Tab", "切替"}, {"?", "ヘルプ"}, {"Esc", "戻る"},
	}
	var parts []string
	for _, h := range hints {
		parts = append(parts, styleKeyDesc.Render(h.desc)+styleKeySep.Render(": ")+styleKeyName.Render(h.key))
	}
	sep := styleKeySep.Render("  |  ")
	topSep := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", width))
	line := lipgloss.NewStyle().Padding(0, 1).Render(strings.Join(parts, sep))
	return topSep + "\n" + line
}

func (m *EditorModel) renderWindowPanel(totalW, totalH int) string {
	innerW := totalW - 2

	var lines []string
	for i, w := range m.project.Windows {
		layoutSuffix := ""
		if isNamedLayout(w.Layout) {
			layoutSuffix = "  " + styleDim.Render(w.Layout)
		} else if w.Layout != "" {
			layoutSuffix = "  " + styleDim.Render("[カスタム]")
		}
		if i == m.winCursor {
			if m.panel == editorPanelWindows {
				rawLayout := ""
				if isNamedLayout(w.Layout) {
					rawLayout = w.Layout
				} else if w.Layout != "" {
					rawLayout = "[カスタム]"
				}
				layoutSuffixW := 0
				if rawLayout != "" {
					layoutSuffixW = runewidth.StringWidth("  " + rawLayout)
				}
				nameMaxW := innerW - runewidth.StringWidth(" ▸ ") - layoutSuffixW
				if nameMaxW < 1 {
					nameMaxW = 1
				}
				truncatedName := truncateStr(w.Name, nameMaxW)
				nameRendered := lipgloss.NewStyle().
					Background(colorSelected).Foreground(colorBlue).Bold(true).
					Render(" ▸ " + truncatedName)
				layoutRendered := ""
				if rawLayout != "" {
					layoutRendered = lipgloss.NewStyle().
						Background(colorSelected).Foreground(lipgloss.Color("#497fab")).
						Render("  " + rawLayout)
				}
				usedW := runewidth.StringWidth(" ▸ "+truncatedName) + layoutSuffixW
				padW := innerW - usedW
				if padW < 0 {
					padW = 0
				}
				pad := lipgloss.NewStyle().Background(colorSelected).Render(strings.Repeat(" ", padW))
				lines = append(lines, nameRendered+layoutRendered+pad)
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(colorCyan).Width(innerW).Render(
					fmt.Sprintf(" ▸ %s%s", w.Name, layoutSuffix),
				))
			}
		} else {
			rawSuffix := ""
			if isNamedLayout(w.Layout) {
				rawSuffix = "  " + w.Layout
			} else if w.Layout != "" {
				rawSuffix = "  [カスタム]"
			}
			nameMaxW := innerW - 3 - runewidth.StringWidth(rawSuffix)
			if nameMaxW < 1 {
				nameMaxW = 1
			}
			lines = append(lines, fmt.Sprintf("   %s%s", styleNormal.Render(truncateStr(w.Name, nameMaxW)), layoutSuffix))
		}
	}

	return panelBorder(strings.Join(lines, "\n"), totalW, totalH, 1, "Windows", m.panel == editorPanelWindows)
}

func (m *EditorModel) renderPanePanel(totalW, totalH int) string {
	innerW := totalW - 2

	win := m.selectedWindow()
	panelTitle := "Panes"
	if win != nil {
		layoutPart := ""
		if isNamedLayout(win.Layout) {
			layoutPart = "  " + styleDim.Render(win.Layout)
		} else if win.Layout != "" {
			layoutPart = "  " + styleDim.Render("[カスタム]")
		}
		panelTitle = fmt.Sprintf("Panes  %s%s", styleCyan.Render(win.Name), layoutPart)
	}

	var lines []string
	if win != nil {
		for i, pane := range win.Panes {
			execChar := "▷"
			execStr := styleDim.Render(execChar)
			if pane.Execute {
				execChar = "▶"
				execStr = styleRed.Render(execChar)
			}
			dir := pane.Dir
			if dir == "" {
				dir = "."
			}
			cmd := pane.Command
			if cmd == "" {
				cmd = "—"
			}

			// プレフィックス幅: " pane N  X  " = 11 + digit数, botは "          " = 10
			paneNumW := len(fmt.Sprintf("%d", i))
			dirMaxW := innerW - 11 - paneNumW
			cmdMaxW := innerW - 10
			if dirMaxW < 4 {
				dirMaxW = 4
			}
			if cmdMaxW < 4 {
				cmdMaxW = 4
			}

			execBotPrefix := "          "
			if pane.Execute {
				execBotPrefix = "        ▶ "
			}
			top := fmt.Sprintf(" pane %d  %s  %s", i, execStr, styleDim.Render(truncateStr(dir, dirMaxW)))
			bot := fmt.Sprintf("%s%s", execBotPrefix, styleYellow.Render(truncateStr(cmd, cmdMaxW)))

			if i == m.paneCursor {
				if m.panel == editorPanelPanes {
					top = styleSelectedItem.Width(innerW).Render(
						fmt.Sprintf(" pane %d  %s  %s", i, execChar, truncateStr(dir, dirMaxW)),
					)
					bot = lipgloss.NewStyle().
						Background(colorBg2).Foreground(colorYellow).Width(innerW).
						Render(fmt.Sprintf("%s%s", execBotPrefix, truncateStr(cmd, cmdMaxW)))
				} else {
					top = lipgloss.NewStyle().Foreground(colorCyan).Render(top)
				}
			}
			lines = append(lines, top, bot, "")
		}
	}

	return panelBorder(strings.Join(lines, "\n"), totalW, totalH, 2, panelTitle, m.panel == editorPanelPanes)
}

// renderDirPicker はDir選択ピッカーをレンダリングする（最大5件表示）
func renderDirPicker(f paneFormState, formWidth int) string {
	focused := f.focus == 0

	// テキスト入力モード
	if f.dirInputMode {
		hint := styleDim.Render("  Esc でリストに戻る")
		return f.dirInput.View() + hint
	}

	const visRows = 5
	opts := f.dirOptions
	if len(opts) == 0 {
		hint := ""
		if focused {
			hint = "  " + styleDim.Render("e で直接入力")
		}
		return styleDim.Render("(なし)") + hint
	}
	cursor := f.dirCursor

	// スクロールウィンドウ計算
	start := 0
	if cursor >= visRows {
		start = cursor - visRows + 1
	}
	end := start + visRows
	if end > len(opts) {
		end = len(opts)
	}

	var rows []string
	for i := start; i < end; i++ {
		var line string
		if i == cursor {
			if focused {
				line = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("▶ " + opts[i])
			} else {
				line = lipgloss.NewStyle().Foreground(colorCyan).Render("▶ " + opts[i])
			}
		} else {
			line = styleDim.Render("  " + opts[i])
		}
		rows = append(rows, line)
	}

	hint := ""
	if focused {
		hint = "\n" + strings.Repeat(" ", 10) + styleDim.Render("e で直接入力")
	}

	return strings.Join(rows, "\n"+strings.Repeat(" ", 10)) + hint
}

func (m *EditorModel) renderPaneFormBox() string {
	f := m.paneForm
	w := 56

	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("ペイン編集") + "\n\n")

	// Dir picker
	dirLabel := styleDim.Render("Dir     ")
	if f.focus == 0 {
		dirLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Dir     ")
	}
	sb.WriteString(dirLabel + "  " + renderDirPicker(f, w) + "\n\n")

	// Command
	cmdLabel := styleDim.Render("Command ")
	if f.focus == 1 {
		cmdLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Command ")
	}
	sb.WriteString(cmdLabel + "  " + f.cmd.View() + "\n\n")

	// Execute
	execLabel := styleDim.Render("Execute ")
	if f.focus == 2 {
		execLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("Execute ")
	}
	var toggle string
	if f.execute {
		toggle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("[ ON  ]")
	} else {
		toggle = styleDim.Render("[ OFF ]")
	}
	hint := ""
	if f.focus == 2 {
		hint = styleDim.Render("  Space で切替")
	}
	sb.WriteString(execLabel + "  " + toggle + hint + "\n\n")

	// ヒント
	if f.dirInputMode {
		sb.WriteString(styleDim.Render("Esc リストに戻る  Enter 保存"))
	} else {
		sb.WriteString(styleDim.Render("Esc キャンセル  Enter 保存"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(w).
		Render(sb.String())
}

// layoutPreviewLines はレイアウトのASCIIアートプレビュー（各要素が1行）
var layoutPreviewLines = map[string][]string{
	"even-horizontal": {
		"┌─────┬─────┐",
		"│     │     │",
		"│     │     │",
		"└─────┴─────┘",
	},
	"even-vertical": {
		"┌───────────┐",
		"│           │",
		"├───────────┤",
		"│           │",
		"└───────────┘",
	},
	"main-horizontal": {
		"┌───────────┐",
		"│   main    │",
		"├─────┬─────┤",
		"│     │     │",
		"└─────┴─────┘",
	},
	"main-vertical": {
		"┌───────┬───┐",
		"│       │   │",
		"│ main  ├───┤",
		"│       │   │",
		"└───────┴───┘",
	},
	"tiled": {
		"┌─────┬─────┐",
		"│     │     │",
		"├─────┼─────┤",
		"│     │     │",
		"└─────┴─────┘",
	},
}

var layoutDescriptions = map[string]string{
	"even-horizontal": "ペインを左右に均等分割",
	"even-vertical":   "ペインを上下に均等分割",
	"main-horizontal": "上に広いメイン、下に小ペイン",
	"main-vertical":   "左に広いメイン、右に小ペイン",
	"tiled":           "グリッド状に配置",
}

func renderLayoutPreview(layout string, focused bool) string {
	var artLines []string
	var desc string
	if namedLines, ok := layoutPreviewLines[layout]; ok {
		artLines = namedLines
		desc = layoutDescriptions[layout]
	} else if layout != "" {
		artLines = renderCustomLayoutPreview(layout)
		desc = "[カスタム]"
	}
	if len(artLines) == 0 {
		return ""
	}

	color := colorFgDim
	if focused {
		color = colorCyan
	}
	previewStyle := lipgloss.NewStyle().Foreground(color)
	descStyle := lipgloss.NewStyle().Foreground(colorFgDim).Italic(true)

	var rows []string
	for i, line := range artLines {
		row := previewStyle.Render(line)
		if i == 1 {
			row += "  " + descStyle.Render(desc)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

// ─── カスタムレイアウトパーサー・レンダラー ────────────────────────────────────

type paneRect struct{ x, y, w, h int }

// parseTmuxLayout は tmux の #{window_layout} 文字列をパースし、
// レイアウト全体の寸法とリーフペイン一覧を返す。
func parseTmuxLayout(layout string) (totalW, totalH int, panes []paneRect) {
	s := layout
	// 先頭のチェックサム（4桁16進数 + カンマ）を除去
	if len(s) > 5 && s[4] == ',' {
		isHex := true
		for _, c := range s[:4] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isHex = false
				break
			}
		}
		if isHex {
			s = s[5:]
		}
	}
	totalW, totalH = parseLayoutFirstDims(s)
	parseLayoutNodeAt(s, 0, &panes)
	return
}

func parseLayoutFirstDims(s string) (w, h int) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		w = w*10 + int(s[i]-'0')
		i++
	}
	if i >= len(s) || s[i] != 'x' {
		return
	}
	i++
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		h = h*10 + int(s[i]-'0')
		i++
	}
	return
}

// parseLayoutNodeAt は位置 i からノードを1つパースし、消費した末尾の位置を返す。
// 子ノードを逐次パースするため、カンマの多義性（区切り vs WxH内）を正しく扱える。
func parseLayoutNodeAt(s string, i int, panes *[]paneRect) int {
	w, i := parseIntFrom(s, i)
	if i >= len(s) || s[i] != 'x' {
		return i
	}
	i++
	h, i := parseIntFrom(s, i)
	if i >= len(s) || s[i] != ',' {
		return i
	}
	i++
	x, i := parseIntFrom(s, i)
	if i >= len(s) || s[i] != ',' {
		return i
	}
	i++
	y, i := parseIntFrom(s, i)

	if i >= len(s) || s[i] == '}' || s[i] == ']' {
		// 文字列末尾 or 親の閉じカッコ直前 → リーフ
		*panes = append(*panes, paneRect{x, y, w, h})
		return i
	}

	switch s[i] {
	case '{', '[':
		closeChar := byte('}')
		if s[i] == '[' {
			closeChar = ']'
		}
		i++ // 開きカッコをスキップ
		for i < len(s) && s[i] != closeChar {
			i = parseLayoutNodeAt(s, i, panes)
			if i < len(s) && s[i] == ',' {
				i++ // 子ノード間のカンマをスキップ
			}
		}
		if i < len(s) {
			i++ // 閉じカッコをスキップ
		}
	case ',':
		// リーフ：カンマの後に pane_id が続く
		*panes = append(*panes, paneRect{x, y, w, h})
		i++ // カンマをスキップ
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++ // pane_id の数字をスキップ
		}
	}
	return i
}

func parseIntFrom(s string, i int) (int, int) {
	n := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}
	return n, i
}

// renderCustomLayoutPreview はtmuxレイアウト文字列からASCIIアート（13x5）を生成する。
// 各ペインの境界区間だけに線を引くため、L字・T字レイアウトも正確に表現できる。
func renderCustomLayoutPreview(layout string) []string {
	totalW, totalH, panes := parseTmuxLayout(layout)
	if len(panes) == 0 || totalW == 0 || totalH == 0 {
		return nil
	}
	const artW, artH = 13, 5

	// tmux座標 → アート座標のスケーリング
	// x=0 は外枠左端(0)、x=totalW は外枠右端(artW-1) に対応させる
	sx := func(x int) int {
		if x == 0 {
			return 0
		}
		return 1 + int(float64(x)/float64(totalW)*float64(artW-2))
	}
	sy := func(y int) int {
		if y == 0 {
			return 0
		}
		return 1 + int(float64(y)/float64(totalH)*float64(artH-2))
	}

	// (row, col) があるペインの「底辺」の水平線上かどうか
	onHoriz := func(row, col int) bool {
		for _, p := range panes {
			if p.y+p.h >= totalH {
				continue // 外枠は除外
			}
			if sy(p.y+p.h) != row {
				continue
			}
			if col >= sx(p.x) && col <= sx(p.x+p.w) {
				return true
			}
		}
		return false
	}

	// (row, col) があるペインの「右辺」の垂直線上かどうか
	onVert := func(row, col int) bool {
		for _, p := range panes {
			if p.x+p.w >= totalW {
				continue // 外枠は除外
			}
			if sx(p.x+p.w) != col {
				continue
			}
			if row >= sy(p.y) && row <= sy(p.y+p.h) {
				return true
			}
		}
		return false
	}

	hasH := func(row, col int) bool {
		return row == 0 || row == artH-1 || onHoriz(row, col)
	}
	hasV := func(row, col int) bool {
		return col == 0 || col == artW-1 || onVert(row, col)
	}

	lines := make([]string, artH)
	for row := 0; row < artH; row++ {
		var sb strings.Builder
		for col := 0; col < artW; col++ {
			h, v := hasH(row, col), hasV(row, col)
			switch {
			case !h && !v:
				sb.WriteRune(' ')
			case h && !v:
				sb.WriteRune('─')
			case !h && v:
				sb.WriteRune('│')
			default:
				// 接続方向を確認してジャンクション文字を決定
				r := col < artW-1 && hasH(row, col+1)
				l := col > 0 && hasH(row, col-1)
				d := row < artH-1 && hasV(row+1, col)
				u := row > 0 && hasV(row-1, col)
				sb.WriteRune(layoutJunctionChar(r, l, d, u))
			}
		}
		lines[row] = sb.String()
	}
	return lines
}

func layoutJunctionChar(r, l, d, u bool) rune {
	switch {
	case r && l && d && u: return '┼'
	case r && l && d:      return '┬'
	case r && l && u:      return '┴'
	case r && d && u:      return '├'
	case l && d && u:      return '┤'
	case r && d:           return '┌'
	case l && d:           return '┐'
	case r && u:           return '└'
	case l && u:           return '┘'
	case r || l:           return '─'
	default:               return '│'
	}
}

// getLayoutArt はレイアウト名またはtmuxレイアウト文字列からASCIIアート行を返す。
func getLayoutArt(layout string) []string {
	if lines, ok := layoutPreviewLines[layout]; ok {
		return lines
	}
	if layout != "" {
		if art := renderCustomLayoutPreview(layout); art != nil {
			return art
		}
	}
	return []string{
		"┌───────────┐",
		"│           │",
		"│           │",
		"└───────────┘",
	}
}

// layoutDisplayName はレイアウトの表示名を返す。
func layoutDisplayName(layout string) string {
	if isNamedLayout(layout) {
		return layout
	}
	if layout != "" {
		return "[カスタム]"
	}
	return "—"
}

func (m *EditorModel) renderWindowFormBox() string {
	f := m.winForm
	w := 58

	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("ウィンドウ編集") + "\n\n")

	// Name
	nameLabel := styleDim.Render("名前      ")
	if f.focus == 0 {
		nameLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("名前      ")
	}
	sb.WriteString(nameLabel + "  " + f.name.View() + "\n\n")

	// Layout selector
	layoutLabel := styleDim.Render("レイアウト ")
	if f.focus == 1 {
		layoutLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("レイアウト ")
	}
	currentLayout := windowLayouts[f.layoutIdx]
	var layoutVal string
	if f.focus == 1 {
		layoutVal = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("◀ " + currentLayout + " ▶")
		layoutVal += styleDim.Render("  h/←  l/→")
	} else {
		layoutVal = styleYellow.Render(currentLayout)
	}
	sb.WriteString(layoutLabel + "  " + layoutVal + "\n\n")

	// レイアウトプレビュー（常に表示、フォーカス時はハイライト）
	preview := renderLayoutPreview(currentLayout, f.focus == 1)
	// 左インデント
	for _, line := range strings.Split(preview, "\n") {
		sb.WriteString("           " + line + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(styleDim.Render("Esc キャンセル  Enter 保存"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(w).
		Render(sb.String())
}

func (m *EditorModel) renderEditorHelp() string {
	type entry struct{ key, desc string }
	type section struct {
		title   string
		entries []entry
	}
	sections := []section{
		{"ナビゲーション", []entry{
			{"j / k", "上 / 下移動"},
			{"h / l", "左 / 右パネル移動"},
			{"Tab", "パネル切替"},
			{"1 / 2", "パネル直接指定"},
		}},
		{"操作", []entry{
			{"a", "追加"},
			{"e", "編集"},
			{"d", "削除"},
			{"w / Ctrl+S", "保存"},
		}},
		{"フォーム内", []entry{
			{"Tab / Shift+Tab", "フィールド移動"},
			{"e", "ディレクトリ入力モード"},
			{"h / l", "レイアウト選択"},
			{"Space", "Execute トグル"},
			{"Enter", "確定"},
			{"Esc", "キャンセル"},
		}},
		{"その他", []entry{
			{"?", "ヘルプ"},
			{"Esc", "エディタを閉じる"},
		}},
	}

	const keyW = 16
	keyStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Width(keyW)
	descStyle := lipgloss.NewStyle().Foreground(colorFg)
	secTitleStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	bdrStyle := lipgloss.NewStyle().Foreground(colorBorder)
	dimStyle := lipgloss.NewStyle().Foreground(colorFgDim)

	colContentW := 36
	pad := 2
	colStyle := lipgloss.NewStyle().Width(colContentW+pad*2).Padding(1, pad)
	descMaxW := colContentW - keyW - 1
	descIndent := strings.Repeat(" ", keyW+1)

	wrapDesc := func(text string) string {
		if runewidth.StringWidth(text) <= descMaxW {
			return text
		}
		var lines []string
		var cur strings.Builder
		curW := 0
		for _, r := range text {
			rw := runewidth.RuneWidth(r)
			if curW+rw > descMaxW && cur.Len() > 0 {
				lines = append(lines, cur.String())
				cur.Reset()
				curW = 0
			}
			cur.WriteRune(r)
			curW += rw
		}
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
		}
		return strings.Join(lines, "\n"+descIndent)
	}

	renderSection := func(sec section) string {
		lines := []string{
			secTitleStyle.Render(sec.title),
			bdrStyle.Render(strings.Repeat("─", colContentW)),
		}
		for _, e := range sec.entries {
			lines = append(lines, keyStyle.Render(e.key)+" "+descStyle.Render(wrapDesc(e.desc)))
		}
		return strings.Join(lines, "\n")
	}

	leftBlock := colStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		renderSection(sections[0]), "", renderSection(sections[3]),
	))
	rightBlock := colStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		renderSection(sections[1]), "", renderSection(sections[2]),
	))

	colH := lipgloss.Height(leftBlock)
	if rh := lipgloss.Height(rightBlock); rh > colH {
		colH = rh
	}
	divLines := make([]string, colH)
	for i := range divLines {
		divLines[i] = bdrStyle.Render("│")
	}
	divider := strings.Join(divLines, "\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, divider, rightBlock)
	bodyW := lipgloss.Width(body)

	footer := lipgloss.NewStyle().Padding(0, pad).Render(
		dimStyle.Render("? / Esc / q で閉じる"),
	)
	sepLine := bdrStyle.Render(strings.Repeat("─", bodyW))
	inner := lipgloss.JoinVertical(lipgloss.Left, sepLine, body, sepLine, footer)

	innerH := lipgloss.Height(inner)
	innerW := lipgloss.Width(inner)
	return panelBorderColored(inner, innerW+2, innerH+2, 0, "Keybindings", colorYellow, colorYellow)
}

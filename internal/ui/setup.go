package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/masaway/lazyprj/internal/config"
)

// SetupModel はスキャンディレクトリ設定画面
type SetupModel struct {
	width, height int
	input         textinput.Model
	cfg           *config.Config
	canSkip       bool // 既存の値があればEscでスキップ可
	done          bool
	skipped       bool // Escで設定せずに閉じた
	status        string
	statusIsErr   bool
}

type setupSavedMsg struct{ err error }

func NewSetup(cfg *config.Config) *SetupModel {
	ti := textinput.New()
	ti.Placeholder = "例: ~/work  または  /home/user/projects"
	ti.CharLimit = 256
	ti.Width = 50

	canSkip := cfg.Settings.ScanDirectory != ""
	if canSkip {
		ti.SetValue(cfg.Settings.ScanDirectory)
	} else if cwd, err := os.Getwd(); err == nil {
		ti.SetValue(cwd)
	}
	ti.Focus()

	return &SetupModel{
		input:   ti,
		cfg:     cfg,
		canSkip: canSkip,
	}
}

func (m *SetupModel) Init() tea.Cmd { return textinput.Blink }

func (m *SetupModel) IsDone() bool    { return m.done }
func (m *SetupModel) IsSkipped() bool { return m.skipped }

func (m *SetupModel) Resize(w, h int) {
	m.width = w
	m.height = h
	if w > 20 {
		m.input.Width = w - 20
	}
}

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setupSavedMsg:
		if msg.err != nil {
			m.status = "保存エラー: " + msg.err.Error()
			m.statusIsErr = true
		} else {
			m.done = true
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				m.status = "ディレクトリを入力してください"
				m.statusIsErr = true
				return m, nil
			}
			m.cfg.Settings.ScanDirectory = val
			cfg := m.cfg
			return m, func() tea.Msg {
				return setupSavedMsg{err: config.Save(cfg)}
			}

		case "esc":
			m.done = true
			m.skipped = !m.canSkip // 既存値なしでスキップした場合はフラグを立てる
			return m, nil

		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *SetupModel) View() string {
	label := styleNormal.Render("プロジェクトをスキャンするディレクトリを入力してください。")
	note := styleDim.Render("チルダ（~/）が使えます。設定後にスキャン画面（n キー）で追加できます。")

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(0, 1).
		Width(m.input.Width + 4).
		Render(m.input.View())

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

	body := lipgloss.JoinVertical(lipgloss.Left,
		"",
		label,
		"",
		note,
		"",
		inputBox,
	)

	panelH := m.height - 3
	panel := panelBorder(body, m.width, panelH, 0, "スキャンディレクトリの設定", true)

	return lipgloss.JoinVertical(lipgloss.Left, panel, statusLine, m.renderKeys())
}

func (m *SetupModel) renderKeys() string {
	var hints []struct{ key, desc string }
	if m.canSkip {
		hints = []struct{ key, desc string }{{"Enter", "保存"}, {"Esc", "キャンセル"}, {"q", "終了"}}
	} else {
		hints = []struct{ key, desc string }{{"Enter", "保存"}, {"Esc", "後で設定"}, {"q", "終了"}}
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

package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/masaway/muxflow/internal/config"
	"github.com/masaway/muxflow/internal/tmux"
)

type focusPanel int

const (
	panelProjects focusPanel = iota
	panelDetail
)

type screen int

const (
	screenMain screen = iota
	screenEditor
	screenScan
	screenSetup
	screenQuickstart
)

// App はメインのbubbletea Model
type App struct {
	width  int
	height int

	cfg      *config.Config
	sessions map[string]bool

	cursor        int
	focus         focusPanel
	status        string
	statusIsErr   bool
	pendingAttach string
	initialLoad   bool

	sortActiveFirst   bool
	originalProjects []config.Project // ソート前の順序を保持

	detailScroll int
	listScroll   int

	// 確認ダイアログ
	confirmTarget string // 確認対象のセッション名（空 = ダイアログ非表示）

	// tmux同期確認ダイアログ
	pendingSyncCfg      *config.Config
	pendingSyncSessions map[string]bool
	pendingSyncNames    []string            // 変更されるプロジェクト名一覧
	pendingSyncChoices  []windowSyncChoice  // ウィンドウごとの選択状態
	pendingSyncCursor   int                 // ダイアログ内カーソル位置

	// サブ画面
	currentScreen screen
	editor        *EditorModel
	scanner       *ScanModel
	setup         *SetupModel
	quickstart    *QuickstartModel

	showHelp bool
}

// New はAppを生成する
func New() *App {
	return &App{
		sessions:    map[string]bool{},
		focus:       panelProjects,
		initialLoad: true,
	}
}

// PendingAttach は終了後にアタッチすべきセッション名を返す
func (m *App) PendingAttach() string {
	return m.pendingAttach
}

// applySortProjects はアクティブ優先でプロジェクトをソートする
func (m *App) applySortProjects() {
	if m.cfg == nil {
		return
	}
	currentName := ""
	if m.cursor < len(m.cfg.Projects) {
		currentName = m.cfg.Projects[m.cursor].Name
	}
	m.originalProjects = make([]config.Project, len(m.cfg.Projects))
	copy(m.originalProjects, m.cfg.Projects)
	sort.SliceStable(m.cfg.Projects, func(i, j int) bool {
		ai := m.sessions[m.cfg.Projects[i].Name]
		aj := m.sessions[m.cfg.Projects[j].Name]
		if ai != aj {
			return ai
		}
		if m.cfg.Projects[i].AutoStart != m.cfg.Projects[j].AutoStart {
			return m.cfg.Projects[i].AutoStart
		}
		return false
	})
	for i, p := range m.cfg.Projects {
		if p.Name == currentName {
			m.cursor = i
			break
		}
	}
}

// restoreProjectOrder は元の順序に戻す
func (m *App) restoreProjectOrder() {
	if m.cfg == nil || m.originalProjects == nil {
		return
	}
	currentName := ""
	if m.cursor < len(m.cfg.Projects) {
		currentName = m.cfg.Projects[m.cursor].Name
	}
	m.cfg.Projects = m.originalProjects
	m.originalProjects = nil
	for i, p := range m.cfg.Projects {
		if p.Name == currentName {
			m.cursor = i
			break
		}
	}
}

// ─── メッセージ ───────────────────────────────────────────────────────────────

type reloadedMsg struct {
	cfg      *config.Config
	sessions map[string]bool
}

type sessionStartedMsg struct {
	name    string
	created bool
	err     error
}

type sessionKilledMsg struct {
	name string
	err  error
}

type sessionRestartedMsg struct {
	name string
	err  error
}

type switchClientMsg struct {
	err error
}

type windowSyncChoice struct {
	projectName  string
	windowIdx    int
	windowName   string
	configLayout string
	tmuxLayout   string
	useTmux      bool // true=tmuxを保存, false=設定を優先
}

type syncPreviewMsg struct {
	cfg      *config.Config
	sessions map[string]bool
	changed  []string
	choices  []windowSyncChoice
}

// ─── コマンド ─────────────────────────────────────────────────────────────────

func reloadCmd() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return reloadedMsg{cfg: &config.Config{}, sessions: map[string]bool{}}
	}
	return reloadedMsg{cfg: cfg, sessions: tmux.ListSessions()}
}

func syncPreviewCmd(projectName string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return reloadedMsg{cfg: &config.Config{}, sessions: map[string]bool{}}
		}
		sessions := tmux.ListSessions()

		var changedNames []string
		var choices []windowSyncChoice
		for i := range cfg.Projects {
			p := &cfg.Projects[i]
			if p.Name != projectName || !sessions[p.Name] {
				continue
			}
			windows := tmux.InspectSession(p.Name)
			normalizedWins := normalizeWindows(p.Path, p.Windows)
			if windows == nil || !windowsDiffer(normalizedWins, windows) {
				continue
			}
			changedNames = append(changedNames, p.Name)
			for j, tmuxWin := range windows {
				if j < len(normalizedWins) && singleWindowDiffer(normalizedWins[j], tmuxWin) {
					choices = append(choices, windowSyncChoice{
						projectName:  p.Name,
						windowIdx:    j,
						windowName:   tmuxWin.Name,
						configLayout: p.Windows[j].Layout,
						tmuxLayout:   tmuxWin.Layout,
						useTmux:      true,
					})
				}
			}
			p.Windows = windows
			p.Commands = config.Commands{}
		}
		if len(changedNames) == 0 {
			return reloadedMsg{cfg: cfg, sessions: sessions}
		}
		return syncPreviewMsg{cfg: cfg, sessions: sessions, changed: changedNames, choices: choices}
	}
}

func windowsDiffer(a, b []config.Window) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if singleWindowDiffer(a[i], b[i]) {
			return true
		}
	}
	return false
}

func singleWindowDiffer(a, b config.Window) bool {
	if len(a.Panes) != len(b.Panes) {
		return true
	}
	for j := range a.Panes {
		if a.Panes[j].Dir != b.Panes[j].Dir {
			return true
		}
	}
	return false
}

// normalizeWindows はプロジェクトパスを基準にペインの Dir を絶対パスに正規化する。
// 比較用途のみで使い、設定の保存には使わない。
func normalizeWindows(projectPath string, windows []config.Window) []config.Window {
	base := config.ExpandPath(projectPath)
	result := make([]config.Window, len(windows))
	for i, w := range windows {
		panes := make([]config.Pane, len(w.Panes))
		for j, p := range w.Panes {
			dir := config.ExpandPath(p.Dir)
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(base, dir)
			}
			panes[j] = config.Pane{Dir: dir}
		}
		result[i] = config.Window{Name: w.Name, Layout: w.Layout, Panes: panes}
	}
	return result
}

func commitSyncCmd(cfg *config.Config, sessions map[string]bool) tea.Cmd {
	return func() tea.Msg {
		_ = config.Save(cfg)
		return reloadedMsg{cfg: cfg, sessions: sessions}
	}
}

func switchClientCmd(name string) tea.Cmd {
	return func() tea.Msg {
		return switchClientMsg{err: tmux.SwitchClient(name)}
	}
}

func startSessionCmd(p config.Project, kill bool) tea.Cmd {
	return func() tea.Msg {
		created, err := tmux.CreateSession(&p, kill)
		return sessionStartedMsg{name: p.Name, created: created, err: err}
	}
}

func killSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		err := tmux.KillSession(name)
		return sessionKilledMsg{name: name, err: err}
	}
}

func restartSessionCmd(p config.Project) tea.Cmd {
	return func() tea.Msg {
		if err := tmux.KillSession(p.Name); err != nil {
			return sessionRestartedMsg{name: p.Name, err: err}
		}
		_, err := tmux.CreateSession(&p, false)
		return sessionRestartedMsg{name: p.Name, err: err}
	}
}

// ─── Init ─────────────────────────────────────────────────────────────────────

func (m *App) Init() tea.Cmd {
	return reloadCmd
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// リサイズは常に処理
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		if m.editor != nil {
			m.editor.Resize(ws.Width, ws.Height)
		}
		if m.scanner != nil {
			m.scanner.Resize(ws.Width, ws.Height)
		}
		if m.quickstart != nil {
			m.quickstart.Resize(ws.Width, ws.Height)
		}
		return m, nil
	}

	// サブ画面に委譲
	switch m.currentScreen {
	case screenEditor:
		if m.editor != nil {
			result, cmd := m.editor.Update(msg)
			m.editor = result.(*EditorModel)
			if m.editor.IsDone() {
				m.currentScreen = screenMain
				m.editor = nil
				return m, reloadCmd
			}
			return m, cmd
		}

	case screenScan:
		if m.scanner != nil {
			result, cmd := m.scanner.Update(msg)
			m.scanner = result.(*ScanModel)
			if m.scanner.IsDone() {
				m.currentScreen = screenMain
				m.scanner = nil
				return m, reloadCmd
			}
			return m, cmd
		}

	case screenSetup:
		if m.setup != nil {
			result, cmd := m.setup.Update(msg)
			m.setup = result.(*SetupModel)
			if m.setup.IsDone() {
				skipped := m.setup.IsSkipped()
				m.currentScreen = screenMain
				m.setup = nil
				if skipped {
					// スキップ時はリロードしない（再度reloadedMsgが来ないのでsetup画面も出ない）
					// 次回起動時はScanDirectoryが空なので再度setup画面に遷移する
					return m, nil
				}
				return m, reloadCmd
			}
			return m, cmd
		}

	case screenQuickstart:
		if m.quickstart != nil {
			result, cmd := m.quickstart.Update(msg)
			m.quickstart = result.(*QuickstartModel)
			if m.quickstart.IsDone() {
				proj := m.quickstart.Result()
				m.currentScreen = screenMain
				m.quickstart = nil
				if proj != nil {
					m.status = fmt.Sprintf("起動中: %s ...", proj.Name)
					m.statusIsErr = false
					return m, startSessionCmd(*proj, false)
				}
			}
			return m, cmd
		}
	}

	// メイン画面のメッセージ処理
	switch msg := msg.(type) {
	case reloadedMsg:
		m.cfg = msg.cfg
		m.sessions = msg.sessions
		m.originalProjects = nil
		if m.cfg != nil && m.cursor >= len(m.cfg.Projects) {
			m.cursor = max(0, len(m.cfg.Projects)-1)
		}
		if m.sortActiveFirst {
			m.applySortProjects()
		}
		if m.status == "更新中..." {
			m.status = ""
			m.statusIsErr = false
		}
		// 初回起動時: スキャンディレクトリ未設定なら setup 画面へ
		if m.cfg != nil && m.cfg.Settings.ScanDirectory == "" && m.currentScreen == screenMain {
			m.setup = NewSetup(m.cfg)
			m.setup.Resize(m.width, m.height)
			m.currentScreen = screenSetup
			return m, m.setup.Init()
		}
		// 初回起動時: tmuxセッションがゼロならauto_startプロジェクトを一括起動
		if m.initialLoad {
			m.initialLoad = false
			if len(msg.sessions) == 0 && m.cfg != nil {
				var cmds []tea.Cmd
				for _, p := range m.cfg.Projects {
					if p.AutoStart {
						cmds = append(cmds, startSessionCmd(p, false))
					}
				}
				if len(cmds) > 0 {
					m.status = fmt.Sprintf("自動起動: %d 個のプロジェクトを起動中...", len(cmds))
					m.statusIsErr = false
					return m, tea.Batch(cmds...)
				}
			}
		}

	case syncPreviewMsg:
		m.pendingSyncCfg = msg.cfg
		m.pendingSyncSessions = msg.sessions
		m.pendingSyncNames = msg.changed
		m.pendingSyncChoices = msg.choices
		m.pendingSyncCursor = 0
		if m.status == "更新中..." {
			m.status = ""
		}

	case sessionStartedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("エラー: %s", msg.err)
			m.statusIsErr = true
		} else if msg.created {
			m.status = fmt.Sprintf("✓  '%s' を起動しました", msg.name)
			m.statusIsErr = false
			return m, reloadCmd
		} else {
			if tmux.IsInsideTmux() {
				return m, switchClientCmd(msg.name)
			}
			m.pendingAttach = msg.name
			return m, tea.Quit
		}

	case switchClientMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("エラー: %s", msg.err)
			m.statusIsErr = true
		}

	case sessionKilledMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("エラー: %s", msg.err)
			m.statusIsErr = true
		} else {
			m.status = fmt.Sprintf("✗  '%s' を停止しました", msg.name)
			m.statusIsErr = false
		}
		return m, reloadCmd

	case sessionRestartedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("エラー: %s", msg.err)
			m.statusIsErr = true
		} else {
			m.status = fmt.Sprintf("↺  '%s' を再起動しました", msg.name)
			m.statusIsErr = false
		}
		return m, reloadCmd

	case tea.KeyMsg:
		if m.pendingSyncCfg != nil {
			return m.handleSyncConfirmKey(msg)
		}
		// 確認ダイアログ中はダイアログのキーのみ処理
		if m.confirmTarget != "" {
			return m.handleConfirmKey(msg)
		}
		// ヘルプ表示中は閉じるキーのみ処理
		if m.showHelp {
			switch msg.String() {
			case "?", "q", "esc":
				m.showHelp = false
			}
			return m, nil
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.showHelp = true

	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		if m.focus == panelProjects {
			m.focus = panelDetail
		} else {
			m.focus = panelProjects
		}

	case "h":
		m.focus = panelProjects

	case "l":
		m.focus = panelDetail

	case "1":
		m.focus = panelProjects

	case "2":
		m.focus = panelDetail

	case "r":
		if m.cfg == nil || len(m.cfg.Projects) == 0 {
			return m, nil
		}
		m.status = "更新中..."
		m.statusIsErr = false
		return m, syncPreviewCmd(m.cfg.Projects[m.cursor].Name)

	case "j", "down":
		if m.focus == panelProjects {
			if m.cfg != nil && m.cursor < len(m.cfg.Projects)-1 {
				m.cursor++
				m.detailScroll = 0
				m.adjustListScroll()
			}
		} else {
			m.detailScroll++
		}

	case "k", "up":
		if m.focus == panelProjects {
			if m.cursor > 0 {
				m.cursor--
				m.detailScroll = 0
				m.adjustListScroll()
			}
		} else {
			if m.detailScroll > 0 {
				m.detailScroll--
			}
		}

	case "g":
		m.cursor = 0
		m.listScroll = 0
		m.detailScroll = 0

	case "G":
		if m.cfg != nil && len(m.cfg.Projects) > 0 {
			m.cursor = len(m.cfg.Projects) - 1
			m.detailScroll = 0
			m.adjustListScroll()
		}

	case "enter":
		if m.cfg == nil || len(m.cfg.Projects) == 0 {
			break
		}
		p := m.cfg.Projects[m.cursor]
		if m.sessions[p.Name] {
			if tmux.IsInsideTmux() {
				return m, switchClientCmd(p.Name)
			}
			m.pendingAttach = p.Name
			return m, tea.Quit
		}
		m.status = fmt.Sprintf("起動中: %s ...", p.Name)
		m.statusIsErr = false
		return m, startSessionCmd(p, false)

	case "x":
		if m.cfg == nil || len(m.cfg.Projects) == 0 {
			break
		}
		p := m.cfg.Projects[m.cursor]
		if !m.sessions[p.Name] {
			m.status = fmt.Sprintf("'%s' は起動していません", p.Name)
			m.statusIsErr = true
			break
		}
		m.status = fmt.Sprintf("停止中: %s ...", p.Name)
		return m, killSessionCmd(p.Name)

	case "e":
		if m.cfg != nil && len(m.cfg.Projects) > 0 {
			m.editor = NewEditor(m.cfg, m.cursor)
			m.editor.Resize(m.width, m.height)
			m.currentScreen = screenEditor
		}

	case "S":
		if m.cfg != nil {
			m.setup = NewSetup(m.cfg)
			m.setup.Resize(m.width, m.height)
			m.currentScreen = screenSetup
			return m, m.setup.Init()
		}

	case "R":
		if m.cfg == nil || len(m.cfg.Projects) == 0 {
			break
		}
		p := m.cfg.Projects[m.cursor]
		if !m.sessions[p.Name] {
			m.status = fmt.Sprintf("'%s' は起動していません", p.Name)
			m.statusIsErr = true
			break
		}
		m.confirmTarget = p.Name

	case "a":
		if m.cfg == nil || len(m.cfg.Projects) == 0 {
			break
		}
		m.cfg.Projects[m.cursor].AutoStart = !m.cfg.Projects[m.cursor].AutoStart
		if err := config.Save(m.cfg); err != nil {
			m.status = fmt.Sprintf("保存エラー: %v", err)
			m.statusIsErr = true
		} else {
			name := m.cfg.Projects[m.cursor].Name
			if m.cfg.Projects[m.cursor].AutoStart {
				m.status = fmt.Sprintf("★ '%s' を自動起動に設定しました", name)
			} else {
				m.status = fmt.Sprintf("'%s' の自動起動を解除しました", name)
			}
			m.statusIsErr = false
		}

	case "o":
		if m.cfg == nil {
			break
		}
		m.sortActiveFirst = !m.sortActiveFirst
		if m.sortActiveFirst {
			m.applySortProjects()
			m.status = "ソート: アクティブ優先"
		} else {
			m.restoreProjectOrder()
			m.status = "ソート: カスタム順"
		}
		m.statusIsErr = false
		m.listScroll = 0

	case "A":
		if m.cfg == nil {
			break
		}
		var cmds []tea.Cmd
		for _, p := range m.cfg.Projects {
			if p.AutoStart && !m.sessions[p.Name] {
				cmds = append(cmds, startSessionCmd(p, false))
			}
		}
		if len(cmds) == 0 {
			m.status = "自動起動プロジェクトはすでに全て起動中です"
			m.statusIsErr = false
		} else {
			m.status = fmt.Sprintf("%d 個のプロジェクトを起動中...", len(cmds))
			m.statusIsErr = false
			return m, tea.Batch(cmds...)
		}

	case "X":
		if m.cfg == nil || len(m.cfg.Projects) == 0 {
			break
		}
		p := m.cfg.Projects[m.cursor]
		m.cfg.Projects = append(m.cfg.Projects[:m.cursor], m.cfg.Projects[m.cursor+1:]...)
		m.cfg.HiddenProjects = append(m.cfg.HiddenProjects, p)
		m.cfg.SkippedPaths = append(m.cfg.SkippedPaths, p.Path)
		if m.cursor >= len(m.cfg.Projects) && m.cursor > 0 {
			m.cursor--
		}
		if err := config.Save(m.cfg); err != nil {
			m.status = fmt.Sprintf("保存エラー: %v", err)
			m.statusIsErr = true
		} else {
			m.status = fmt.Sprintf("'%s' をスキップ済みへ移動しました", p.Name)
			m.statusIsErr = false
		}
		return m, reloadCmd

	case "K":
		if m.cfg == nil || len(m.cfg.Projects) == 0 || m.cursor == 0 {
			break
		}
		if m.sortActiveFirst {
			m.restoreProjectOrder()
			m.sortActiveFirst = false
		}
		m.cfg.Projects[m.cursor], m.cfg.Projects[m.cursor-1] = m.cfg.Projects[m.cursor-1], m.cfg.Projects[m.cursor]
		m.cursor--
		m.adjustListScroll()
		if err := config.Save(m.cfg); err != nil {
			m.status = fmt.Sprintf("保存エラー: %v", err)
			m.statusIsErr = true
		} else {
			m.status = ""
			m.statusIsErr = false
		}

	case "J":
		if m.cfg == nil || len(m.cfg.Projects) == 0 || m.cursor >= len(m.cfg.Projects)-1 {
			break
		}
		if m.sortActiveFirst {
			m.restoreProjectOrder()
			m.sortActiveFirst = false
		}
		m.cfg.Projects[m.cursor], m.cfg.Projects[m.cursor+1] = m.cfg.Projects[m.cursor+1], m.cfg.Projects[m.cursor]
		m.cursor++
		m.adjustListScroll()
		if err := config.Save(m.cfg); err != nil {
			m.status = fmt.Sprintf("保存エラー: %v", err)
			m.statusIsErr = true
		} else {
			m.status = ""
			m.statusIsErr = false
		}

	case "s":
		m.scanner = NewScanner(m.cfg)
		m.scanner.Resize(m.width, m.height)
		m.currentScreen = screenScan
		return m, m.scanner.LoadCmd()

	case "n":
		if m.cfg != nil {
			m.quickstart = NewQuickstart(m.cfg)
			m.quickstart.Resize(m.width, m.height)
			m.currentScreen = screenQuickstart
			return m, m.quickstart.Init()
		}
	}

	return m, nil
}

func (m *App) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.confirmTarget
		m.confirmTarget = ""
		if m.cfg == nil {
			break
		}
		for _, p := range m.cfg.Projects {
			if p.Name == name {
				m.status = fmt.Sprintf("再起動中: %s ...", name)
				m.statusIsErr = false
				return m, restartSessionCmd(p)
			}
		}
	case "n", "N", "esc":
		m.confirmTarget = ""
	}
	return m, nil
}

func (m *App) handleSyncConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.pendingSyncChoices)
	switch msg.String() {
	case "j", "down":
		if n > 0 {
			m.pendingSyncCursor = (m.pendingSyncCursor + 1) % n
		}
	case "k", "up":
		if n > 0 {
			m.pendingSyncCursor = (m.pendingSyncCursor - 1 + n) % n
		}
	case "left", "h":
		if n > 0 {
			m.pendingSyncChoices[m.pendingSyncCursor].useTmux = false
		}
	case "right", "l":
		if n > 0 {
			m.pendingSyncChoices[m.pendingSyncCursor].useTmux = true
		}
	case " ":
		if n > 0 {
			m.pendingSyncChoices[m.pendingSyncCursor].useTmux = !m.pendingSyncChoices[m.pendingSyncCursor].useTmux
		}
	case "enter":
		// 選択に応じてconfigレイアウトを復元
		cfg := m.pendingSyncCfg
		for _, ch := range m.pendingSyncChoices {
			if !ch.useTmux {
				for i := range cfg.Projects {
					if cfg.Projects[i].Name == ch.projectName {
						if ch.windowIdx < len(cfg.Projects[i].Windows) {
							cfg.Projects[i].Windows[ch.windowIdx].Layout = ch.configLayout
						}
						break
					}
				}
			}
		}
		sessions := m.pendingSyncSessions
		m.clearPendingSync()
		return m, commitSyncCmd(cfg, sessions)
	case "esc":
		m.clearPendingSync()
		return m, reloadCmd
	}
	return m, nil
}

func (m *App) clearPendingSync() {
	m.pendingSyncCfg = nil
	m.pendingSyncSessions = nil
	m.pendingSyncNames = nil
	m.pendingSyncChoices = nil
	m.pendingSyncCursor = 0
}

func (m *App) adjustListScroll() {
	listH := m.visibleListItems()
	if m.cursor < m.listScroll {
		m.listScroll = m.cursor
	} else if m.cursor >= m.listScroll+listH {
		m.listScroll = m.cursor - listH + 1
	}
	if m.listScroll < 0 {
		m.listScroll = 0
	}
}

func (m *App) visibleListItems() int {
	panelH := m.height - 3
	contentH := panelH - 2
	items := contentH - 1
	if items < 1 {
		return 1
	}
	return items
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *App) View() string {
	if m.width == 0 || m.height == 0 {
		return "読み込み中..."
	}

	// サブ画面
	switch m.currentScreen {
	case screenEditor:
		if m.editor != nil {
			bg := m.renderMainScreen()
			return overlayCenter(bg, m.editor.View(), m.width, m.height)
		}
	case screenScan:
		if m.scanner != nil {
			bg := m.renderMainScreen()
			return overlayCenter(bg, m.scanner.View(), m.width, m.height)
		}
	case screenSetup:
		if m.setup != nil {
			return m.setup.View()
		}
	case screenQuickstart:
		if m.quickstart != nil {
			bg := m.renderMainScreen()
			return overlayCenter(bg, m.quickstart.View(), m.width, m.height)
		}
	}

	base := m.renderMainScreen()
	if m.showHelp {
		return m.renderHelpDialog(base)
	}
	if m.pendingSyncCfg != nil {
		return m.renderSyncConfirmDialog(base)
	}
	if m.confirmTarget != "" {
		return m.renderConfirmDialog(base)
	}
	return base
}

func (m *App) renderMainScreen() string {
	statusBar := m.renderStatusBar()
	keyHints := m.renderKeyHints()

	panelH := m.height - 3
	leftW := 38
	if m.width < 60 {
		leftW = m.width / 3
	}
	rightW := m.width - leftW

	left := m.renderProjectList(leftW, panelH)
	right := m.renderDetail(rightW, panelH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar, keyHints)
}

func (m *App) renderStatusBar() string {
	if m.statusIsErr {
		return styleStatusErr.Width(m.width).Render(m.status)
	}
	return styleStatusOk.Width(m.width).Render(m.status)
}

func (m *App) renderKeyHints() string {
	type hint struct{ key, desc string }
	hints := []hint{
		{"Enter", "起動/アタッチ"}, {"e", "編集"},
		{"a", "自動起動"}, {"n", "新規作成"}, {"s", "スキャン"}, {"?", "ヘルプ"},
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

func (m *App) renderHelpDialog(bg string) string {
	type entry struct{ key, desc string }
	type section struct {
		title   string
		entries []entry
	}
	sections := []section{
		{"ナビゲーション", []entry{
			{"j / k", "上 / 下移動"},
			{"h / l", "左 / 右パネル移動"},
			{"g / G", "先頭 / 末尾"},
			{"Tab", "フォーカス切替"},
			{"1 / 2", "フォーカス直接指定"},
		}},
		{"セッション", []entry{
			{"Enter", "起動 / アタッチ"},
			{"x", "停止"},
			{"R", "再起動（確認あり）"},
			{"r", "設定再読み込み・同期"},
		}},
		{"プロジェクト", []entry{
			{"e", "編集"},
			{"n", "新規セッション作成"},
			{"a", "自動起動トグル"},
			{"A", "自動起動を全て起動"},
			{"K / J", "順番を上 / 下に移動"},
			{"o", "ソート切替（アクティブ優先 / カスタム順）"},
			{"X", "スキップ済みへ移動"},
		}},
		{"スキャン", []entry{
			{"s", "スキャン実行"},
			{"S", "スキャン先ディレクトリ設定"},
		}},
		{"その他", []entry{
			{"?", "ヘルプ"},
			{"q / Ctrl+C", "終了"},
		}},
	}

	const keyW = 12
	keyStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Width(keyW)
	descStyle := lipgloss.NewStyle().Foreground(colorFg)
	secTitleStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	bdrStyle := lipgloss.NewStyle().Foreground(colorBorder)
	dimStyle := lipgloss.NewStyle().Foreground(colorFgDim)

	colContentW := 40
	pad := 2
	colStyle := lipgloss.NewStyle().Width(colContentW + pad*2).Padding(1, pad)
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
		renderSection(sections[0]), "", renderSection(sections[1]), "", renderSection(sections[4]),
	))
	rightBlock := colStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		renderSection(sections[2]), "", renderSection(sections[3]),
	))

	// 縦線をカラムの高さに合わせる
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
	dialog := panelBorderColored(inner, innerW+2, innerH+2, 0, "Keybindings", colorYellow, colorYellow)

	return overlayCenter(bg, dialog, m.width, m.height)
}

// overlayCenter は bg の上にダイアログを中央配置で重ねて返す。
// ダイアログが占める行は左右の余白も bg の内容を保持する。
func overlayCenter(bg, dialog string, termW, termH int) string {
	bgLines := strings.Split(bg, "\n")
	dlgLines := strings.Split(dialog, "\n")

	dlgH := len(dlgLines)
	dlgW := 0
	for _, l := range dlgLines {
		if w := lipgloss.Width(l); w > dlgW {
			dlgW = w
		}
	}

	startY := (termH - dlgH) / 2
	startX := (termW - dlgW) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	result := make([]string, termH)
	for i := range result {
		if i < len(bgLines) {
			result[i] = bgLines[i]
		} else {
			result[i] = strings.Repeat(" ", termW)
		}
	}

	for i, dlgLine := range dlgLines {
		y := startY + i
		if y < 0 || y >= termH {
			continue
		}
		bgLine := result[y]
		left := ansiTruncate(bgLine, startX)
		lw := lipgloss.Width(ansiStrip(left))
		if lw < startX {
			left += strings.Repeat(" ", startX-lw)
		}
		right := ansiSkip(bgLine, startX+dlgW)
		result[y] = left + dlgLine + right
	}

	return strings.Join(result, "\n")
}

// ansiTruncate は ANSI エスケープを保持しつつ表示幅 maxW 列で切り詰める。
func ansiTruncate(s string, maxW int) string {
	var buf strings.Builder
	w := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			buf.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runewidth.RuneWidth(r)
		if w+rw > maxW {
			break
		}
		buf.WriteRune(r)
		w += rw
		i += size
	}
	return buf.String()
}

// ansiSkip は s の先頭 skipW 表示列をスキップした残りを返す（ANSI 状態を引き継ぐ）。
func ansiSkip(s string, skipW int) string {
	var ansiState strings.Builder
	w := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			ansiState.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runewidth.RuneWidth(r)
		if w >= skipW {
			return ansiState.String() + s[i:]
		}
		w += rw
		i += size
	}
	return ""
}

// ansiStrip は ANSI エスケープを除去した文字列を返す。
func ansiStrip(s string) string {
	var buf strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		buf.WriteRune(r)
		i += size
	}
	return buf.String()
}

// panelBorderColored はボーダー色・タイトル色を指定できる panelBorder の汎用版。
func panelBorderColored(content string, totalW, totalH, idx int, title string, borderColor, titleColor lipgloss.Color) string {
	bStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)

	innerW := totalW - 2
	innerH := totalH - 2

	var idxStr string
	if idx > 0 {
		idxStr = bStyle.Render(fmt.Sprintf("─[%d]─", idx))
	} else {
		idxStr = bStyle.Render("─")
	}
	titleStr := titleStyle.Render(title)
	usedW := 1 + lipgloss.Width(idxStr) + lipgloss.Width(titleStr) + 1
	fillLen := totalW - usedW
	if fillLen < 0 {
		fillLen = 0
	}
	topBorder := bStyle.Render("╭") + idxStr + titleStr + bStyle.Render(strings.Repeat("─", fillLen)+"╮")

	lines := strings.Split(content, "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	lines = lines[:innerH]

	var rows []string
	for _, line := range lines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		rows = append(rows, bStyle.Render("│")+line+pad+bStyle.Render("│"))
	}

	bottomBorder := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")
	return topBorder + "\n" + strings.Join(rows, "\n") + "\n" + bottomBorder
}

// panelBorder はタイトルをボーダー上辺に埋め込んだパネルを描画する
// 例: ╭─[1]─Projects──────────────────────────╮
func panelBorder(content string, totalW, totalH, idx int, title string, focused bool) string {
	borderFg := colorBorder
	if focused {
		borderFg = colorBorderFocus
	}
	bStyle := lipgloss.NewStyle().Foreground(borderFg)
	titleStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)

	innerW := totalW - 2
	innerH := totalH - 2

	// 上辺: ╭─[N]─Title──────────╮ (idx=0 の場合は [N] を省略)
	var idxStr string
	if idx > 0 {
		idxStr = bStyle.Render(fmt.Sprintf("─[%d]─", idx))
	} else {
		idxStr = bStyle.Render("─")
	}
	titleStr := titleStyle.Render(title)
	// ╭(1) + idxStr + titleStr + fill + ╮(1) = totalW
	usedW := 1 + lipgloss.Width(idxStr) + lipgloss.Width(titleStr) + 1
	fillLen := totalW - usedW
	if fillLen < 0 {
		fillLen = 0
	}
	topBorder := bStyle.Render("╭") + idxStr + titleStr + bStyle.Render(strings.Repeat("─", fillLen)+"╮")

	// コンテンツ行
	lines := strings.Split(content, "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	lines = lines[:innerH]

	var rows []string
	for _, line := range lines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		rows = append(rows, bStyle.Render("│")+line+pad+bStyle.Render("│"))
	}

	bottomBorder := bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")
	return topBorder + "\n" + strings.Join(rows, "\n") + "\n" + bottomBorder
}

func (m *App) renderProjectList(totalW, totalH int) string {
	innerW := totalW - 2
	innerH := totalH - 2

	visibleH := innerH
	if visibleH < 1 {
		visibleH = 1
	}

	var lines []string
	if m.cfg != nil {
		end := m.listScroll + visibleH
		if end > len(m.cfg.Projects) {
			end = len(m.cfg.Projects)
		}
		for i := m.listScroll; i < end; i++ {
			p := m.cfg.Projects[i]
			running := m.sessions[p.Name]

			dot := styleDim.Render("○")
			if running {
				dot = styleGreen.Render("●")
			}
			star := " "
			if p.AutoStart {
				star = styleYellow.Render("★")
			}

			nameW := innerW - 5
			if nameW < 1 {
				nameW = 1
			}
			name := p.Name
			if len(name) > nameW {
				name = name[:nameW-1] + "…"
			}

			if i == m.cursor {
				dotRaw := "○"
				if running {
					dotRaw = "●"
				}
				starRaw := " "
				if p.AutoStart {
					starRaw = "★"
				}
				line := styleSelectedItem.Width(innerW).Render(
					fmt.Sprintf("%s%s %-*s ", starRaw, dotRaw, nameW, name),
				)
				lines = append(lines, line)
			} else {
				line := fmt.Sprintf("%s%s %-*s ", star, dot, nameW, styleNormal.Render(name))
				lines = append(lines, line)
			}
		}
	}

	for len(lines) < visibleH {
		lines = append(lines, "")
	}

	return panelBorder(strings.Join(lines, "\n"), totalW, totalH, 1, "Projects", m.focus == panelProjects)
}

func (m *App) renderDetail(totalW, totalH int) string {
	innerW := totalW - 2
	innerH := totalH - 2

	panelTitle := "Detail"
	var content string
	if m.cfg == nil || len(m.cfg.Projects) == 0 {
		content = styleDim.Render("プロジェクトがありません")
	} else if m.cursor < len(m.cfg.Projects) {
		p := m.cfg.Projects[m.cursor]
		panelTitle = p.Name
		content = m.buildDetailContent(&p, innerW)
	}

	allLines := strings.Split(content, "\n")
	maxScroll := len(allLines) - innerH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	end := m.detailScroll + innerH
	if end > len(allLines) {
		end = len(allLines)
	}
	visible := strings.Join(allLines[m.detailScroll:end], "\n")

	return panelBorder(visible, totalW, totalH, 2, panelTitle, m.focus == panelDetail)
}

func (m *App) buildDetailContent(p *config.Project, width int) string {
	var sb strings.Builder

	sb.WriteString(styleDim.Render(p.Path) + "\n")

	if m.sessions[p.Name] {
		sb.WriteString(styleGreen.Render("● 起動中") + "\n")
	} else {
		sb.WriteString(styleDim.Render("○ 停止中") + "\n")
	}
	if p.AutoStart {
		sb.WriteString(styleYellow.Render("★ 自動起動") + "\n")
	}
	if p.Description != "" {
		sb.WriteString("\n" + styleNormal.Render(p.Description) + "\n")
	}

	sb.WriteString("\n")
	sep := styleDim.Render(strings.Repeat("─", min(width, 40)))
	sb.WriteString(sep + "\n\n")

	windows := p.Windows
	if !p.HasWindows() {
		tmp := *p
		tmp.MigrateFromCommands()
		windows = tmp.Windows
	}
	for wIdx, window := range windows {
		layoutSuffix := ""
		if isNamedLayout(window.Layout) {
			layoutSuffix = "  " + styleDim.Render(window.Layout)
		} else if window.Layout != "" {
			layoutSuffix = "  " + styleDim.Render("[カスタム]")
		}
		sb.WriteString(fmt.Sprintf("%s  %s%s\n",
			styleCyan.Render(fmt.Sprintf("⬛ window %d", wIdx)),
			styleNormal.Render(window.Name),
			layoutSuffix,
		))
		for pIdx, pane := range window.Panes {
			execMark := styleDim.Render("  ▷")
			if pane.Execute {
				execMark = styleRed.Render("  ▶")
			}
			dir := pane.Dir
			if dir == "" {
				dir = "."
			}
			cmd := pane.Command
			if cmd == "" {
				cmd = "—"
			}
			sb.WriteString(fmt.Sprintf("%s  pane %d  %s\n", execMark, pIdx, styleDim.Render(dir)))
			sb.WriteString(fmt.Sprintf("       %s\n", styleYellow.Render(cmd)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *App) renderConfirmDialog(bg string) string {
	msg := fmt.Sprintf("'%s' を kill して再起動しますか？", m.confirmTarget)
	line1 := styleNormal.Render(msg)
	line2 := styleDim.Render("y: 実行  /  n, Esc: キャンセル")

	dialogW := lipgloss.Width(msg) + 6
	if dw := lipgloss.Width(line2) + 6; dw > dialogW {
		dialogW = dw
	}

	inner := lipgloss.NewStyle().Padding(1, 2).Width(dialogW).
		Render(lipgloss.JoinVertical(lipgloss.Center, line1, "", line2))
	innerW := lipgloss.Width(inner)
	innerH := lipgloss.Height(inner)
	dialog := panelBorderColored(inner, innerW+2, innerH+2, 0, "再起動確認", colorYellow, colorYellow)

	return overlayCenter(bg, dialog, m.width, m.height)
}

func (m *App) renderSyncConfirmDialog(bg string) string {
	var lines []string
	lines = append(lines, styleNormal.Render("レイアウトの変更が検出されました"))
	lines = append(lines, "")

	for _, ch := range m.pendingSyncChoices {
		// カーソル行
		idx := -1
		for i := range m.pendingSyncChoices {
			if m.pendingSyncChoices[i].projectName == ch.projectName &&
				m.pendingSyncChoices[i].windowIdx == ch.windowIdx {
				idx = i
				break
			}
		}
		focused := idx == m.pendingSyncCursor
		cursor := "  "
		if focused {
			cursor = styleCyan.Render("▸ ")
		}
		lines = append(lines, cursor+styleYellow.Render(ch.projectName)+styleDim.Render("  "+ch.windowName))
		lines = append(lines, "")

		cfgArt := getLayoutArt(ch.configLayout)
		tmuxArt := getLayoutArt(ch.tmuxLayout)
		maxH := max(len(cfgArt), len(tmuxArt))
		for len(cfgArt) < maxH {
			cfgArt = append(cfgArt, strings.Repeat(" ", 13))
		}
		for len(tmuxArt) < maxH {
			tmuxArt = append(tmuxArt, strings.Repeat(" ", 13))
		}

		// 選択状態に応じてASCIIアートの色を決定
		var cfgArtStyle, tmuxArtStyle lipgloss.Style
		var cfgCheck, tmuxCheck string
		if ch.useTmux {
			cfgArtStyle = lipgloss.NewStyle().Foreground(colorFgDim)
			tmuxArtStyle = lipgloss.NewStyle().Foreground(colorCyan)
			cfgCheck = "  "
			tmuxCheck = styleGreen.Render("✓ ")
		} else {
			cfgArtStyle = lipgloss.NewStyle().Foreground(colorCyan)
			tmuxArtStyle = lipgloss.NewStyle().Foreground(colorFgDim)
			cfgCheck = styleGreen.Render("✓ ")
			tmuxCheck = "  "
		}

		cfgLbl := layoutDisplayName(ch.configLayout)
		tmuxLbl := layoutDisplayName(ch.tmuxLayout)
		// ヘッダー
		lines = append(lines, fmt.Sprintf("  %s%-15s  %s%s",
			cfgCheck, styleDim.Render("設定"),
			tmuxCheck, styleDim.Render("現在のtmux")))
		// ASCIIアート行
		for j := 0; j < maxH; j++ {
			lines = append(lines, "  "+cfgArtStyle.Render(cfgArt[j])+"    "+tmuxArtStyle.Render(tmuxArt[j]))
		}
		// ラベル行
		lines = append(lines, fmt.Sprintf("  %-13s    %s",
			styleDim.Render(cfgLbl), styleDim.Render(tmuxLbl)))
		lines = append(lines, "")
	}

	lines = append(lines, styleDim.Render("←/→: 選択  Space: 切替  Enter: 保存  Esc: キャンセル"))

	dialogW := 50
	for _, l := range lines {
		if w := lipgloss.Width(l) + 6; w > dialogW {
			dialogW = w
		}
	}

	inner := lipgloss.NewStyle().Padding(1, 2).Width(dialogW).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	innerW := lipgloss.Width(inner)
	innerH := lipgloss.Height(inner)
	dialog := panelBorderColored(inner, innerW+2, innerH+2, 0, "レイアウト同期", colorYellow, colorYellow)

	return overlayCenter(bg, dialog, m.width, m.height)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

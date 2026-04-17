package ui

import "github.com/charmbracelet/lipgloss"

// Tokyo Night カラーパレット（lazygitに近いデザイン）
var (
	colorBg          = lipgloss.Color("#1a1b26")
	colorBg2         = lipgloss.Color("#24283b")
	colorBg3         = lipgloss.Color("#1f2335")
	colorBorder      = lipgloss.Color("#414868")
	colorBorderFocus = lipgloss.Color("#7aa2f7")
	colorFg          = lipgloss.Color("#c0caf5")
	colorFgDim       = lipgloss.Color("#565f89")
	colorGreen       = lipgloss.Color("#9ece6a")
	colorRed         = lipgloss.Color("#f7768e")
	colorYellow      = lipgloss.Color("#e0af68")
	colorCyan        = lipgloss.Color("#7dcfff")
	colorBlue        = lipgloss.Color("#7aa2f7")
	colorPurple      = lipgloss.Color("#bb9af7")
	colorSelected    = lipgloss.Color("#30477a") // 青がかった選択色
)

var (
	stylePanelNormal = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)

	stylePanelFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderFocus)

	stylePanelTitle = lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true)

	styleSelectedItem = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorBlue).
				Bold(true)

	styleDim = lipgloss.NewStyle().
			Foreground(colorFgDim)

	styleGreen = lipgloss.NewStyle().
			Foreground(colorGreen)

	styleRed = lipgloss.NewStyle().
			Foreground(colorRed)

	styleYellow = lipgloss.NewStyle().
			Foreground(colorYellow)

	styleCyan = lipgloss.NewStyle().
			Foreground(colorCyan)

	stylePurple = lipgloss.NewStyle().
			Foreground(colorPurple)

	styleTitleBar = lipgloss.NewStyle().
			Foreground(colorFg)

	styleStatusOk = lipgloss.NewStyle().
			Foreground(colorFgDim).
			Padding(0, 1)

	styleStatusErr = lipgloss.NewStyle().
			Foreground(colorRed).
			Padding(0, 1)

	styleKeyBar = lipgloss.NewStyle().
			Foreground(colorFgDim)

	styleKeyName = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	styleKeyDesc = lipgloss.NewStyle().
			Foreground(colorFg)

	styleKeySep = lipgloss.NewStyle().
			Foreground(colorFgDim)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorFg)

	styleGreenBold = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	styleDimBold = lipgloss.NewStyle().
			Foreground(colorFgDim).
			Bold(true)

	styleKeyBarSep = lipgloss.NewStyle().
			Foreground(colorBorder)

	styleKeyBarPad = lipgloss.NewStyle().
			Padding(0, 1)
)

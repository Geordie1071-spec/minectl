package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Brand and component colours (Lipgloss)
var (
	ColorGreen  = lipgloss.Color("#5DB85D")
	ColorIron   = lipgloss.Color("#8A9BAD")
	ColorBrown  = lipgloss.Color("#8B6914")
	ColorRed    = lipgloss.Color("#E05252")
	ColorDim    = lipgloss.Color("#666666")
	ColorBorder = lipgloss.Color("#333333")
	ColorWarn   = lipgloss.Color("#E0A030")
)

// Component styles
var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true).
			MarginBottom(1)

	ServerRunningStyle = lipgloss.NewStyle().
				Foreground(ColorGreen).
				Bold(true)

	ServerStoppedStyle = lipgloss.NewStyle().
				Foreground(ColorRed)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(ColorGreen)

	DimStyle = lipgloss.NewStyle().
		Foreground(ColorDim)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorGreen).
			SetString("✓")

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorRed).
			SetString("✗")

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorGreen)
)

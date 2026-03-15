package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func ColorLogLine(line string) string {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "[ERROR]") || strings.Contains(upper, "FATAL") {
		return lipgloss.NewStyle().Foreground(ColorRed).Render(line)
	}
	if strings.Contains(upper, "[WARN]") {
		return lipgloss.NewStyle().Foreground(ColorWarn).Render(line)
	}
	if strings.Contains(upper, "joined the game") || strings.Contains(upper, "left the game") {
		return lipgloss.NewStyle().Foreground(ColorGreen).Render(line)
	}
	return line
}

func FilterLogLines(lines []string, filter string) []string {
	if filter == "" {
		return lines
	}
	var out []string
	for _, l := range lines {
		if strings.Contains(l, filter) {
			out = append(out, l)
		}
	}
	return out
}

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

type ContainerStatsView struct {
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	MemoryPercent float64
}

func RenderStats(name, serverType, version, status string, uptime string, stats *ContainerStatsView) string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render(fmt.Sprintf("%s (%s %s)", name, serverType, version)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 50) + "\n")
	b.WriteString(fmt.Sprintf("  Status    %s", status))
	if uptime != "" {
		b.WriteString(fmt.Sprintf("  (uptime: %s)", uptime))
	}
	b.WriteString("\n")
	if stats != nil {
		memUsed := humanize.Bytes(stats.MemoryUsage)
		memLimit := humanize.Bytes(stats.MemoryLimit)
		b.WriteString(fmt.Sprintf("  Memory    %s / %s  %.0f%%\n", memUsed, memLimit, stats.MemoryPercent))
		b.WriteString(fmt.Sprintf("  CPU       %.1f%%\n", stats.CPUPercent))
	}
	return PanelStyle.Render(b.String())
}

func RenderBar(used, total float64, width int) string {
	if total <= 0 {
		return strings.Repeat("░", width)
	}
	pct := used / total
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	style := ProgressBarStyle
	if pct > 0.85 {
		style = lipgloss.NewStyle().Foreground(ColorRed)
	}
	return style.Render(bar)
}

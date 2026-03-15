package tui

import (
	"fmt"
	"strings"

	"github.com/minectl/minectl/internal/domain"
)

// RenderListTable renders a simple table of servers (non-interactive)
func RenderListTable(servers []domain.Server, all bool) string {
	if len(servers) == 0 {
		return DimStyle.Render("No servers found.")
	}
	var b strings.Builder
	header := fmt.Sprintf("%-12s %-8s %-8s %-6s %-8s %-10s\n", "NAME", "TYPE", "VERSION", "PORT", "STATUS", "MEMORY")
	b.WriteString(TitleStyle.Render("minectl — " + fmt.Sprintf("%d server(s)", len(servers))))
	b.WriteString("\n")
	b.WriteString(DimStyle.Render(header))
	b.WriteString(strings.Repeat("─", 60) + "\n")
	for _, s := range servers {
		if !all && s.Status == domain.StatusDeleted {
			continue
		}
		statusStr := s.Status
		if s.Status == domain.StatusRunning {
			statusStr = ServerRunningStyle.Render("running")
		} else if s.Status == domain.StatusStopped {
			statusStr = ServerStoppedStyle.Render("stopped")
		}
		mem := fmt.Sprintf("%dM", s.MemoryMB)
		row := fmt.Sprintf("%-12s %-8s %-8s %-6d %-8s %-10s\n", s.Name, s.MCType, s.MCVersion, s.Port, statusStr, mem)
		b.WriteString(row)
	}
	return b.String()
}

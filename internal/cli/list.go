package cli

import (
	"encoding/json"
	"fmt"

	"github.com/minectl/minectl/internal/domain"
	"github.com/spf13/cobra"
)

var listAll bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Minecraft servers",
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "show stopped and deleted servers")
}

func runList(cmd *cobra.Command, args []string) error {
	st := getStore()
	servers, err := st.ListServers()
	if err != nil {
		return err
	}
	if jsonOut {
		data, _ := json.MarshalIndent(servers, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if !quiet {
		fmt.Print(renderListTable(servers, listAll))
	}
	return nil
}

func renderListTable(servers []domain.Server, all bool) string {
	if len(servers) == 0 {
		return "No servers found.\n"
	}
	out := fmt.Sprintf("minectl — %d server(s)\n", len(servers))
	out += fmt.Sprintf("%-12s %-8s %-8s %-6s %-8s %-10s\n", "NAME", "TYPE", "VERSION", "PORT", "STATUS", "MEMORY")
	out += "------------------------------------------------------------\n"
	for _, s := range servers {
		if !all && s.Status == domain.StatusDeleted {
			continue
		}
		mem := fmt.Sprintf("%dM", s.MemoryMB)
		out += fmt.Sprintf("%-12s %-8s %-8s %-6d %-8s %-10s\n", s.Name, s.MCType, s.MCVersion, s.Port, s.Status, mem)
	}
	return out
}

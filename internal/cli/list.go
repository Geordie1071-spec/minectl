package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/expose"
	"github.com/Geordie1071-spec/minectl/internal/server"
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
	ctx := context.Background()
	if d, err := getDockerClient(ctx); err == nil {
		if synced, err := server.SyncAll(ctx, d, st); err == nil {
			servers = synced
		}
		_ = d.Close()
	}
	if jsonOut {
		data, _ := json.MarshalIndent(servers, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if !quiet {
		publicAddr := ""
		if cfg, err := st.GetConfig(); err == nil {
			publicAddr = cfg.PublicAddress
		}
		fmt.Print(renderListTable(servers, listAll, publicAddr))
	}
	return nil
}

func renderListTable(servers []domain.Server, all bool, publicAddr string) string {
	var shown []domain.Server
	running := 0
	for _, s := range servers {
		if !all && s.Status == domain.StatusDeleted {
			continue
		}
		shown = append(shown, s)
		if s.Status == domain.StatusRunning {
			running++
		}
	}
	if len(shown) == 0 {
		return "No servers yet. Create one: " + accent("minectl create -n survival -t paper") + "\n"
	}
	// What players type: the server's forward or tunnel, else the panel's
	// public address (a cloud machine only sees its private IP), else this
	// machine's address on the local network.
	host := publicAddr
	if host == "" {
		host = expose.LANIP()
	}
	rows := make([][]string, 0, len(shown))
	for _, s := range shown {
		join := fmt.Sprintf("localhost:%d", s.Port)
		if s.Public != nil {
			join = s.Public.Address
		} else if host != "" {
			join = fmt.Sprintf("%s:%d", host, s.Port)
		}
		rows = append(rows, []string{bold(s.Name), statusDot(s.Status), s.MCType + " " + s.MCVersion, formatMB(s.MemoryMB), join})
	}
	title := fmt.Sprintf("%d server%s · %d running", len(shown), plural(len(shown)), running)
	return bold(title) + "\n\n" + table([]string{"NAME", "STATUS", "TYPE", "MEMORY", "JOIN"}, rows)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// formatMB shows memory the way the panel does: 1024-based, "2 GB".
func formatMB(mb int) string {
	switch {
	case mb >= 1024 && mb%1024 == 0:
		return fmt.Sprintf("%d GB", mb/1024)
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

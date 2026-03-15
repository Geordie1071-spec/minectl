package cli

import (
	"encoding/json"
	"fmt"

	"github.com/minectl/minectl/internal/tui"
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
		fmt.Print(tui.RenderListTable(servers, listAll))
	}
	return nil
}

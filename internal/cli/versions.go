package cli

import (
	"fmt"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/versions"
	"github.com/spf13/cobra"
)

var versionsAll bool

var versionsCmd = &cobra.Command{
	Use:   "versions [type]",
	Short: "List Minecraft versions available for a server type",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runVersions,
}

func init() {
	versionsCmd.Flags().BoolVar(&versionsAll, "all", false, "show every version, not just the newest 15")
}

func runVersions(cmd *cobra.Command, args []string) error {
	mcType := "paper"
	if len(args) == 1 {
		mcType = strings.ToLower(args[0])
	}
	list, err := versions.NewClient().Supported(mcType)
	if err == versions.ErrUnchecked {
		return fmt.Errorf("can't list versions for %s; use vanilla, paper, fabric, quilt, forge, or neoforge", mcType)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(list)
	}
	if !versionsAll && len(list) > 15 {
		list = list[:15]
	}
	fmt.Printf("%s versions (newest first):\n  %s\n", mcType, strings.Join(list, "  "))
	return nil
}

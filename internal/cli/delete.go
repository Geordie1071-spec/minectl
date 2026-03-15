package cli

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/server"
	"github.com/minectl/minectl/internal/tui"
	"github.com/spf13/cobra"
)

var deletePurge bool

var deleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a server (container removed; world data kept unless --purge)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func init() {
	deleteCmd.Flags().BoolVar(&deletePurge, "purge", false, "also delete world files (irreversible)")
}

func runDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	if err := server.Delete(ctx, d, st, name, deletePurge); err != nil {
		return err
	}
	if !quiet {
		fmt.Println(tui.SuccessStyle.Render("Deleted"), name)
		if deletePurge {
			fmt.Println(tui.DimStyle.Render("World data removed."))
		}
	}
	return nil
}

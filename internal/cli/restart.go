package cli

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/server"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [name]",
	Short: "Restart a server",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestart,
}

func runRestart(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	pb := NewProgressBar("")
	defer pb.End()

	if err := server.Restart(ctx, d, st, name, pb.ServerProgress()); err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Restarted", name)
	}
	return nil
}

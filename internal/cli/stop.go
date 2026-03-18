package cli

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/server"
	"github.com/spf13/cobra"
)

var stopForce   bool
var stopTimeout int

var stopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop a running server",
	Args:  cobra.ExactArgs(1),
	RunE:  runStop,
}

func init() {
	stopCmd.Flags().BoolVar(&stopForce, "force", false, "skip rcon, stop container immediately")
	stopCmd.Flags().IntVar(&stopTimeout, "timeout", 30, "seconds to wait for clean shutdown")
}

func runStop(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	if err := server.Stop(ctx, d, st, name, stopForce, stopTimeout); err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Stopped", name)
	}
	return nil
}

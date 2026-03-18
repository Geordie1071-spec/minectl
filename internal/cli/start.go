package cli

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/server"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a stopped server",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	s, err := server.Start(ctx, d, st, name)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Started", s.Name)
	}
	return nil
}

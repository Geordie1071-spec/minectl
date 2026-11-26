package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/server"
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
	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()

	pb := NewProgressBar("")
	before := time.Now()
	s, err := server.Start(ctx, d, st, name, pb.ServerProgress())
	pb.End()
	if err != nil {
		return err
	}
	if !s.LastStartedAt.Before(before) && !noWait {
		if err := waitForReady(ctx, d, s, s.LastStartedAt); err != nil {
			if isStartFailed(err) {
				_ = server.Halt(context.Background(), d, st, s.Name)
				return fmt.Errorf("%w\n\nStopped %s so it doesn't keep restarting.", err, s.Name)
			}
			return err
		}
	}
	if !quiet {
		fmt.Println("Started", s.Name)
	}
	return nil
}

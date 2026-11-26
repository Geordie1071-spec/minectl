package cli

import (
	"context"
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/server"
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
	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	pb := NewProgressBar("")
	err = server.Restart(ctx, d, st, name, pb.ServerProgress())
	pb.End()
	if err != nil {
		return err
	}
	if !noWait {
		s, err := st.GetServer(name)
		if err != nil || s == nil {
			return fmt.Errorf("server not found: %s", name)
		}
		if err := waitForReady(ctx, d, s, s.LastStartedAt); err != nil {
			if isStartFailed(err) {
				_ = server.Halt(context.Background(), d, st, s.Name)
				return fmt.Errorf("%w\n\nStopped %s so it doesn't keep restarting.", err, s.Name)
			}
			return err
		}
	}
	if !quiet {
		fmt.Println("Restarted", name)
	}
	return nil
}

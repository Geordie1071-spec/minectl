package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopForce bool
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
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	err = m.Stop(ctx, args[0], stopForce, stopTimeout, ev)
	ev.finish()
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Stopped", args[0])
	}
	return nil
}

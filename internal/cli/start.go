package cli

import (
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a stopped server",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	s, err := m.Start(ctx, args[0], ev)
	ev.finish()
	if err != nil {
		return err
	}
	success("Started %s", s.Name)
	return nil
}

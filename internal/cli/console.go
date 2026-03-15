package cli

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/tui"
	"github.com/spf13/cobra"
)

var consoleCmd = &cobra.Command{
	Use:   "console [name]",
	Short: "Open interactive server console (TUI)",
	Args:  cobra.ExactArgs(1),
	RunE:  runConsole,
}

func runConsole(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if s.ContainerID == "" {
		return fmt.Errorf("server has no container")
	}
	logCh := make(chan string, 100)
	go func() {
		_ = d.StreamLogs(ctx, s.ContainerID, true, logCh)
		close(logCh)
	}()
	sendCmd := func(c string) {
		_, _ = d.ExecRcon(ctx, s.ContainerID, c)
	}
	return tui.RunConsole(ctx, name, logCh, sendCmd)
}

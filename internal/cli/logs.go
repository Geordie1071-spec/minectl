package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/minectl/minectl/internal/tui"
	"github.com/spf13/cobra"
)

var logsTail int
var logsFollow bool
var logsFilter string

var logsCmd = &cobra.Command{
	Use:   "logs [name]",
	Short: "Tail server logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().IntVar(&logsTail, "tail", 50, "number of lines to show")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "stream new lines")
	logsCmd.Flags().StringVar(&logsFilter, "filter", "", "filter lines containing string")
}

func runLogs(cmd *cobra.Command, args []string) error {
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
	lines, err := d.GetLogs(ctx, s.ContainerID, logsTail)
	if err != nil {
		return err
	}
	lines = tui.FilterLogLines(lines, logsFilter)
	for _, l := range lines {
		fmt.Println(tui.ColorLogLine(l))
	}
	if logsFollow {
		ch := make(chan string, 50)
		go func() {
			_ = d.StreamLogs(ctx, s.ContainerID, true, ch)
		}()
		for line := range ch {
			if logsFilter == "" || strings.Contains(line, logsFilter) {
				fmt.Println(tui.ColorLogLine(line))
			}
		}
	}
	return nil
}

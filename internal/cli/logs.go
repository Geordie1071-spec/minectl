package cli

import (
	"fmt"
	"os"
	"strings"

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
	ctx, cancel := signalContext()
	defer cancel()
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
		return fmt.Errorf("%s has no container yet; start it with: minectl start %s", name, name)
	}
	if !logsFollow {
		lines, err := d.GetLogs(ctx, s.ContainerID, logsTail)
		if err != nil {
			return err
		}
		for _, l := range filterLogLines(lines, logsFilter) {
			fmt.Println(l)
		}
		return nil
	}

	ch := make(chan string, 50)
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.FollowLogs(ctx, s.ContainerID, logsTail, ch)
		close(ch)
	}()
	for line := range ch {
		if logsFilter == "" || strings.Contains(line, logsFilter) {
			fmt.Println(line)
		}
	}
	if err := <-errCh; err != nil && ctx.Err() == nil {
		return err
	}
	if ctx.Err() == nil && !quiet {
		fmt.Fprintf(os.Stderr, "%s stopped; log stream ended.\n", name)
	}
	return nil
}

func filterLogLines(lines []string, contains string) []string {
	if contains == "" {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.Contains(l, contains) {
			out = append(out, l)
		}
	}
	return out
}

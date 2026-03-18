package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var statsWatch bool
var statsInterval int

var statsCmd = &cobra.Command{
	Use:   "stats [name]",
	Short: "Show server resource usage",
	Args:  cobra.ExactArgs(1),
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().BoolVarP(&statsWatch, "watch", "w", false, "keep refreshing")
	statsCmd.Flags().IntVar(&statsInterval, "interval", 2, "refresh interval in seconds")
}

func runStats(cmd *cobra.Command, args []string) error {
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
	for {
		stats, err := d.GetStats(ctx, s.ContainerID)
		if err != nil {
			return err
		}
		if !quiet {
			fmt.Printf("%s  CPU %.1f%%  Mem %s / %s (%.1f%%)\n",
				s.Name,
				stats.CPUPercent,
				humanize.Bytes(stats.MemoryUsage),
				humanize.Bytes(stats.MemoryLimit),
				stats.MemoryPercent,
			)
		} else {
			fmt.Fprintln(os.Stdout, stats.CPUPercent)
		}
		if !statsWatch {
			break
		}
		time.Sleep(time.Duration(statsInterval) * time.Second)
	}
	return nil
}

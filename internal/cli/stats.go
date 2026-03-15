package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/minectl/minectl/internal/tui"
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
		view := &tui.ContainerStatsView{
			CPUPercent:    stats.CPUPercent,
			MemoryUsage:   stats.MemoryUsage,
			MemoryLimit:   stats.MemoryLimit,
			MemoryPercent: stats.MemoryPercent,
		}
		fmt.Print(tui.RenderStats(s.Name, s.MCType, s.MCVersion, s.Status, "", view))
		if !quiet {
			fmt.Printf("  Memory    %s / %s\n", humanize.Bytes(stats.MemoryUsage), humanize.Bytes(stats.MemoryLimit))
		}
		if !statsWatch {
			break
		}
		time.Sleep(time.Duration(statsInterval) * time.Second)
	}
	return nil
}

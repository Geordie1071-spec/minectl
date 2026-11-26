package cli

import (
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/spf13/cobra"
)

var (
	resizeMemory  string
	resizeCPU     string
	resizePlayers int

	recommendType    string
	recommendPlayers int
	recommendMods    int
	recommendModpack string
)

var resizeCmd = &cobra.Command{
	Use:   "resize [name]",
	Short: "Change a server's memory or CPU limit",
	Args:  cobra.ExactArgs(1),
	RunE:  runResize,
}

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Recommend memory and CPU for a server",
	Long:  "Estimate memory and CPU from server type, players, and mods. Use --modpack to count a Modrinth modpack's mods automatically.",
	Args:  cobra.NoArgs,
	RunE:  runRecommend,
}

func init() {
	resizeCmd.Flags().StringVarP(&resizeMemory, "memory", "m", "", "new RAM (e.g. 6G), or auto to recommend")
	resizeCmd.Flags().StringVar(&resizeCPU, "cpu", "", "new CPU limit (e.g. 4), auto, or none")
	resizeCmd.Flags().IntVar(&resizePlayers, "players", 5, "expected players (used by auto)")

	recommendCmd.Flags().StringVarP(&recommendType, "type", "t", "paper", "server type")
	recommendCmd.Flags().IntVar(&recommendPlayers, "players", 5, "expected number of players at once")
	recommendCmd.Flags().IntVar(&recommendMods, "mods", 0, "number of mods")
	recommendCmd.Flags().StringVar(&recommendModpack, "modpack", "", "Modrinth modpack slug (counts its mods and sets the type)")
}

func runResize(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	err = m.Resize(ctx, args[0], resizeMemory, resizeCPU, resizePlayers, ev)
	ev.finish()
	return err
}

func runRecommend(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m := &service.Manager{St: getStore()}
	if d, err := getDockerClient(ctx); err == nil {
		m.D = d
		defer d.Close()
	}
	res, err := m.Recommend(ctx, service.RecommendRequest{Type: recommendType, Players: recommendPlayers, Mods: recommendMods, Modpack: recommendModpack})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(res.Recommendation)
	}
	if rel := res.Modpack; rel != nil {
		fmt.Printf("Modpack: %s %s (%s, Minecraft %s, %d server mods)\n", rel.Slug, rel.VersionNumber, rel.Loader, rel.MCVersion, res.Mods)
	}
	rec := res.Recommendation
	fmt.Printf("Memory: %s  (%s)\n", server.FormatMB(rec.MemoryMB), rec.Reason)
	fmt.Printf("CPU:    %g cores\n", rec.CPUCores)
	for _, w := range rec.Warnings {
		fmt.Println("Warning:", w)
	}
	return nil
}

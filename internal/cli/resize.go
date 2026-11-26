package cli

import (
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/server"
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
	name := args[0]
	if resizeMemory == "" && resizeCPU == "" {
		return fmt.Errorf("nothing to change; pass --memory and/or --cpu")
	}
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

	mods := len(s.Mods)
	if s.HasModrinthModpack() {
		mc := modrinth.NewClient()
		ver := ""
		if s.ModpackVersion != nil {
			ver = *s.ModpackVersion
		}
		if rel, err := mc.ResolveModpack(*s.ModpackID, s.MCType, "", ver); err == nil {
			if n, err := mc.CountModpackMods(rel.FileURL); err == nil {
				mods = n
			}
		}
	}
	memFlag, cpuFlag := resizeMemory, resizeCPU
	if memFlag == "" {
		memFlag = fmt.Sprintf("%dM", s.MemoryMB)
	}
	if cpuFlag == "" {
		cpuFlag = fmt.Sprintf("%g", s.CPUCores)
	}
	plan, err := planResources(ctx, d, st, s.MCType, resizePlayers, mods, memFlag, cpuFlag)
	if err != nil {
		return err
	}
	s.MemoryMB, s.CPUCores = plan.MemoryMB, plan.CPUCores
	if err := st.SaveServer(s); err != nil {
		return err
	}
	if !quiet {
		plan.print()
	}
	return applyServerChange(ctx, d, st, name)
}

func runRecommend(cmd *cobra.Command, args []string) error {
	mcType, mods := recommendType, recommendMods
	if recommendModpack != "" {
		mc := modrinth.NewClient()
		rel, err := mc.ResolveModpack(recommendModpack, "", "", "")
		if err != nil {
			return err
		}
		n, err := mc.CountModpackMods(rel.FileURL)
		if err != nil {
			return err
		}
		mcType, mods = rel.Loader, n
		fmt.Printf("Modpack: %s %s (%s, Minecraft %s, %d server mods)\n", rel.Slug, rel.VersionNumber, rel.Loader, rel.MCVersion, n)
	}
	var host docker.HostResources
	ctx, cancel := signalContext()
	defer cancel()
	if d, err := getDockerClient(ctx); err == nil {
		host, _ = d.HostResources(ctx)
		_ = d.Close()
	}
	rec := server.Recommend(mcType, recommendPlayers, mods, host)
	if jsonOut {
		return printJSON(rec)
	}
	fmt.Printf("Memory: %s  (%s)\n", server.FormatMB(rec.MemoryMB), rec.Reason)
	fmt.Printf("CPU:    %g cores\n", rec.CPUCores)
	for _, w := range rec.Warnings {
		fmt.Println("Warning:", w)
	}
	return nil
}

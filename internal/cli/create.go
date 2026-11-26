package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/spf13/cobra"
)

var (
	createName           string
	createType           string
	createVersion        string
	createMemory         string
	createCPU            string
	createPlayers        int
	createMods           int
	createPort           int
	createDataDir        string
	createNoStart        bool
	createModpack        string
	createModpackVersion string
	createCurseForge     string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and start a new Minecraft server",
	RunE:  runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createName, "name", "n", "", "server name (required)")
	createCmd.Flags().StringVarP(&createType, "type", "t", "paper", "server type: vanilla, paper, spigot, fabric, forge, neoforge, quilt")
	createCmd.Flags().StringVarP(&createVersion, "version", "v", "latest", "Minecraft version (e.g. 26.3, 1.21.1)")
	createCmd.Flags().StringVarP(&createMemory, "memory", "m", "auto", "RAM for the server (e.g. 4G, 512M), or auto to recommend from players and mods")
	createCmd.Flags().StringVar(&createCPU, "cpu", "auto", "CPU core limit (e.g. 2, 1.5), auto, or none for no limit")
	createCmd.Flags().IntVar(&createPlayers, "players", 5, "expected number of players at once (used by auto memory)")
	createCmd.Flags().IntVar(&createMods, "mods", 0, "expected number of mods you will add (used by auto memory; modpacks are counted automatically)")
	createCmd.Flags().IntVarP(&createPort, "port", "p", config.MinecraftPort, "host port to bind")
	createCmd.Flags().StringVar(&createDataDir, "data-dir", "", "directory for world data")
	createCmd.Flags().BoolVar(&createNoStart, "no-start", false, "create config only, do not start container")
	createCmd.Flags().StringVar(&createModpack, "modpack", "", "Modrinth modpack slug (e.g. into-the-backrooms-found-footage)")
	createCmd.Flags().StringVar(&createModpackVersion, "modpack-version", "", "modpack version ID or number (Modrinth), or file ID (CurseForge); omit for latest")
	createCmd.Flags().StringVar(&createCurseForge, "curseforge", "", "CurseForge modpack slug or page URL")
	createCmd.MarkFlagsMutuallyExclusive("modpack", "curseforge")
	_ = createCmd.MarkFlagRequired("name")
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	req := service.CreateRequest{
		Name:           createName,
		Type:           createType,
		TypeExplicit:   cmd.Flags().Changed("type"),
		Version:        createVersion,
		Memory:         createMemory,
		CPU:            createCPU,
		Players:        createPlayers,
		Mods:           createMods,
		ModsExplicit:   cmd.Flags().Changed("mods"),
		DataDir:        createDataDir,
		NoStart:        createNoStart,
		Modpack:        createModpack,
		ModpackVersion: createModpackVersion,
		CurseForge:     createCurseForge,
	}
	if cmd.Flags().Changed("port") {
		req.Port = createPort
	}
	ev := newEvents("")
	s, err := m.Create(ctx, req, ev)
	ev.finish()
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(s)
	}
	if !quiet {
		fmt.Println("Server created:", s.Name)
		fmt.Println("  Type:", s.MCType, "| Version:", s.MCVersion, "| Port:", s.Port, "| Memory:", s.MemoryMB, "MB")
		if s.ModpackID != nil && *s.ModpackID != "" {
			fmt.Println("  Modpack:", *s.ModpackID)
		}
		if s.Status == domain.StatusRunning {
			fmt.Println("  Connect at localhost:" + strconv.Itoa(s.Port) + " (use your machine's IP for other players)")
		}
	}
	return nil
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

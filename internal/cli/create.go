package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/store"
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

func parseMemory(s string) (int, error) {
	in := s
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return config.DefaultMemoryMB, nil
	}
	mult := 0
	if strings.HasSuffix(s, "G") {
		mult = 1024
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "M") {
		mult = 1
		s = strings.TrimSuffix(s, "M")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid memory: %s (use e.g. 4G or 4096M)", in)
	}
	if mult == 0 {
		mult = 1
		if n <= 64 {
			mult = 1024
		}
	}
	return n * mult, nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	switch createType {
	case domain.TypeVanilla, domain.TypePaper, domain.TypeSpigot, domain.TypeFabric, domain.TypeForge, domain.TypeNeoForge, domain.TypeQuilt:
	default:
		return fmt.Errorf("invalid type %q; use: vanilla, paper, spigot, fabric, forge, neoforge, quilt", createType)
	}
	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()

	version := createVersion
	modpackVersion := createModpackVersion
	mods := createMods
	if createModpack != "" {
		loader := ""
		if cmd.Flags().Changed("type") {
			if modrinth.NormalizeLoader(createType) == "" {
				return fmt.Errorf("modpack requires a mod loader (use --type fabric, forge, neoforge, or quilt), not %q", createType)
			}
			loader = createType
		}
		mc := modrinth.NewClient()
		rel, err := mc.ResolveModpack(createModpack, loader, createVersion, createModpackVersion)
		if err != nil {
			return err
		}
		createType, version, modpackVersion = rel.Loader, rel.MCVersion, rel.VersionID
		if n, err := mc.CountModpackMods(rel.FileURL); err == nil {
			mods = n
		} else if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: could not count mods in modpack: %v\n", err)
		}
		if !quiet && !jsonOut {
			fmt.Printf("Modpack: %s %s (%s, Minecraft %s, %d server mods)\n", rel.Slug, rel.VersionNumber, rel.Loader, rel.MCVersion, mods)
		}
	}

	port := createPort
	if !cmd.Flags().Changed("port") {
		port = nextFreePort(st, createPort)
	}

	modpackID, modpackSource, dataDir := createModpack, "", createDataDir
	if createCurseForge != "" {
		dataDir, err = server.DataDirFor(st, createName, createDataDir)
		if err != nil {
			return err
		}
		pack, err := installCurseForge(ctx, d, st, dataDir)
		if err != nil {
			return err
		}
		modpackID, modpackSource = createCurseForge, "curseforge"
		createType, version, modpackVersion = pack.MCType, pack.MCVersion, createModpackVersion
		if !cmd.Flags().Changed("mods") {
			mods = pack.Mods
		}
		if !quiet && !jsonOut {
			fmt.Printf("Modpack: %s %s (%s, Minecraft %s, %d mods)\n", pack.Name, pack.PackVersion, pack.MCType, pack.MCVersion, pack.Mods)
		}
	}

	plan, err := planResources(ctx, d, st, createType, createPlayers, mods, createMemory, createCPU)
	if err != nil {
		return err
	}
	if !quiet && !jsonOut {
		plan.print()
	}

	opts := server.CreateOptions{
		Name:           createName,
		MCType:         createType,
		Version:        version,
		MemoryMB:       plan.MemoryMB,
		CPUCores:       plan.CPUCores,
		Port:           port,
		DataDir:        dataDir,
		NoStart:        createNoStart,
		ModpackID:      modpackID,
		ModpackSource:  modpackSource,
		ModpackVersion: modpackVersion,
	}

	var pb *ProgressBar
	if !quiet && !jsonOut {
		pb = NewProgressBar("")
		opts.OnProgress = pb.ServerProgress()
	}
	s, err := server.Create(ctx, d, st, opts)
	if pb != nil {
		pb.End()
	}
	if err != nil {
		return err
	}
	if s.Status == domain.StatusRunning && !noWait {
		if err := waitForReady(ctx, d, s, s.LastStartedAt); err != nil {
			if isStartFailed(err) {
				_ = server.AbortCreate(context.Background(), d, st, s.Name)
				return fmt.Errorf("%w\n\nRemoved the failed server %q so it doesn't keep restarting (its folder %s was kept). Fix the problem and run create again.", err, s.Name, s.DataDir)
			}
			return err
		}
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
			portStr := strconv.Itoa(s.Port)
			fmt.Println("  Connect at localhost:" + portStr + " (use your machine's IP for other players)")
		}
	}
	return nil
}

func installCurseForge(ctx context.Context, d *docker.Client, st *store.Store, dataDir string) (*server.CurseForgePack, error) {
	var pb *ProgressBar
	var sp *spinner
	onPercent := func(int) {}
	var onLine func(string)
	if !quiet && !jsonOut {
		pb = NewProgressBar("")
		onPercent = func(p int) { pb.Callback(p, "downloading Java image") }
	}
	start := func() {
		if pb != nil && sp == nil {
			pb.End()
			sp = startSpinner("Installing modpack")
		}
	}
	if pb != nil {
		onLine = func(line string) {
			start()
			sp.setDetail(logPrefix.ReplaceAllString(strings.TrimSpace(line), ""))
		}
	}
	pack, err := server.InstallCurseForgePack(ctx, d, st, createName, dataDir, createCurseForge, createModpackVersion, onPercent, onLine)
	if sp != nil {
		took := sp.end()
		if err == nil {
			fmt.Printf("Modpack installed in %s\n", took)
		}
	} else if pb != nil {
		pb.End()
	}
	var setupErr *server.SetupError
	if !errors.As(err, &setupErr) {
		return pack, err
	}
	msg := "the modpack failed to install"
	if hint := server.DescribeStartFailure(setupErr.Logs); hint != "" {
		msg += ": " + hint
	}
	if summary := server.FailureSummary(setupErr.Logs); len(summary) > 0 {
		msg += "\n\nKey log lines:\n  " + strings.Join(summary, "\n  ")
	}
	if server.NeedsManualDownloads(setupErr.Logs) {
		msg += fmt.Sprintf("\n\nSave those files into %s, then run the same create command again.", filepath.Join(dataDir, server.CurseForgeDownloadsDir))
	}
	return nil, errors.New(msg)
}

func nextFreePort(st *store.Store, start int) int {
	servers, _ := st.ListServers()
	used := map[int]bool{}
	for _, s := range servers {
		used[s.Port] = true
	}
	for p := start; ; p++ {
		if !used[p] {
			return p
		}
	}
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

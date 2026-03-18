package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/minectl/minectl/internal/config"
	"github.com/minectl/minectl/internal/domain"
	"github.com/minectl/minectl/internal/modrinth"
	"github.com/minectl/minectl/internal/server"
	"github.com/spf13/cobra"
)

var (
	createName         string
	createType         string
	createVersion      string
	createMemory       string
	createCPU          float64
	createPort         int
	createDataDir      string
	createNoStart      bool
	createModpack      string
	createModpackVersion string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and start a new Minecraft server",
	RunE:  runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createName, "name", "n", "", "server name (required)")
	createCmd.Flags().StringVarP(&createType, "type", "t", "paper", "server type: vanilla, paper, spigot, fabric, forge, neoforge, quilt")
	createCmd.Flags().StringVarP(&createVersion, "version", "v", "latest", "Minecraft version (e.g. 1.21.1)")
	createCmd.Flags().StringVarP(&createMemory, "memory", "m", "2G", "RAM allocation (e.g. 2G, 4G, 512M)")
	createCmd.Flags().Float64Var(&createCPU, "cpu", 1.0, "CPU core limit")
	createCmd.Flags().IntVarP(&createPort, "port", "p", config.MinecraftPort, "host port to bind")
	createCmd.Flags().StringVar(&createDataDir, "data-dir", "", "directory for world data")
	createCmd.Flags().BoolVar(&createNoStart, "no-start", false, "create config only, do not start container")
	createCmd.Flags().StringVar(&createModpack, "modpack", "", "Modrinth modpack slug (e.g. all-of-fabric-6)")
	createCmd.Flags().StringVar(&createModpackVersion, "modpack-version", "", "modpack version ID (omit for latest)")
	_ = createCmd.MarkFlagRequired("name")
}

func parseMemory(s string) (int, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return config.DefaultMemoryMB, nil
	}
	var mult int = 1024
	if strings.HasSuffix(s, "G") {
		mult = 1024
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "M") {
		mult = 1
		s = strings.TrimSuffix(s, "M")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid memory: %s", createMemory)
	}
	return n * mult, nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	memMB, err := parseMemory(createMemory)
	if err != nil {
		return err
	}
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()

	version := createVersion
	modpackVersion := createModpackVersion
	if createModpack != "" {
		mc := modrinth.NewClient()

		typeFlagChanged := cmd.Flags().Changed("type")
		loader := ""
		if typeFlagChanged {
			loader = modrinth.NormalizeLoader(createType)
			if loader == "" {
				return fmt.Errorf("modpack requires a mod loader (use --type fabric, forge, neoforge, or quilt), not %q", createType)
			}
		} else {
			inferredType, err := inferModpackServerType(mc, createModpack, createVersion)
			if err != nil {
				return fmt.Errorf("infer mod loader for modpack %q: %w", createModpack, err)
			}
			createType = inferredType
			loader = modrinth.NormalizeLoader(createType)
			if loader == "" {
				return fmt.Errorf("inferred mod loader is invalid: %q", createType)
			}
			if !quiet {
				fmt.Println("Inferred mod loader from modpack:", createType)
			}
		}

		if createVersion == "" || strings.ToLower(createVersion) == "latest" {
			info, err := mc.GetModpackRecommendedVersion(createModpack, loader)
			if err != nil {
				return fmt.Errorf("modpack version: %w", err)
			}
			version = info.MCVersion
			modpackVersion = info.ModpackVersionID
			if !quiet {
				fmt.Println("Using modpack-compatible Minecraft version:", version)
			}
		} else {
			ok, err := mc.ModpackSupportsVersion(createModpack, createVersion, loader)
			if err != nil && !quiet {
				fmt.Fprintf(os.Stderr, "Warning: could not check modpack compatibility: %v\n", err)
			} else if !ok && !quiet {
				fmt.Fprintf(os.Stderr, "Warning: Minecraft version %s may not be compatible with modpack %q. Omit --version to use the modpack's recommended version.\n", createVersion, createModpack)
			}
		}
	}

	opts := server.CreateOptions{
		Name:           createName,
		MCType:         createType,
		Version:        version,
		MemoryMB:       memMB,
		CPUCores:       createCPU,
		Port:           createPort,
		DataDir:        createDataDir,
		NoStart:        createNoStart,
		ModpackID:      createModpack,
		ModpackVersion: modpackVersion,
	}

	switch createType {
	case domain.TypeVanilla, domain.TypePaper, domain.TypeSpigot, domain.TypeFabric, domain.TypeForge, domain.TypeNeoForge, domain.TypeQuilt:
	default:
		return fmt.Errorf("invalid type %q; use: vanilla, paper, spigot, fabric, forge, neoforge, quilt", createType)
	}

	var s *domain.Server
	s, err = server.Create(ctx, d, st, opts)
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
			portStr := strconv.Itoa(s.Port)
			fmt.Println("  Connect at localhost:" + portStr + " (use your machine's IP for other players)")
		}
	}
	return nil
}

func mapModrinthLoaderToServerType(loader string) string {
	switch strings.ToLower(loader) {
	case "fabric":
		return domain.TypeFabric
	case "forge":
		return domain.TypeForge
	case "neoforge":
		return domain.TypeNeoForge
	case "quilt":
		return domain.TypeQuilt
	default:
		return ""
	}
}

func inferModpackServerType(mc *modrinth.Client, modpackSlug, requestedVersion string) (string, error) {
	versions, err := mc.GetModpackVersions(modpackSlug, "")
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found")
	}

	var chosen modrinth.ModpackVersion
	if requestedVersion == "" || strings.ToLower(requestedVersion) == "latest" {
		chosen = versions[0]
	} else {
		found := false
		for _, v := range versions {
			for _, gv := range v.GameVersions {
				if gv == requestedVersion {
					chosen = v
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return "", fmt.Errorf("no modpack version found for Minecraft %q", requestedVersion)
		}
	}

	for _, l := range chosen.Loaders {
		if serverType := mapModrinthLoaderToServerType(l); serverType != "" {
			return serverType, nil
		}
	}

	return "", fmt.Errorf("modpack version has no supported loaders")
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

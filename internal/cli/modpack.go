package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/modrinth"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/store"
	"github.com/Geordie1071-spec/minectl/internal/versions"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var (
	modpackVersion string
	modpackForce   bool
)

var modpackCmd = &cobra.Command{
	Use:   "modpack",
	Short: "Set or show modpack",
}

var modpackSetCmd = &cobra.Command{
	Use:   "set [name] [pack-slug]",
	Short: "Install a modpack from Modrinth",
	Args:  cobra.ExactArgs(2),
	RunE:  runModpackSet,
}

var modpackInfoCmd = &cobra.Command{
	Use:   "info [name]",
	Short: "Show current modpack",
	Args:  cobra.ExactArgs(1),
	RunE:  runModpackInfo,
}

var modpackSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search Modrinth for modpacks",
	Long:  "Search for Modrinth modpacks by name. Use the returned slug with: minectl create --modpack <slug> ...",
	Args:  cobra.ExactArgs(1),
	RunE:  runModpackSearch,
}

func init() {
	modpackSetCmd.Flags().StringVarP(&modpackVersion, "version", "v", "", "modpack version ID or number (omit for latest)")
	modpackSetCmd.Flags().BoolVar(&modpackForce, "force", false, "allow switching to an older Minecraft version (may break the world)")
	modpackCmd.AddCommand(modpackSetCmd, modpackInfoCmd, modpackSearchCmd)
}

func runModpackSet(cmd *cobra.Command, args []string) error {
	name, packSlug := args[0], args[1]
	st := getStore()
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}

	mc := modrinth.NewClient()
	rel, err := mc.ResolveModpack(packSlug, "", "", modpackVersion)
	if err != nil {
		return err
	}
	resolved, err := versions.NewClient().Resolve(rel.Loader, rel.MCVersion)
	if err != nil {
		return err
	}
	_, statErr := os.Stat(filepath.Join(s.DataDir, "world"))
	hasWorld := statErr == nil
	if hasWorld && versions.Compare(resolved.Version, s.MCVersion) < 0 && !modpackForce {
		return fmt.Errorf("%s's world is on Minecraft %s but %s needs %s; Minecraft can't downgrade worlds safely.\nCreate a new server for the modpack instead, or pass --force to replace it anyway (a backup is made first)", name, s.MCVersion, packSlug, resolved.Version)
	}

	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()

	if hasWorld {
		b, err := server.CreateBackup(ctx, d, st, name)
		if err != nil {
			return fmt.Errorf("backup before switching to modpack: %w", err)
		}
		if !quiet {
			fmt.Println("Backed up current world:", b.ID)
		}
		s, err = st.GetServer(name)
		if err != nil || s == nil {
			return fmt.Errorf("server not found: %s", name)
		}
	}

	source := "modrinth"
	s.ModpackSource = &source
	s.ModpackID = &packSlug
	s.ModpackVersion = &rel.VersionID
	s.MCType, s.ModLoader = rel.Loader, rel.Loader
	s.MCVersion, s.ImageTag = resolved.Version, resolved.ImageTag
	if err := st.SaveServer(s); err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Modpack: %s %s (%s, Minecraft %s)\n", packSlug, rel.VersionNumber, rel.Loader, resolved.Version)
		if len(s.Mods) > 0 {
			fmt.Fprintln(os.Stderr, "Note: individually added mods are ignored while a modpack is set.")
		}
		if n, err := mc.CountModpackMods(rel.FileURL); err == nil {
			host, _ := d.HostResources(ctx)
			for _, w := range server.CheckMemoryChoice(s.MemoryMB, server.Recommend(rel.Loader, 5, n, host), host, 0) {
				fmt.Fprintf(os.Stderr, "Warning: %s. Change it with: minectl resize %s --memory auto\n", w, name)
			}
		}
	}
	return applyServerChange(ctx, d, st, name)
}

func applyServerChange(ctx context.Context, d *docker.Client, st *store.Store, name string) error {
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if s.ContainerID == "" {
		if !quiet {
			fmt.Println("Saved. It applies the next time you run: minectl start", name)
		}
		return nil
	}
	if !quiet {
		fmt.Println("Recreating container to apply changes...")
	}
	started, err := server.RecreateContainer(ctx, d, st, name)
	if err != nil {
		return err
	}
	if !started {
		if !quiet {
			fmt.Println("Saved. It applies the next time you run: minectl start", name)
		}
		return nil
	}
	s, err = st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	if noWait {
		return nil
	}
	if err := waitForReady(ctx, d, s, s.LastStartedAt); err != nil {
		if isStartFailed(err) {
			_ = server.Halt(context.Background(), d, st, name)
			return fmt.Errorf("%w\n\nStopped %s so it doesn't keep restarting.", err, name)
		}
		return err
	}
	return nil
}

func runModpackInfo(cmd *cobra.Command, args []string) error {
	st := getStore()
	s, err := st.GetServer(args[0])
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", args[0])
	}
	if s.ModpackID == nil {
		fmt.Println("No modpack")
		return nil
	}
	fmt.Printf("Modpack: %s (version: %s)\n", *s.ModpackID, func() string {
		if s.ModpackVersion != nil {
			return *s.ModpackVersion
		}
		return "latest"
	}())
	return nil
}

func runModpackSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	client := modrinth.NewClient()
	hits, err := client.SearchModpacks(query)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Println("No modpacks found. Try a different query.")
		return nil
	}
	if !quiet {
		fmt.Printf("Modpacks matching %q:\n", query)
		fmt.Println("  Slug / ID          Loaders                         Downloads   Title")
		fmt.Println("  ------------------  --------------------------------  ---------  -----")
	}
	for _, h := range hits {
		loaders := inferLoadersFromCategories(h.Categories)
		fmt.Printf(
			"  %-18s  %-32s  %-10s  %s\n",
			h.Slug,
			strings.Join(loaders, ","),
			humanize.Comma(int64(h.Downloads)),
			h.Title,
		)
	}
	if !quiet {
		fmt.Println()
		fmt.Fprintln(os.Stderr, "Tip: create with: minectl create --modpack <slug> [--type fabric|forge|neoforge|quilt]")
	}
	return nil
}

func inferLoadersFromCategories(categories []string) []string {
	known := []string{"fabric", "forge", "neoforge", "quilt"}

	set := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		set[strings.ToLower(c)] = struct{}{}
	}

	out := make([]string, 0, len(known))
	for _, k := range known {
		if _, ok := set[k]; ok {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return []string{"?unknown"}
	}
	return out
}

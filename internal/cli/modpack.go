package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/modrinth"
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
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	err = m.SetModpack(ctx, args[0], args[1], modpackVersion, modpackForce, ev)
	ev.finish()
	return err
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

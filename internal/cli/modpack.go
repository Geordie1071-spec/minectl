package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/minectl/minectl/internal/modrinth"
	"github.com/minectl/minectl/internal/server"
	"github.com/spf13/cobra"
)

var modpackVersion string

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
	modpackSetCmd.Flags().StringVarP(&modpackVersion, "version", "v", "", "modpack version")
	modpackCmd.AddCommand(modpackSetCmd, modpackInfoCmd, modpackSearchCmd)
}

func runModpackSet(cmd *cobra.Command, args []string) error {
	name, packSlug := args[0], args[1]
	st := getStore()
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	source := "modrinth"
	s.ModpackSource = &source
	s.ModpackID = &packSlug
	if modpackVersion != "" {
		s.ModpackVersion = &modpackVersion
	} else {
		s.ModpackVersion = nil
	}
	if err := st.SaveServer(s); err != nil {
		return err
	}
	if s.ContainerID != "" {
		ctx := context.Background()
		d, err := getDockerClient(ctx)
		if err != nil {
			return err
		}
		defer d.Close()
		s, err = server.RecreateContainer(ctx, d, st, name)
		if err != nil {
			return fmt.Errorf("recreate container with modpack: %w", err)
		}
		if !quiet {
			fmt.Println("Modpack set and server recreated:", name, "→", packSlug)
		}
	} else {
		if !quiet {
			fmt.Println("Modpack set:", name, "→", packSlug, "(start the server to apply)")
		}
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
		fmt.Println("  Slug / ID          Title")
		fmt.Println("  ------------------  -----")
	}
	for _, h := range hits {
		fmt.Printf("  %-18s  %s\n", h.Slug, h.Title)
	}
	if !quiet {
		fmt.Println()
		fmt.Fprintln(os.Stderr, "Tip: create with: minectl create --modpack <slug> --type fabric|forge|quilt")
	}
	return nil
}

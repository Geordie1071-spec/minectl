package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/minectl/minectl/internal/modrinth"
	"github.com/minectl/minectl/internal/server"
	"github.com/spf13/cobra"
)

var (
	modsVersion   string
	modsSearchVer string
	modsSearchLoad string
	modsSearchSrv string
)

var modsCmd = &cobra.Command{
	Use:   "mods",
	Short: "Add, list, remove, enable, or disable mods",
}

var modsAddCmd = &cobra.Command{
	Use:   "add [name] [mod-id-or-slug]",
	Short: "Add a mod from Modrinth",
	Args:  cobra.ExactArgs(2),
	RunE:  runModsAdd,
}

var modsListCmd = &cobra.Command{
	Use:   "list [name]",
	Short: "List installed mods",
	Args:  cobra.ExactArgs(1),
	RunE:  runModsList,
}

var modsSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search Modrinth for mods to add",
	Long:  "Search for mods by name. Use --server to filter by a server's Minecraft version and loader, or set --version and --loader.",
	Args:  cobra.ExactArgs(1),
	RunE:  runModsSearch,
}

func init() {
	modsAddCmd.Flags().StringVar(&modsVersion, "version", "", "specific version (default: latest compatible)")
	modsSearchCmd.Flags().StringVar(&modsSearchSrv, "server", "", "filter by this server's MC version and loader")
	modsSearchCmd.Flags().StringVar(&modsSearchVer, "version", "", "Minecraft version to filter (e.g. 1.21.1)")
	modsSearchCmd.Flags().StringVar(&modsSearchLoad, "loader", "", "loader to filter: fabric, forge, quilt")
	modsCmd.AddCommand(modsAddCmd, modsListCmd, modsSearchCmd)
}

func runModsAdd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	st := getStore()
	if !quiet {
		fmt.Fprintln(os.Stderr, "Warning: individual mods can conflict; prefer a modpack when possible.")
	}
	mod, err := server.AddMod(ctx, st, args[0], args[1], modsVersion)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Mod added:", mod.ModName)
	}
	return nil
}

func runModsList(cmd *cobra.Command, args []string) error {
	st := getStore()
	name := args[0]
	s, err := st.GetServer(name)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("server not found: %q (use 'minectl list' to see server names)", name)
	}
	for _, m := range s.Mods {
		en := "enabled"
		if !m.Enabled {
			en = "disabled"
		}
		fmt.Printf("%s  %s  %s\n", m.ModName, m.VersionID, en)
	}
	return nil
}

func runModsSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	gameVersion := modsSearchVer
	loader := modsSearchLoad
	if modsSearchSrv != "" {
		st := getStore()
		s, err := st.GetServer(modsSearchSrv)
		if err != nil {
			return err
		}
		if s == nil {
			return fmt.Errorf("server not found: %q (use 'minectl list' to see server names)", modsSearchSrv)
		}
		gameVersion = s.MCVersion
		loader = modrinth.NormalizeLoader(s.MCType)
	}
	client := modrinth.NewClient()
	hits, err := client.SearchMods(query, gameVersion, loader)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Println("No mods found. Try a different query or relax --version/--loader.")
		return nil
	}
	if !quiet {
		fmt.Printf("Mods matching %q (version=%s loader=%s):\n", query, orEmpty(gameVersion), orEmpty(loader))
		fmt.Println("  Slug / ID          Title")
		fmt.Println("  ------------------  -----")
	}
	for _, h := range hits {
		fmt.Printf("  %-18s  %s\n", h.Slug, h.Title)
	}
	fmt.Println()
	fmt.Println("  Add one with: minectl mods add <server-name> <slug>")
	return nil
}

func orEmpty(s string) string {
	if s == "" {
		return "any"
	}
	return s
}

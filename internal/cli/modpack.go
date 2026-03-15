package cli

import (
	"context"
	"fmt"

	"github.com/minectl/minectl/internal/server"
	"github.com/minectl/minectl/internal/tui"
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

func init() {
	modpackSetCmd.Flags().StringVarP(&modpackVersion, "version", "v", "", "modpack version")
	modpackCmd.AddCommand(modpackSetCmd, modpackInfoCmd)
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
	// If server has a container, recreate it so env (MODRINTH_MODPACK) is applied
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
			fmt.Println(tui.SuccessStyle.Render("Modpack set and server recreated:"), name, "→", packSlug)
		}
	} else {
		if !quiet {
			fmt.Println(tui.SuccessStyle.Render("Modpack set:"), name, "→", packSlug, "(start the server to apply)")
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

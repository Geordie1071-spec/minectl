package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/minectl/minectl/internal/config"
	"github.com/minectl/minectl/internal/docker"
	"github.com/minectl/minectl/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configDir string
	noColor   bool
	jsonOut   bool
	quiet     bool
)

// rootCmd is the root command
var rootCmd = &cobra.Command{
	Use:   "minectl",
	Short: "Manage Minecraft servers on any Linux VPS using Docker",
	Long:  `minectl creates and manages Minecraft servers (Paper, Fabric, Forge, etc.) via Docker. No cloud, no accounts — just your server.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&configDir, "config", "", "config directory (default: ~/.minectl)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")
	_ = viper.BindPFlag("config_dir", rootCmd.PersistentFlags().Lookup("config"))
	rootCmd.Version = config.Version

	rootCmd.AddCommand(createCmd, listCmd, startCmd, stopCmd, restartCmd, deleteCmd)
	rootCmd.AddCommand(consoleCmd, execCmd, logsCmd, statsCmd)
	rootCmd.AddCommand(backupCmd, modsCmd, modpackCmd, upgradeCmd)
}

func initConfig() {
	if configDir != "" {
		os.Setenv("MINECTL_CONFIG_DIR", configDir)
	}
	if err := store.InitConfigDir(); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Warning: could not init config dir:", err)
	}
}

// getDockerClient returns a Docker client and ensures Docker is running
func getDockerClient(ctx context.Context) (*docker.Client, error) {
	cli, err := docker.New()
	if err != nil {
		return nil, fmt.Errorf("Docker client: %w", err)
	}
	if err := cli.CheckDockerRunning(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("Docker is not running or not accessible: %w", err)
	}
	return cli, nil
}

// getStore returns the state store
func getStore() *store.Store {
	return store.New()
}

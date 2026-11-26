package cli

import (
	"fmt"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/spf13/cobra"
)

var upgradeVersion string
var upgradeNoBackup bool

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [name]",
	Short: "Upgrade server to a new Minecraft version",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpgrade,
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeVersion, "version", "v", "", "target MC version (required)")
	upgradeCmd.Flags().BoolVar(&upgradeNoBackup, "no-backup", false, "skip pre-upgrade backup")
	_ = upgradeCmd.MarkFlagRequired("version")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	before := time.Now()
	if err := server.Upgrade(ctx, d, st, args[0], upgradeVersion, upgradeNoBackup); err != nil {
		return err
	}
	if err := waitIfRestarted(ctx, d, st, args[0], before); err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Upgraded", args[0], "to", upgradeVersion)
	}
	return nil
}

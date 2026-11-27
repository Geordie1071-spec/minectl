package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/update"
	"github.com/spf13/cobra"
)

var (
	updateCheck   bool
	updateVersion string
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update minectl to the latest release",
	Long: `Download the latest minectl release, check it against the release checksums,
and replace this binary. If the panel runs as the minectl service, it's
restarted onto the new version. Minecraft servers keep running.`,
	Example: `  sudo minectl update
  minectl update --check
  sudo minectl update --version v2.0.1`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "only say whether an update is available")
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "install this release instead of the latest (can go back a version)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	rel, err := update.Latest(ctx, updateVersion)
	if err != nil {
		return err
	}
	cur := config.Version
	cmp := update.Compare(cur, rel.Version)
	if updateCheck {
		if cmp < 0 {
			fmt.Printf("minectl %s is available (you have %s). Run: sudo minectl update\n%s\n", rel.Version, cur, rel.URL)
		} else {
			fmt.Printf("minectl %s is the latest release.\n", cur)
		}
		return nil
	}
	if updateVersion == "" && cmp >= 0 {
		fmt.Printf("minectl %s is already the latest release.\n", cur)
		return nil
	}
	if cmp == 0 {
		fmt.Printf("minectl %s is already installed.\n", cur)
		return nil
	}
	if cmp > 0 {
		fmt.Printf("Going back from %s to %s. If %s has already updated the database, %s will refuse to start.\n", cur, rel.Version, cur, rel.Version)
	}
	exe, err := update.Executable()
	if err != nil {
		return err
	}
	if !update.CanReplace(exe) {
		return fmt.Errorf("can't replace %s as this user; run: sudo minectl update", exe)
	}
	fmt.Printf("Downloading minectl %s...\n", rel.Version)
	if err := update.Apply(ctx, rel, exe); err != nil {
		return err
	}
	success("Updated %s to %s", exe, rel.Version)
	restartService()
	return nil
}

// restartService restarts the panel's systemd service if it's running, so it
// picks up the new binary.
func restartService() {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	out, _ := exec.Command("systemctl", "is-active", "minectl").Output()
	if strings.TrimSpace(string(out)) != "active" {
		return
	}
	if os.Geteuid() != 0 {
		fmt.Println("The panel is still running the old version. Restart it: sudo systemctl restart minectl")
		return
	}
	if out, err := exec.Command("systemctl", "restart", "minectl").CombinedOutput(); err != nil {
		fmt.Printf("Couldn't restart the panel (%v): %s\nRestart it: sudo systemctl restart minectl\n", err, out)
		return
	}
	success("Restarted the panel")
}

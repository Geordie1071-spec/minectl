package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Geordie1071-spec/minectl/scripts"
	"github.com/spf13/cobra"
)

var (
	uninstallPurge      bool
	uninstallKeepConfig bool
	uninstallKeepData   bool
	uninstallYes        bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove minectl, its panel service and the server containers",
	Long: `Remove the panel service, close router port forwards and playit.gg tunnels,
stop and remove the Minecraft server containers, and remove this binary.
Worlds, backups and settings are kept unless you pass --purge.

This runs the same steps as scripts/uninstall.sh.`,
	Example: `  minectl uninstall
  minectl uninstall --purge
  minectl uninstall --purge --keep-data --yes`,
	Args: cobra.NoArgs,
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "also delete config (users, settings) and server data (worlds, backups)")
	uninstallCmd.Flags().BoolVar(&uninstallKeepConfig, "keep-config", false, "with --purge, keep the config directory")
	uninstallCmd.Flags().BoolVar(&uninstallKeepData, "keep-data", false, "with --purge, keep the data directory")
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false, "don't ask before removing things")
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if runtime.GOOS == "windows" {
		return errors.New(`on Windows, delete the servers with 'minectl delete --all', then delete minectl.exe
(%LOCALAPPDATA%\Programs\minectl if you used install.ps1), and the minectl folder in your home directory to delete worlds too`)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return errors.New("uninstall needs bash, which isn't on the PATH")
	}
	var flags []string
	for _, f := range []struct {
		on   bool
		name string
	}{{uninstallPurge, "--purge"}, {uninstallKeepConfig, "--keep-config"}, {uninstallKeepData, "--keep-data"}, {uninstallYes, "--yes"}} {
		if f.on {
			flags = append(flags, f.name)
		}
	}
	c := exec.Command(bash, append([]string{"-c", scripts.Uninstall, "uninstall.sh"}, flags...)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = os.Environ()
	// Remove the binary that's actually on the PATH, wherever it was installed.
	if p, err := exec.LookPath(os.Args[0]); err == nil {
		if p, err := filepath.Abs(p); err == nil {
			c.Env = append(c.Env, "MINECTL_INSTALL_DIR="+filepath.Dir(p), "MINECTL_BINARY="+filepath.Base(p))
		}
	}
	return c.Run()
}

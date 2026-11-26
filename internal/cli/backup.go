package cli

import (
	"fmt"

	"github.com/Geordie1071-spec/minectl/internal/server"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create, list, restore, or delete backups",
}

var backupCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a backup now",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupCreate,
}

var backupListCmd = &cobra.Command{
	Use:   "list [name]",
	Short: "List backups",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupList,
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore [name] [backup-id]",
	Short: "Restore from a backup",
	Args:  cobra.ExactArgs(2),
	RunE:  runBackupRestore,
}

func init() {
	backupCmd.AddCommand(backupCreateCmd, backupListCmd, backupRestoreCmd)
}

func runBackupCreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	b, err := server.CreateBackup(ctx, d, st, args[0])
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Backup created:", b.ID, humanize.Bytes(uint64(b.SizeBytes)))
	}
	return nil
}

func runBackupList(cmd *cobra.Command, args []string) error {
	st := getStore()
	backups, err := server.ListBackups(st, args[0])
	if err != nil {
		return err
	}
	for _, b := range backups {
		fmt.Printf("%s  %s  %s\n", b.ID, b.CreatedAt.Format("2006-01-02 15:04"), humanize.Bytes(uint64(b.SizeBytes)))
	}
	return nil
}

func runBackupRestore(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	before := time.Now()
	if err := server.RestoreBackup(ctx, d, st, args[0], args[1]); err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Restored", args[1])
	}
	return waitIfRestarted(ctx, d, st, args[0], before)
}

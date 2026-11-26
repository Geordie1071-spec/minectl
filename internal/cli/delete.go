package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	deletePurge bool
	deleteAll   bool
	deleteYes   bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete [name] | --all",
	Short: "Delete a server, or all servers (container removed; world data kept unless --purge)",
	Args: func(cmd *cobra.Command, args []string) error {
		if deleteAll && len(args) > 0 {
			return errors.New("pass a server name or --all, not both")
		}
		if !deleteAll && len(args) != 1 {
			return errors.New("pass a server name, or --all to delete every server")
		}
		return nil
	},
	RunE: runDelete,
}

func init() {
	deleteCmd.Flags().BoolVar(&deletePurge, "purge", false, "also delete world files and backups (irreversible)")
	deleteCmd.Flags().BoolVar(&deleteAll, "all", false, "delete every server")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "don't ask for confirmation with --all")
}

func runDelete(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	if !deleteAll {
		return deleteOne(ctx, m, args[0])
	}

	servers, err := m.St.ListServers()
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		if !quiet {
			fmt.Println("No servers to delete.")
		}
		return nil
	}
	if !deleteYes {
		names := make([]string, len(servers))
		for i, s := range servers {
			names[i] = s.Name
		}
		what := "their containers (world files are kept)"
		if deletePurge {
			what = "their containers, world files, and backups. This cannot be undone"
		}
		fmt.Fprintf(os.Stderr, "This deletes %d server(s): %s\nand %s.\n", len(servers), strings.Join(names, ", "), what)
		ok, err := confirm(os.Stdin, "Type 'yes' to continue: ")
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("aborted")
		}
	}

	var failed []string
	for _, s := range servers {
		if err := deleteOne(ctx, m, s.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", s.Name, err)
			failed = append(failed, s.Name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not delete: %s", strings.Join(failed, ", "))
	}
	return nil
}

func deleteOne(ctx context.Context, m *service.Manager, name string) error {
	ev := newEvents(name)
	err := m.Delete(ctx, name, deletePurge, ev)
	ev.finish()
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Deleted", name)
		if deletePurge {
			fmt.Fprintln(os.Stderr, "Warning: world data removed (irreversible).")
		}
	}
	return nil
}

func confirm(in io.Reader, prompt string) (bool, error) {
	if f, ok := in.(*os.File); ok && !term.IsTerminal(int(f.Fd())) {
		return false, errors.New("refusing to delete all servers without a terminal to confirm; pass --yes")
	}
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes"), nil
}

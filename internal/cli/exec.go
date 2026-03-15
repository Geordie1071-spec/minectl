package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [name] [command]",
	Short: "Run a single console command on the server",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runExec,
}

func runExec(cmd *cobra.Command, args []string) error {
	name := args[0]
	command := args[1]
	for i := 2; i < len(args); i++ {
		command += " " + args[i]
	}
	ctx := context.Background()
	d, err := getDockerClient(ctx)
	if err != nil {
		return err
	}
	defer d.Close()
	st := getStore()
	s, err := st.GetServer(name)
	if err != nil || s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	out, err := d.ExecRcon(ctx, s.ContainerID, command)
	if err != nil {
		return err
	}
	if out != "" {
		fmt.Println(out)
	}
	return nil
}

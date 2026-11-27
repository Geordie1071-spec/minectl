package cli

import (
	"fmt"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/server"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/spf13/cobra"
)

var propsCmd = &cobra.Command{
	Use:   "props [name]",
	Short: "Show or change game settings (MOTD, difficulty, max players...)",
	Long:  "Show a server's game settings. Change them with: minectl props set <name> key=value ...",
	Args:  cobra.ExactArgs(1),
	RunE:  runPropsShow,
}

var propsSetCmd = &cobra.Command{
	Use:   "set [name] key=value...",
	Short: "Change game settings and restart the server to apply them",
	Example: `  minectl props set survival motd="Welcome!" difficulty=hard max-players=30
  minectl props set survival pvp=false whitelist=true`,
	Args: cobra.MinimumNArgs(2),
	RunE: runPropsSet,
}

var propsUnsetCmd = &cobra.Command{
	Use:   "unset [name] key...",
	Short: "Go back to Minecraft's default for settings",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runPropsUnset,
}

func init() {
	propsCmd.AddCommand(propsSetCmd, propsUnsetCmd)
	rootCmd.AddCommand(propsCmd)
}

func runPropsShow(cmd *cobra.Command, args []string) error {
	s, err := getStore().GetServer(args[0])
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("server not found: %s", args[0])
	}
	values := server.PropertyValues(s.Properties)
	if jsonOut {
		return printJSON(values)
	}
	for _, p := range server.PropertyList {
		v, ok := values[p.Key]
		if !ok {
			v = "(default)"
		}
		fmt.Printf("%-14s %-24s %s\n", p.Key, v, p.Help)
	}
	return nil
}

func changeProps(args []string, edit func(p *domain.Properties) error) error {
	name := args[0]
	s, err := getStore().GetServer(name)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("server not found: %s", name)
	}
	p := domain.Properties{}
	if s.Properties != nil {
		p = *s.Properties
	}
	if err := edit(&p); err != nil {
		return err
	}
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	err = m.SetProperties(ctx, name, p, ev)
	ev.finish()
	return err
}

func runPropsSet(cmd *cobra.Command, args []string) error {
	return changeProps(args, func(p *domain.Properties) error {
		for _, kv := range args[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return fmt.Errorf("%q should look like key=value", kv)
			}
			if v == "" {
				return fmt.Errorf("%s has no value; use 'minectl props unset %s %s' to reset it", k, args[0], k)
			}
			if err := server.SetProperty(p, k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

func runPropsUnset(cmd *cobra.Command, args []string) error {
	return changeProps(args, func(p *domain.Properties) error {
		for _, k := range args[1:] {
			if err := server.SetProperty(p, k, ""); err != nil {
				return err
			}
		}
		return nil
	})
}

var backupScheduleCmd = &cobra.Command{
	Use:   "schedule [name] [hourly|6h|daily|off]",
	Short: "Back up automatically (runs while the panel, minectl serve, is running)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		st := getStore()
		defer st.Close()
		m := &service.Manager{St: st}
		s, err := m.SetBackupSchedule(args[0], strings.ToLower(args[1]))
		if err != nil {
			return err
		}
		if quiet {
			return nil
		}
		if s.BackupSchedule == "" {
			fmt.Printf("Automatic backups for %s are off.\n", s.Name)
			return nil
		}
		fmt.Printf("%s backs up %s; the first one is at %s.\n", s.Name, describeSchedule(s.BackupSchedule), s.AutoBackupNextAt.Local().Format("Mon 15:04"))
		fmt.Println("Scheduled backups run while the panel is running (minectl serve, or the minectl service).")
		return nil
	},
}

func describeSchedule(s string) string {
	switch s {
	case domain.ScheduleHourly:
		return "every hour"
	case domain.Schedule6Hours:
		return "every 6 hours"
	case domain.ScheduleDaily:
		return "every day"
	}
	return s
}

func init() {
	backupCmd.AddCommand(backupScheduleCmd)
}

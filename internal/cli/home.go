package cli

import (
	"fmt"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/spf13/cobra"
)

// Command groups for help, in display order.
var commandGroups = []struct {
	id, title string
	names     []string
}{
	{"servers", "Servers", []string{"create", "list", "start", "stop", "restart", "delete", "logs", "stats", "exec"}},
	{"world", "World, mods and settings", []string{"backup", "props", "mods", "modpack", "upgrade", "resize", "recommend", "versions"}},
	{"share", "Panel and sharing", []string{"serve", "public", "playit"}},
	{"other", "Other", []string{"update", "help"}},
}

// setupHelp groups the commands and styles help. It runs from Execute, after
// every file's init has added its commands.
func setupHelp() {
	for _, g := range commandGroups {
		rootCmd.AddGroup(&cobra.Group{ID: g.id, Title: g.title})
	}
	rootCmd.SetHelpCommandGroupID("other")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	group := map[string]string{}
	for _, g := range commandGroups {
		for _, n := range g.names {
			group[n] = g.id
		}
	}
	// Show each group's commands in the order listed above, not A-Z.
	cobra.EnableCommandSorting = false
	byName := map[string]*cobra.Command{}
	for _, c := range rootCmd.Commands() {
		byName[c.Name()] = c
		if id, ok := group[c.Name()]; ok {
			c.GroupID = id
		}
	}
	rootCmd.ResetCommands()
	for _, g := range commandGroups {
		for _, n := range g.names {
			if c := byName[n]; c != nil {
				rootCmd.AddCommand(c)
				delete(byName, n)
			}
		}
	}
	for _, c := range byName {
		rootCmd.AddCommand(c)
	}
	cobra.AddTemplateFunc("heading", func(s string) string { return paint(sgrAccent+";"+sgrBold, s) })
	cobra.AddTemplateFunc("cmdname", func(s string) string { return bold(s) })
	rootCmd.SetUsageTemplate(usageTemplate)
}

// usageTemplate is cobra's default with styled headings and command names.
const usageTemplate = `{{heading "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{heading "Aliases"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{heading "Examples"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{heading "Commands"}}{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{cmdname (rpad .Name .NamePadding) }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{heading .Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{cmdname (rpad .Name .NamePadding) }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{heading "Additional Commands"}}{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{cmdname (rpad .Name .NamePadding) }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{heading "Flags"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{heading "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

{{heading "Additional help topics"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// runHome is what plain "minectl" shows: who we are, the servers at a
// glance, and where to go next.
func runHome(cmd *cobra.Command, args []string) error {
	var servers []domain.Server
	st := getStore()
	defer st.Close()
	servers, _ = st.ListServers()
	running, stopped := 0, 0
	for _, s := range servers {
		switch s.Status {
		case domain.StatusRunning:
			running++
		case domain.StatusDeleted:
		default:
			stopped++
		}
	}

	var b strings.Builder
	b.WriteString("\n")
	for _, line := range fullLogo() {
		b.WriteString("  " + line + "\n")
	}
	fmt.Fprintf(&b, "\n  %s %s · Minecraft servers on your own machine, with Docker\n\n", bold("minectl"), dim(config.Version))

	if running+stopped == 0 {
		fmt.Fprintf(&b, "  %s  none yet\n\n", dim("Servers"))
	} else {
		fmt.Fprintf(&b, "  %s  %s · %d stopped   %s\n\n", dim("Servers"), paint(sgrGreen, fmt.Sprintf("%d running", running)), stopped, dim("(minectl list)"))
	}

	type tip struct{ cmd, what string }
	tips := []tip{
		{"minectl create -n survival -t paper", "Create a server"},
		{"minectl list", "See your servers and how to join"},
		{"minectl serve", "Open the web panel"},
		{"minectl public <name>", "Let friends join from anywhere"},
	}
	if running+stopped > 0 {
		tips[0] = tip{"minectl logs <name> -f", "Follow a server's console"}
	}
	fmt.Fprintf(&b, "  %s\n", paint(sgrAccent+";"+sgrBold, "Get started"))
	for _, t := range tips {
		fmt.Fprintf(&b, "    %s %s\n", pad(accent(t.cmd), 40), t.what)
	}
	fmt.Fprintf(&b, "\n  %s\n\n", dim("minectl help <command> for details · minectl --help for every command"))
	fmt.Fprint(cmd.OutOrStdout(), b.String())
	return nil
}

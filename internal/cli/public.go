package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/expose"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/spf13/cobra"
)

var (
	publicVia string
	publicOff bool
)

var publicCmd = &cobra.Command{
	Use:   "public [name]",
	Short: "Let friends join from the internet (router port forwarding, or a playit.gg tunnel)",
	Long: `Make a server reachable from the internet.

minectl first asks your router to forward the port (UPnP or NAT-PMP). If the
router won't, or your internet provider shares your address with others
(CGNAT), it opens a playit.gg tunnel instead; link playit once with
'minectl playit link'.`,
	Example: `  minectl public survival
  minectl public survival --via playit
  minectl public survival --off`,
	Args: cobra.ExactArgs(1),
	RunE: runPublic,
}

var playitCmd = &cobra.Command{
	Use:   "playit",
	Short: "Link or unlink a playit.gg account for tunnels",
}

var playitLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link a free playit.gg account so servers can use a tunnel",
	Args:  cobra.NoArgs,
	RunE:  runPlayitLink,
}

var playitUnlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Close every playit.gg tunnel and forget the account",
	Args:  cobra.NoArgs,
	RunE:  runPlayitUnlink,
}

// cloudCheckCmd is for install.sh: on a known cloud it prints the public IP,
// where to open ports in the provider's firewall and any other tip, and
// nothing elsewhere.
var cloudCheckCmd = &cobra.Command{
	Use:    "cloud-check",
	Hidden: true,
	Args:   cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := expose.DetectCloud()
		if c == nil {
			return
		}
		if c.IP != nil {
			fmt.Printf("This machine runs on %s, with the public IP %s.\n", c.Name, c.IP)
		}
		ports := "25565-25575"
		if cloudCheckPanel {
			ports += ", 80 and 443"
		}
		fmt.Printf("On %s, %s.\n", c.Name, c.Firewall(ports))
		if tip := c.Tip(); tip != "" {
			fmt.Println(tip)
		}
	},
}

var cloudCheckPanel bool

func init() {
	cloudCheckCmd.Flags().BoolVar(&cloudCheckPanel, "panel", false, "include the panel's ports")
	rootCmd.AddCommand(cloudCheckCmd)
	publicCmd.Flags().StringVar(&publicVia, "via", service.ViaAuto, "auto, router, or playit")
	publicCmd.Flags().BoolVar(&publicOff, "off", false, "stop forwarding and close the tunnel")
	playitCmd.AddCommand(playitLinkCmd, playitUnlinkCmd)
	rootCmd.AddCommand(publicCmd, playitCmd)
}

func runPublic(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	defer ev.finish()
	if publicOff {
		if err := m.MakePrivate(ctx, args[0], ev); err != nil {
			return err
		}
		ev.Info(args[0] + " is only reachable on your network now.")
		return nil
	}
	pub, err := m.MakePublic(ctx, args[0], publicVia, ev)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(pub)
	}
	if pub.Method != expose.MethodPlayit && !quiet {
		fmt.Println("Keep the panel running (minectl serve) to restore the forward if your router restarts, or run this command again.")
	}
	return nil
}

func runPlayitLink(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	st := getStore()
	defer st.Close()
	m := &service.Manager{St: st}
	code := expose.NewClaimCode()
	fmt.Printf("Open this link and approve minectl (sign up for free if you need to):\n\n  %s\n\nWaiting for approval...\n", expose.ClaimURL(code))
	deadline := time.Now().Add(15 * time.Minute)
	for {
		reqCtx, done := context.WithTimeout(ctx, 20*time.Second)
		status, err := m.PlayitClaim(reqCtx, code)
		done()
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			fmt.Println("Warning:", err)
		case status == service.ClaimLinked:
			fmt.Println("Linked. Servers can now use a tunnel: minectl public <name>")
			return nil
		case status == service.ClaimRejected:
			return fmt.Errorf("the link was rejected on playit.gg")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("gave up waiting for approval; run 'minectl playit link' again")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runPlayitUnlink(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalContext()
	defer cancel()
	m, closeFn, err := newManager(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	ev := newEvents("")
	defer ev.finish()
	if err := m.UnlinkPlayit(ctx, ev); err != nil {
		return err
	}
	ev.Info("playit.gg is unlinked and its tunnels are closed.")
	return nil
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/api"
	"github.com/Geordie1071-spec/minectl/internal/auth"
	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/jobs"
	"github.com/Geordie1071-spec/minectl/internal/service"
	"github.com/Geordie1071-spec/minectl/internal/store"
	"github.com/Geordie1071-spec/minectl/internal/update"
	"github.com/Geordie1071-spec/minectl/internal/watch"
	"github.com/Geordie1071-spec/minectl/internal/web"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/acme/autocert"
)

const defaultListen = "127.0.0.1:8080"

var (
	serveListen     string
	serveDomain     string
	serveEmail      string
	serveTrustProxy bool
	servePublicAddr string
	serviceUser     string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the web panel",
	Long: `Run the minectl web panel and its API.

Without --domain it listens on ` + defaultListen + `; reach it from your computer with an
SSH tunnel, or put a reverse proxy in front and pass --listen and --trust-proxy.

With --domain it gets a free HTTPS certificate from Let's Encrypt and listens on
ports 443 and 80. Point the domain's DNS at this machine first.

The first time it runs it prints a one-time setup token; enter it in the panel to
create the admin account.`,
	Args: cobra.NoArgs,
	RunE: runServe,
}

var installServiceCmd = &cobra.Command{
	Use:   "install-service",
	Short: "Run the panel at boot with systemd (needs sudo)",
	Args:  cobra.NoArgs,
	RunE:  runInstallService,
}

var resetPasswordCmd = &cobra.Command{
	Use:   "reset-password [username]",
	Short: "Set a new random password for a panel user",
	Args:  cobra.ExactArgs(1),
	RunE:  runResetPassword,
}

func init() {
	for _, c := range []*cobra.Command{serveCmd, installServiceCmd} {
		c.Flags().StringVar(&serveListen, "listen", "", "address to listen on (default "+defaultListen+", or :443 with --domain)")
		c.Flags().StringVar(&serveDomain, "domain", "", "domain for automatic HTTPS, e.g. panel.example.com")
		c.Flags().StringVar(&serveEmail, "email", "", "email for Let's Encrypt expiry notices (optional)")
		c.Flags().BoolVar(&serveTrustProxy, "trust-proxy", false, "trust X-Forwarded-For from a reverse proxy")
		c.Flags().StringVar(&servePublicAddr, "public-address", "", "address players use to join, e.g. play.example.com (default: worked out automatically)")
	}
	installServiceCmd.Flags().StringVar(&serviceUser, "user", "", "user the panel runs as (default: the user running sudo)")
	serveCmd.AddCommand(installServiceCmd, resetPasswordCmd)
	rootCmd.AddCommand(serveCmd)
}

func setupTokenPath() string {
	return filepath.Join(config.ConfigDir(), "setup-token")
}

func runServe(cmd *cobra.Command, args []string) error {
	sigCtx, stop := signalContext()
	defer stop()

	d, err := docker.New()
	if err != nil {
		return fmt.Errorf("Docker client: %w", err)
	}
	defer d.Close()
	if err := d.CheckDockerRunning(sigCtx); err != nil {
		log.Printf("Warning: Docker isn't reachable yet (%v); server actions will fail until it is", err)
	}
	st := getStore()
	defer st.Close()

	jobCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	runner, err := jobs.New(jobCtx, st)
	if err != nil {
		return err
	}

	setupToken, err := prepareSetupToken(st)
	if err != nil {
		return err
	}
	restartCh := make(chan struct{}, 1)
	handler := api.New(api.Config{
		Manager:       &service.Manager{D: d, St: st, Wait: true},
		Store:         st,
		Jobs:          runner,
		SetupToken:    setupToken,
		OnSetupDone:   func() { _ = os.Remove(setupTokenPath()) },
		TrustProxy:    serveTrustProxy,
		PublicAddress: strings.TrimSpace(servePublicAddr),
		UI:            web.Handler(),
		Restart: func() {
			select {
			case restartCh <- struct{}{}:
			default:
			}
		},
	})

	watcher := watch.New(st, &service.Manager{D: d, St: st}, runner)
	go watcher.RunScheduler(jobCtx)
	go watcher.RunCrashWatch(jobCtx)

	servers, url, err := buildServers(handler, sigCtx)
	if err != nil {
		return err
	}
	log.Printf("minectl panel %s listening on %s", config.Version, url)
	if setupToken != "" {
		log.Printf("First-time setup: open %s/setup and enter this token: %s", url, setupToken)
		log.Printf("(the token is also saved in %s)", setupTokenPath())
	}
	if serveDomain == "" && isLoopback(servers[0].Addr) {
		_, port, _ := net.SplitHostPort(servers[0].Addr)
		log.Printf("This address only works on this machine. From your computer, run:")
		log.Printf("  ssh -L %s:localhost:%s <user>@<this-server>   then open http://localhost:%s", port, port, port)
	}

	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		go func(srv *http.Server) {
			var err error
			if srv.TLSConfig != nil {
				err = srv.ListenAndServeTLS("", "")
			} else {
				err = srv.ListenAndServe()
			}
			if !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("listen on %s: %w", srv.Addr, err)
			}
		}(srv)
	}

	restart := false
	select {
	case err = <-errCh:
	case <-sigCtx.Done():
		log.Printf("Shutting down...")
	case <-restartCh:
		log.Printf("Updated; restarting onto the new version...")
		restart = true
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		_ = srv.Shutdown(shutdownCtx)
	}
	waitJobs(runner, cancelJobs)
	if restart {
		_ = st.Close()
		return reexec()
	}
	return err
}

// reexec replaces this process with the binary on disk, keeping the PID so
// systemd sees the same service.
func reexec() error {
	exe, err := update.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}

// waitJobs gives running jobs a minute to finish before cancelling them.
func waitJobs(r *jobs.Runner, cancel context.CancelFunc) {
	done := make(chan struct{})
	go func() { r.Wait(); close(done) }()
	select {
	case <-done:
		return
	case <-time.After(time.Minute):
	}
	log.Printf("Cancelling jobs that are still running")
	cancel()
	<-done
}

func prepareSetupToken(st *store.Store) (string, error) {
	n, err := st.CountUsers()
	if err != nil {
		return "", err
	}
	if n > 0 {
		_ = os.Remove(setupTokenPath())
		return "", nil
	}
	token := auth.RandomToken()
	if err := os.WriteFile(setupTokenPath(), []byte(token+"\n"), 0600); err != nil {
		return "", err
	}
	return token, nil
}

func buildServers(h http.Handler, baseCtx context.Context) ([]*http.Server, string, error) {
	base := func(net.Listener) context.Context { return baseCtx }
	newServer := func(addr string, h http.Handler) *http.Server {
		return &http.Server{
			Addr:              addr,
			Handler:           h,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			BaseContext:       base,
			ErrorLog:          log.New(os.Stderr, "http: ", log.LstdFlags),
		}
	}
	if serveDomain == "" {
		addr := serveListen
		if addr == "" {
			addr = defaultListen
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return nil, "", fmt.Errorf("invalid --listen %q (use host:port, e.g. 0.0.0.0:8080): %w", addr, err)
		}
		return []*http.Server{newServer(addr, h)}, "http://" + displayAddr(addr), nil
	}

	domain := strings.ToLower(strings.TrimSpace(serveDomain))
	if strings.ContainsAny(domain, "/: ") {
		return nil, "", fmt.Errorf("--domain should be a bare hostname like panel.example.com, not %q", serveDomain)
	}
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache(filepath.Join(config.ConfigDir(), "certs")),
		Email:      serveEmail,
	}
	addr := serveListen
	if addr == "" {
		addr = ":443"
	}
	tlsSrv := newServer(addr, h)
	tlsSrv.TLSConfig = m.TLSConfig()
	redirect := newServer(":80", m.HTTPHandler(nil))
	url := "https://" + domain
	if _, port, _ := net.SplitHostPort(addr); port != "443" {
		url += ":" + port
	}
	return []*http.Server{tlsSrv, redirect}, url, nil
}

func displayAddr(addr string) string {
	host, port, _ := net.SplitHostPort(addr)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "<this-server-ip>"
	}
	return net.JoinHostPort(host, port)
}

func isLoopback(addr string) bool {
	host, _, _ := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

const unitTemplate = `[Unit]
Description=minectl panel
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
User=%s
SupplementaryGroups=docker
Environment=HOME=%s
ExecStart=%s
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
%s
[Install]
WantedBy=multi-user.target
`

const unitPath = "/etc/systemd/system/minectl.service"

// panelBinaryDir holds a copy of minectl owned by the panel's user, so the
// panel can update itself. That grants nothing new: the user is in the docker
// group, which is already as good as root.
const panelBinaryDir = "/opt/minectl/bin"

// panelBinary moves the running binary into panelBinaryDir, owned by u, and
// leaves a symlink where it was so "minectl" on the PATH keeps working.
func panelBinary(bin string, u *user.User) (string, error) {
	dest := filepath.Join(panelBinaryDir, "minectl")
	if u.Username == "root" || bin == dest {
		return bin, nil
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	if err := os.MkdirAll(panelBinaryDir, 0755); err != nil {
		return "", err
	}
	if err := os.Chown(panelBinaryDir, uid, gid); err != nil {
		return "", err
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		return "", err
	}
	tmp := dest + ".new"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		return "", err
	}
	if err := os.Chown(tmp, uid, gid); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	link := bin + ".link"
	_ = os.Remove(link)
	if err := os.Symlink(dest, link); err != nil {
		return "", err
	}
	if err := os.Rename(link, bin); err != nil {
		return "", err
	}
	fmt.Printf("Moved minectl to %s (owned by %s, so the panel can update itself); %s links to it.\n", dest, u.Username, bin)
	return dest, nil
}

func runInstallService(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		return errors.New("install-service needs Linux with systemd")
	}
	if os.Geteuid() != 0 {
		return errors.New("run it with sudo: sudo minectl serve install-service ...")
	}
	name := serviceUser
	if name == "" {
		name = os.Getenv("SUDO_USER")
	}
	if name == "" {
		name = "root"
	}
	if name == "root" {
		fmt.Println("Note: the panel will run as root. Pass --user <name> to run it as the user who owns your servers instead.")
	}
	u, err := user.Lookup(name)
	if err != nil {
		return fmt.Errorf("user %s: %w", name, err)
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	if bin, err = filepath.EvalSymlinks(bin); err != nil {
		return err
	}
	if bin, err = panelBinary(bin, u); err != nil {
		return err
	}
	execArgs := []string{bin, "serve"}
	extra := ""
	if serveListen != "" {
		execArgs = append(execArgs, "--listen", serveListen)
	}
	if serveDomain != "" {
		execArgs = append(execArgs, "--domain", serveDomain)
		extra = "AmbientCapabilities=CAP_NET_BIND_SERVICE\n"
	}
	if serveEmail != "" {
		execArgs = append(execArgs, "--email", serveEmail)
	}
	if serveTrustProxy {
		execArgs = append(execArgs, "--trust-proxy")
	}
	if servePublicAddr != "" {
		execArgs = append(execArgs, "--public-address", servePublicAddr)
	}
	for _, a := range execArgs {
		if strings.ContainsAny(a, " \t\n\"'\\%$") {
			return fmt.Errorf("unsupported character in %q", a)
		}
	}
	unit := fmt.Sprintf(unitTemplate, u.Username, u.HomeDir, strings.Join(execArgs, " "), extra)
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return err
	}
	fmt.Println("Wrote", unitPath)
	for _, c := range [][]string{{"daemon-reload"}, {"enable", "minectl"}, {"restart", "minectl"}} {
		out, err := exec.Command("systemctl", c...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl %s: %v\n%s", strings.Join(c, " "), err, out)
		}
	}
	fmt.Println("The panel is running and starts at boot.")
	fmt.Println("  Status: systemctl status minectl")
	fmt.Println("  Logs:   journalctl -u minectl -f")

	tokenFile := filepath.Join(u.HomeDir, ".minectl", "setup-token")
	for i := 0; i < 20; i++ {
		if data, err := os.ReadFile(tokenFile); err == nil {
			fmt.Println()
			fmt.Println("Setup token (enter it in the panel to create your admin account):")
			fmt.Println("  " + strings.TrimSpace(string(data)))
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if serveDomain != "" {
		fmt.Printf("\nOpen https://%s/setup (make sure ports 80 and 443 are open in your firewall).\n", serveDomain)
	}
	return nil
}

func runResetPassword(cmd *cobra.Command, args []string) error {
	st := getStore()
	defer st.Close()
	u, _, err := st.GetUserByName(args[0])
	if err != nil {
		return err
	}
	if u == nil {
		return fmt.Errorf("no panel user named %s", args[0])
	}
	password := auth.RandomToken()[:20]
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.SetUserPassword(u.ID, hash); err != nil {
		return err
	}
	if err := st.DeleteUserSessions(u.ID, ""); err != nil {
		return err
	}
	fmt.Printf("New password for %s: %s\nThey've been logged out everywhere; change it after logging in.\n", u.Username, password)
	return nil
}

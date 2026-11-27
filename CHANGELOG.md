# Changelog

## 2.1.0 (2026-11-27)

- On AWS, Google Cloud, Azure and Oracle Cloud, minectl finds the machine's public IP itself, so the panel and `minectl list` show the right join address without `--public-address`.
- minectl recognises AWS, Google Cloud, Azure, Oracle Cloud, Hetzner, DigitalOcean and Vultr, and recommends the easiest setup for each: where to open ports in that provider's console, and on AWS and Google Cloud, keeping the IP from changing. The advice shows in the panel, `minectl public`, `install-service` and at the end of the install script. minectl never needs access to your cloud account.
- The install script installs Docker on Amazon Linux.
- The install script's reminder to log out and back in (for Docker without sudo) moved to the next steps, where it's seen.
- The README has a setup table for each provider.
- The install script and `install-service` open ports on Oracle Cloud's Ubuntu images, which reject everything but SSH in plain iptables.
- `minectl uninstall` removes minectl the way `minectl update` updates it: the panel service, port forwards and tunnels, server containers and the binary, with `--purge` to delete worlds and settings too.
- The install script's fallback build with Go works: it passed `go install` a URL instead of the module path.
- `sudo minectl serve install-service --domain` warns when the domain doesn't point at the machine yet, before the HTTPS certificate fails.
- Windows has a one-line installer: `irm https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/install.ps1 | iex` installs Docker Desktop (with winget) and minectl, adds it to your PATH and opens the firewall when run as administrator.
- `minectl update` on Windows names the right download (a .zip) and points to the installer.

## 2.0.2 (2026-11-27)

- When your user can't use Docker yet (just added to the docker group), minectl says so once and explains the fix, instead of repeating "permission denied" every few seconds.
- `sudo minectl serve install-service` labels the panel for SELinux itself (Oracle Linux, RHEL, Fedora), so the service starts without a manual `chcon`, and with `--domain` it opens ports 80 and 443 in `firewalld` or `ufw`.
- The install script installs Docker and opens the firewall on Oracle Linux, Rocky, Alma, RHEL, CentOS Stream and Fedora.

## 2.0.1 (2026-11-27)

- The ⋯ menus on the Players and People pages are no longer cut off by their card.
- On cloud machines that only see a private IP (Azure, Oracle and others), `minectl list` shows the panel's `--domain` or `--public-address` as the join address, and `minectl public` knows the machine is already on the internet.

## 2.0.0 (2026-11-27)

The big one: minectl now has a web panel. Every part of the CLI was also
reworked so servers start reliably and say why when they don't.

### Web panel (`minectl serve`)
- A web interface built into the binary:
  - A dashboard of every server, with status, players and the join address.
  - A new-server wizard: pick a server type, search Modrinth, or paste a CurseForge link. A players slider updates the memory and CPU recommendation live.
  - Live install and start progress.
  - Server pages with live usage graphs, a console with command history, player management, backups, mods and modpacks, resize, upgrade, and delete.
  - A players page with everyone who has joined, online or not: ops, whitelist and bans at a glance, unban in one click, and kick or ban with a reason.
  - Pages for users and roles, account settings and API tokens, and a paged activity log.
  - Light and dark themes, and it works on phones.
- A JSON API under `/api/v1`. Job progress, logs and stats stream live over server-sent events.
- First-run setup token, so nobody can claim a freshly exposed panel before you do.
- Admins, plus per-server operators and viewers, for sharing servers with friends.
- Personal API tokens for scripts and bots.
- An audit log of every change.
- Automatic HTTPS with `--domain`.
- `sudo minectl serve install-service` runs the panel at boot.
- `minectl serve reset-password` gets a locked-out user back in.
- `install.sh --domain panel.example.com` sets up minectl and the panel in one command.

### Running servers unattended
- Scheduled backups every hour, every 6 hours, or every day, set from the Backups tab or `minectl backup schedule`. They run while the panel is running.
- Crash alerts: the panel notices servers that exit on their own and records why (out of memory, an error exit), and can message a Discord webhook.
  A server that crashes 4 times in 10 minutes is stopped instead of restarted again and again.
- Game settings: MOTD, max players, difficulty, gamemode, view distance, PvP, hardcore, online mode, whitelist and flight, from the Settings tab or `minectl props`.
- Panel settings for the Discord webhook and how many backups to keep.
- The panel's own database (users, settings, tokens) is backed up daily, and admins can download a copy.

### Hosting from home
- `minectl public` and a "Make public" button let friends join a server running on your own computer.
  minectl asks the router to forward the port (UPnP or NAT-PMP). When it can't, or the connection is behind CGNAT, it falls back to a free playit.gg tunnel.
- Link playit.gg once with `minectl playit link` or from Panel settings; minectl runs the agent and makes the tunnels.
- The panel keeps router forwards alive and shows the public address as the join address.
- On a VPS there's nothing to forward: the panel says the server is already reachable and which firewall port to open.

### Command line
- Run `minectl` on its own for the logo, your servers at a glance and the commands you'll need next.
- Grouped, coloured help, a `list` with a status dot and the address to join, and clear ✓ / Warning / ✗ messages. Output stays plain when piped or with `NO_COLOR` or `--no-color`.

### Updates
- `minectl update` installs the latest release after checking it against the release checksums, and restarts the panel.
- The panel tells admins about new releases and can update itself with one click.
- The install script verifies checksums. It no longer falls back to installing 1.0 when GitHub can't be reached, and it restarts the panel after updating.
- An older minectl refuses to open a database written by a newer one, instead of hiding your servers.

### Modpacks and versions
- Modrinth modpacks with `--modpack`: the loader, Minecraft version and Java version are detected from the pack.
- CurseForge modpacks with `--curseforge <url or slug>`.
- Versions are checked against the real Mojang, Paper, Fabric, Quilt, Forge
  and NeoForge lists, with "did you mean" suggestions. The right Java image is chosen automatically.
- `minectl versions <type>` lists the available versions.

### Resources
- `--memory auto` (the default) recommends memory and CPU from the server type, players, and mod count.
- `minectl recommend` previews the numbers, and `minectl resize` changes them later.
- Containers get headroom above the Java heap, so they aren't killed for running out of memory.

### Reliability
- `create`, `start`, `restart`, `restore` and `upgrade` wait until the server is ready.
- When a server fails to start, the relevant log lines are shown with a plain-English hint
  (client-only mod, out of memory, port in use, SELinux, and so on).
- Real backups with pruning, and a restore that rolls back if it fails.
- `status` follows Docker, and `logs -f` streams live output.
- Works on SELinux systems such as Fedora.
- `delete --all` removes every server (asks for confirmation; skip it with `--yes`).
- `modpack set` refuses to downgrade an existing world unless you pass `--force`.

### Under the hood
- The uninstall script removes the panel service, port forwards, tunnels and server containers, not just the binary. Worlds are kept unless you pass `--purge`.
- State moved to SQLite (`~/.minectl/minectl.db`). `servers.json` and
  `config.json` are imported automatically on first run.
  **After upgrading, 1.0 can no longer see your servers.**
- Per-server locks stop the CLI and the panel from changing the same server at once.
- Server names are validated: 1-32 lowercase letters, digits and dashes.
- `minectl exec` output no longer contains stray binary bytes.
- `stats` reports CPU correctly on cgroup v2 and leaves out page cache.
- CI runs lint, race-checked tests, builds for 5 platforms, an end-to-end suite
  against real Docker, staticcheck and govulncheck.

## 1.0 (2026-03-18)

Initial release.

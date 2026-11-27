# Changelog

## 2.0.0 (2026-11-27)

The big one: minectl now has a web panel. Every part of the CLI was also
reworked so servers start reliably and say why when they don't.

### Web panel (`minectl serve`)
- A web interface built into the binary:
  - A dashboard of every server, with status, players and the join address.
  - A new-server wizard: pick a server type, search Modrinth, or paste a CurseForge link. A players slider updates the memory and CPU recommendation live.
  - Live install and start progress.
  - Server pages with live usage graphs, a console with command history, player management, backups, mods and modpacks, resize, upgrade, and delete.
  - Pages for users and roles, account settings and API tokens, and an activity log.
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

### Hosting from home
- `minectl public` and a "Make public" button let friends join a server running on your own computer.
  minectl asks the router to forward the port (UPnP or NAT-PMP). When it can't, or the connection is behind CGNAT, it falls back to a free playit.gg tunnel.
- Link playit.gg once with `minectl playit link` or from Panel settings; minectl runs the agent and makes the tunnels.
- The panel keeps router forwards alive and shows the public address as the join address.

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

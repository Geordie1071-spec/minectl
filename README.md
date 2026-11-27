# minectl

![minectl](assets/logo.png)

A CLI to create and manage Minecraft servers on any Linux (or Windows with Docker) using Docker. No cloud, no accounts — just your machine and [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server).

## Quick start

```bash
# Create a Paper 1.21.1 server with 4GB RAM
minectl create --name survival --type paper --version 1.21.1 --memory 4G

# List servers
minectl list

# Start / stop / restart
minectl start survival
minectl stop survival
minectl restart survival

# One-off server command
minectl exec survival "say Hello"
```

## Requirements

- **Docker** — Engine 20.x+ running (e.g. `docker ps` works).
- **Go 1.26+** — only if building from source.

## Install

- **From source:** `go install github.com/Geordie1071-spec/minectl/cmd/minectl@latest`
- **Script:** `curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/install.sh | bash`
- **Releases:** download the binary for your OS/arch from [Releases](https://github.com/Geordie1071-spec/minectl/releases).

Note: the install script also sets up bash tab-completion for `minectl` automatically.

### Uninstall

To remove the `minectl` binary:
`curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/uninstall.sh | bash`

To also delete config/data (destructive):
`MINECTL_PURGE=1 curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/uninstall.sh | bash`

### Install on your VPS (one command)

On a fresh **Ubuntu or Debian** VPS (e.g. Hetzner, DigitalOcean), run:

```bash
curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/install.sh | bash
```

This installs **Docker** (if missing) and **minectl**, creates data dirs, and prints next steps. Then:

```bash
minectl create -n myserver -t paper -m 4G
minectl list
```

**Firewall:** open TCP port **25565** so players can connect (e.g. Hetzner Cloud Firewall, or `sudo ufw allow 25565 && sudo ufw reload`).

To skip Docker install (you already have it): `MINECTL_SKIP_DOCKER=1 curl -sSL ... | bash`  
To pin version: `MINECTL_VERSION=v2.0.0 curl -sSL ... | bash`

## Web panel

`minectl serve` runs a web panel and API from the same binary. Use it to manage servers from a browser and give friends limited access. From the panel you can:
- Create servers from a server type, a Modrinth search, or a CurseForge link, with live install progress.
- Use a live console, live CPU and memory graphs, and the player list (whitelist, op, kick, ban).
- Make and restore backups, manage mods and modpacks, resize, and upgrade.
- Schedule automatic backups, change game settings (MOTD, difficulty, max players...), and get a Discord message when a server crashes.

On a VPS with a domain pointed at it (ports 80 and 443 open), this installs minectl and the panel as a systemd service with automatic HTTPS:

```bash
curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/install.sh | bash -s -- --domain panel.example.com
```

**Firewall:** open TCP **80** and **443** for the panel and **25565** for players (for example `sudo ufw allow 80,443,25565/tcp`).

It prints a one-time **setup token**. Open `https://panel.example.com/setup`, enter the token, and create your admin account. Until someone does, nobody else can claim the panel.

Without a domain the panel only listens on `127.0.0.1:8080`. Reach it through an SSH tunnel: `ssh -L 8080:localhost:8080 you@your-vps`, then open http://localhost:8080.

| | |
|---|---|
| `minectl serve` | Run the panel in the foreground (`--listen`, `--domain`, `--email`, `--trust-proxy`, `--public-address`) |
| `sudo minectl serve install-service` | Run it at boot with systemd (same flags, plus `--user`) |
| `minectl serve reset-password <user>` | Set a new random password if someone is locked out |

**Join address.** The panel shows players the address to type into Minecraft. It uses the panel's own hostname, or this machine's network IP when you open the panel at `localhost`. To set it yourself, for example to a domain pointed at the machine, pass `--public-address play.example.com`.

**Roles.** Admins can do everything. Other users only see the servers they've been given access to:
- **Operator:** start, stop, restart, console, backups, and managing players.
- **Viewer:** status, logs, stats, and the player list.

**Behind a reverse proxy** (nginx, Caddy): run `minectl serve --listen 127.0.0.1:8080 --trust-proxy`. Pass the original `Host` header through, and set `X-Forwarded-Proto: https`.

The CLI and the panel share the same state (`~/.minectl/minectl.db`), so servers created in one appear in the other. If one is already changing a server, the other is told it's busy.

### Running servers unattended

The panel also looks after servers while you're away. These run inside `minectl serve`, so use `install-service` to keep it running at boot.

**Scheduled backups.** Back up every hour, every 6 hours, or every day from a server's Backups tab, or with `minectl backup schedule survival daily` (`off` turns it off). The panel keeps the newest backups and deletes older ones. Change how many it keeps (1-100) under Panel settings.

**Crash alerts.** The panel watches Docker for servers that exit on their own and tells those apart from stops you asked for. It records the reason on the server's Overview, for example running out of memory or exiting with an error. Docker restarts a crashed server automatically. A server that crashes 4 times within 10 minutes is stopped instead, so it doesn't keep restarting. To get a message in Discord, paste a channel webhook URL into Panel settings and press **Send a test**. The webhook is stored on the server and never shown again.

**Game settings.** Change the MOTD, max players, difficulty, gamemode, view distance, PvP, hardcore, online mode, whitelist and flight from a server's Settings tab, or from the CLI:

```bash
minectl props survival                                   # show current values
minectl props set survival motd="Welcome!" difficulty=hard max-players=30
minectl props unset survival pvp                         # back to Minecraft's default
```

Saving restarts the server to apply the change. Unsetting a setting writes Minecraft's default back to `server.properties`. Settings minectl doesn't manage are left as they are.

### API

Everything the panel does goes through a JSON API under `/api/v1`. Create a personal token with `POST /api/v1/tokens` and send it as `Authorization: Bearer mct_...`. A token has the same permissions as its owner.

```bash
curl -H "Authorization: Bearer $TOKEN" https://panel.example.com/api/v1/servers
curl -H "Authorization: Bearer $TOKEN" -X POST https://panel.example.com/api/v1/servers \
  -d '{"name":"survival","type":"paper","memory":"auto","players":10}'
```

Long operations (create, start, stop, restart, backup, restore, upgrade, resize, modpack and mod changes) return `202` with a job. Follow it with `GET /api/v1/jobs/{id}/events`, a server-sent event stream. Live logs and stats are streamed the same way at `/api/v1/servers/{name}/logs` and `/stats`. Every change is recorded in the audit log (`GET /api/v1/audit`, admins only).

### Developing the panel

The React app lives in `web/`. Release binaries have it embedded; for a local build with the UI, run `make panel`. Without it, `go build` still works and serves a placeholder page.

```bash
minectl serve                 # API on 127.0.0.1:8080
npm --prefix web run dev      # UI with hot reload on http://localhost:5173, proxied to the API
make smoke                    # Playwright smoke test against the built binary
```

## Commands

| Command | Description |
|--------|-------------|
| `create` | Create and start a new server |
| `list` | List all servers |
| `start` / `stop` / `restart` | Lifecycle |
| `delete` | Remove container (use `--purge` to delete world data) |
| `exec` | Run a single command (e.g. `exec survival "op Steve"`) |
| `logs` | Tail logs (`--follow` to stream) |
| `stats` | CPU/memory usage |
| `backup create/list/restore` | Backups |
| `backup schedule` | Back up automatically: `hourly`, `6h`, `daily` or `off` (runs while the panel is running) |
| `props` / `props set` / `props unset` | Show or change game settings |
| `mods add/list` | Modrinth mods (Fabric/Forge) |
| `modpack set/info/search` | Modpack (Modrinth) |
| `upgrade` | Upgrade MC version |
| `resize` / `recommend` | Change or preview memory and CPU |
| `versions` | List available Minecraft versions for a server type |
| `serve` | Run the web panel and API |

## Config and data

- Config: `~/.minectl/` (or `MINECTL_CONFIG_DIR`).
- Servers state and settings: `~/.minectl/minectl.db` (SQLite). Older `servers.json` / `config.json` files are imported automatically on first run and renamed to `*.migrated`.
- World data: `/opt/minectl/servers/<name>` by default (configurable).

## Testing (Docker required)

With Docker running, you can verify:

```bash
# 1. List servers (from store; no Docker needed)
minectl list
minectl list --all    # include stopped

# 2. Create and start a vanilla/Paper server
minectl create -n my-server -t paper -v 1.21.1 -m 4G -p 25565

# 3. Create a server with a Modrinth modpack (one command)
minectl create -n modpack-server -t fabric -v 1.21.1 -m 4G -p 25566 --modpack all-of-fabric-6

# Or create first, then set modpack and recreate container
minectl create -n other -t fabric -v 1.21.1 -m 2G --no-start
minectl modpack set other all-of-fabric-6
minectl start other

# 4. Check status and start/stop
minectl list
minectl start my-server
minectl stop my-server
```

## License

MIT.

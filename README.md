# minectl

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

# Console (interactive) or one-off command
minectl console survival
minectl exec survival "say Hello"
```

## Requirements

- **Docker** — Engine 20.x+ running (e.g. `docker ps` works).
- **Go 1.23+** — only if building from source.

## Install

- **From source:** `go install github.com/minectl/minectl/cmd/minectl@latest`
- **Script:** `curl -sSL https://raw.githubusercontent.com/minectl/minectl/main/scripts/install.sh | bash`
- **Releases:** download the binary for your OS/arch from [Releases](https://github.com/minectl/minectl/releases).

### Install on your VPS (one command)

On a fresh **Ubuntu or Debian** VPS (e.g. Hetzner, DigitalOcean), run:

```bash
curl -sSL https://raw.githubusercontent.com/minectl/minectl/main/scripts/vps-install.sh | bash
```

This installs **Docker** (if missing) and **minectl**, creates data dirs, and prints next steps. Then:

```bash
minectl create -n myserver -t paper -m 4G
minectl list
```

**Firewall:** open TCP port **25565** so players can connect (e.g. Hetzner Cloud Firewall, or `sudo ufw allow 25565 && sudo ufw reload`).

To skip Docker install (you already have it): `MINECTL_SKIP_DOCKER=1 curl -sSL ... | bash`  
To pin version: `MINECTL_VERSION=v0.1.0 curl -sSL ... | bash`

## Commands

| Command | Description |
|--------|-------------|
| `create` | Create and start a new server |
| `list` | List all servers |
| `start` / `stop` / `restart` | Lifecycle |
| `delete` | Remove container (use `--purge` to delete world data) |
| `console` | Interactive TUI console |
| `exec` | Run a single command (e.g. `exec survival "op Steve"`) |
| `logs` | Tail logs (`--follow` to stream) |
| `stats` | CPU/memory usage |
| `backup create/list/restore` | Backups |
| `mods add/list` | Modrinth mods (Fabric/Forge) |
| `modpack set/info` | Modpack (Modrinth) |
| `upgrade` | Upgrade MC version |

## Config and data

- Config: `~/.minectl/` (or `MINECTL_CONFIG_DIR`).
- Servers state: `~/.minectl/servers.json`.
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

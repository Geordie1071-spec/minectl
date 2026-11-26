# Changelog

## 2.0.0 (unreleased)

The big one: minectl now has a web panel. Every part of the CLI was also
reworked so servers start reliably and say why when they don't.

### Web panel (`minectl serve`)
- A JSON API under `/api/v1` with live job progress, logs and stats over
  server-sent events. The React interface is coming in this release.
- First-run setup token, so nobody can claim a freshly exposed panel before you do.
- Admins, plus per-server operators and viewers, for sharing servers with friends.
- Personal API tokens for scripts and bots.
- An audit log of every change.
- Automatic HTTPS with `--domain`.
- `sudo minectl serve install-service` runs the panel at boot.
- `minectl serve reset-password` gets a locked-out user back in.
- `install.sh --domain panel.example.com` sets up minectl and the panel in one command.

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

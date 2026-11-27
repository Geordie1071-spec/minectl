#!/usr/bin/env bash
set -e

BINARY="${MINECTL_BINARY:-minectl}"
INSTALL_DIR="${MINECTL_INSTALL_DIR:-/usr/local/bin}"

# If running via `sudo ...`, $HOME may be /root; use the original user's home.
TARGET_USER="${SUDO_USER:-$(whoami)}"
TARGET_HOME=$(getent passwd "$TARGET_USER" | cut -d: -f6)
TARGET_HOME="${TARGET_HOME:-$HOME}"

CONFIG_DIR="${MINECTL_CONFIG_DIR:-${TARGET_HOME}/.minectl}"
DATA_DIR="${MINECTL_DATA_DIR:-/opt/minectl}"

PURGE="${MINECTL_PURGE:-0}"
KEEP_CONFIG="${MINECTL_KEEP_CONFIG:-0}"
KEEP_DATA="${MINECTL_KEEP_DATA:-0}"
ASSUME_YES="${MINECTL_YES:-0}"

usage() {
  cat <<EOF
Usage: uninstall.sh [--purge] [--keep-config] [--keep-data] [--yes]

Removes the minectl panel service, stops and removes the Minecraft server
containers (worlds are kept), closes router port forwards and playit.gg
tunnels, and removes the minectl binary.

Options:
  --purge           Also delete config (users, settings) and server data (worlds, backups).
  --keep-config     With --purge, keep the config directory.
  --keep-data       With --purge, keep the data directory.
  --yes             Don't ask before removing things.

Env vars:
  MINECTL_INSTALL_DIR=/path    (default: /usr/local/bin)
  MINECTL_CONFIG_DIR=/path     (default: \$HOME/.minectl)
  MINECTL_DATA_DIR=/path       (default: /opt/minectl)
  MINECTL_PURGE=1              (same as --purge)
  MINECTL_YES=1                (same as --yes)
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --purge) PURGE=1 ;;
    --keep-config) KEEP_CONFIG=1 ;;
    --keep-data) KEEP_DATA=1 ;;
    --yes) ASSUME_YES=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
  shift
done

# confirm asks on the terminal, which also works under "curl ... | bash".
confirm() {
  [ "${ASSUME_YES}" = "1" ] && return 0
  if ! { exec 3</dev/tty; } 2>/dev/null; then
    echo "$1 There's no terminal to answer on; re-run with --yes to confirm."
    exit 1
  fi
  read -r -u 3 -p "$1 [y/N] " ans
  exec 3<&-
  case "${ans}" in
    y|Y|yes|YES) return 0 ;;
  esac
  echo "Aborted."
  exit 1
}

# as_user runs a command as the user who owns the servers, with their config.
as_user() {
  if [ "$(id -u)" = "0" ] && [ "$TARGET_USER" != "root" ]; then
    sudo -u "$TARGET_USER" env HOME="$TARGET_HOME" MINECTL_CONFIG_DIR="$CONFIG_DIR" "$@"
  else
    env MINECTL_CONFIG_DIR="$CONFIG_DIR" "$@"
  fi
}

docker_cmd() {
  if docker info >/dev/null 2>&1; then docker "$@"; else sudo docker "$@"; fi
}

LINK="${INSTALL_DIR}/${BINARY}"
REAL="$LINK"
[ -L "$LINK" ] && REAL=$(readlink -f "$LINK")

echo "minectl uninstall"
echo "=================="
echo "This removes the minectl panel service, stops and removes the Minecraft"
echo "server containers, closes port forwards and tunnels, and removes the binary."
if [ "${PURGE}" = "1" ]; then
  [ "${KEEP_CONFIG}" != "1" ] && echo "It also DELETES the config: ${CONFIG_DIR} (users, settings)"
  [ "${KEEP_DATA}" != "1" ] && echo "It also DELETES the server data: ${DATA_DIR} (worlds, backups)"
else
  echo "Worlds, backups and settings are kept (use --purge to delete them)."
fi
confirm "Continue?"

# 1. The panel service.
if command -v systemctl >/dev/null 2>&1 && [ -f /etc/systemd/system/minectl.service ]; then
  echo "Removing the minectl panel service..."
  sudo systemctl disable --now minectl >/dev/null 2>&1 || true
  sudo rm -f /etc/systemd/system/minectl.service
  sudo systemctl daemon-reload || true
fi

# 2. Router port forwards and playit tunnels, while minectl can still undo them.
if [ -x "$REAL" ]; then
  NAMES=$(as_user "$REAL" list --json 2>/dev/null | grep -o '"name": *"[^"]*"' | sed 's/.*"\([^"]*\)"$/\1/' || true)
  for n in $NAMES; do
    as_user "$REAL" public "$n" --off --quiet >/dev/null 2>&1 || true
  done
  as_user "$REAL" playit unlink >/dev/null 2>&1 || true
fi

# 3. Containers. World data stays in the data directory.
if command -v docker >/dev/null 2>&1; then
  IDS=$(docker_cmd ps -aq --filter "name=^minectl-" --filter "name=^minectl_playit$" 2>/dev/null || true)
  if [ -n "$IDS" ]; then
    echo "Stopping and removing Minecraft server containers..."
    # shellcheck disable=SC2086
    docker_cmd rm -f $IDS >/dev/null
  fi
fi

# 4. The binary: the link on the PATH and, after install-service, the real
#    file the panel owns.
for f in "$LINK" "$REAL"; do
  if [ -e "$f" ] || [ -L "$f" ]; then
    echo "Removing $f"
    sudo rm -f "$f" 2>/dev/null || rm -f "$f"
  fi
done
rmdir "${DATA_DIR}/bin" 2>/dev/null || sudo rmdir "${DATA_DIR}/bin" 2>/dev/null || true

if [ "${PURGE}" != "1" ]; then
  echo ""
  echo "minectl is removed. Your worlds and backups are still in ${DATA_DIR},"
  echo "and settings in ${CONFIG_DIR}. To delete them too:"
  echo "  curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/uninstall.sh | bash -s -- --purge"
  exit 0
fi

if [ "${KEEP_CONFIG}" != "1" ] && [ -d "${CONFIG_DIR}" ]; then
  echo "Deleting ${CONFIG_DIR}"
  sudo rm -rf "${CONFIG_DIR}" 2>/dev/null || rm -rf "${CONFIG_DIR}"
fi
if [ "${KEEP_DATA}" != "1" ] && [ -d "${DATA_DIR}" ]; then
  echo "Deleting ${DATA_DIR}"
  sudo rm -rf "${DATA_DIR}" 2>/dev/null || rm -rf "${DATA_DIR}"
fi
echo "Uninstall complete."

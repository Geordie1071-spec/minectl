#!/usr/bin/env bash
set -e

BINARY="${MINECTL_BINARY:-minectl}"
INSTALL_DIR="${MINECTL_INSTALL_DIR:-/usr/local/bin}"

# If running via `sudo ...`, $HOME may be /root; try to infer the original user's home.
if [ -n "${SUDO_USER:-}" ]; then
  TARGET_HOME="$(eval echo "~$SUDO_USER")"
else
  TARGET_HOME="${HOME:-}"
fi

CONFIG_DIR="${MINECTL_CONFIG_DIR:-${TARGET_HOME}/.minectl}"
DATA_DIR="${MINECTL_DATA_DIR:-/opt/minectl}"

PURGE="${MINECTL_PURGE:-0}"
KEEP_CONFIG="${MINECTL_KEEP_CONFIG:-0}"
KEEP_DATA="${MINECTL_KEEP_DATA:-0}"
ASSUME_YES="${MINECTL_YES:-0}"

usage() {
  cat <<EOF
Usage: uninstall.sh [--purge] [--keep-config] [--keep-data] [--yes]

Default: removes only the minectl binary from ${INSTALL_DIR}.

Options:
  --purge           Also remove config and data directories.
  --keep-config     Do not remove config directory (implies --purge won't delete it).
  --keep-data       Do not remove data directory (implies --purge won't delete it).
  --yes             Do not prompt; assume 'yes' for destructive actions.

Env vars:
  MINECTL_INSTALL_DIR=/path
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

echo "minectl uninstall"
echo "=================="
echo "Binary: ${INSTALL_DIR}/${BINARY}"

if [ -f "${INSTALL_DIR}/${BINARY}" ]; then
  if [ "${ASSUME_YES}" = "1" ]; then
    sudo rm -f "${INSTALL_DIR}/${BINARY}" || rm -f "${INSTALL_DIR}/${BINARY}"
  else
    echo "Removing ${INSTALL_DIR}/${BINARY}..."
    sudo rm -f "${INSTALL_DIR}/${BINARY}" || rm -f "${INSTALL_DIR}/${BINARY}"
  fi
else
  echo "Not found: ${INSTALL_DIR}/${BINARY} (nothing to remove)."
fi

if [ "${PURGE}" != "1" ]; then
  echo ""
  echo "Run again with --purge if you want to delete config and server data:"
  echo "  MINECTL_PURGE=1 curl -sSL https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/uninstall.sh | bash"
  exit 0
fi

if [ "${KEEP_CONFIG}" = "1" ] && [ "${KEEP_DATA}" = "1" ]; then
  echo "Nothing to purge (both KEEP_CONFIG and KEEP_DATA are set)."
  exit 0
fi

echo ""
echo "Purging minectl config/data:"
if [ "${KEEP_CONFIG}" != "1" ]; then
  echo "  Config: ${CONFIG_DIR}"
fi
if [ "${KEEP_DATA}" != "1" ]; then
  echo "  Data:   ${DATA_DIR}"
fi

if [ "${ASSUME_YES}" != "1" ]; then
  read -r -p "Continue? [y/N] " ans
  case "${ans}" in
    y|Y|yes|YES) ;;
    *) echo "Aborted."; exit 1 ;;
  esac
fi

if [ "${KEEP_CONFIG}" != "1" ] && [ -d "${CONFIG_DIR}" ]; then
  sudo rm -rf "${CONFIG_DIR}" || rm -rf "${CONFIG_DIR}"
fi

if [ "${KEEP_DATA}" != "1" ] && [ -d "${DATA_DIR}" ]; then
  sudo rm -rf "${DATA_DIR}" || rm -rf "${DATA_DIR}"
fi

echo "Uninstall complete."


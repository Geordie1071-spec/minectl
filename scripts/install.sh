#!/usr/bin/env bash
set -e

REPO="${MINECTL_REPO:-https://github.com/Geordie1071-spec/minectl}"
INSTALL_DIR="${MINECTL_INSTALL_DIR:-/usr/local/bin}"
BINARY="minectl"

echo "minectl installer"
echo "================="

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
[ "$OS" = "linux" ] || { echo "This script is for Linux."; exit 1; }

TARGET_USER="${SUDO_USER:-$(whoami)}"
TARGET_HOME=$(getent passwd "$TARGET_USER" | cut -d: -f6)
TARGET_HOME="${TARGET_HOME:-$HOME}"

# Panel options, as flags (curl ... | bash -s -- --panel --domain x) or env vars.
PANEL="${MINECTL_PANEL:-}"
DOMAIN="${MINECTL_DOMAIN:-}"
EMAIL="${MINECTL_EMAIL:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --panel) PANEL=1 ;;
    --domain) DOMAIN="$2"; PANEL=1; shift ;;
    --email) EMAIL="$2"; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

if [ -z "${MINECTL_SKIP_DOCKER}" ] && ! command -v docker &>/dev/null; then
  if command -v apt-get &>/dev/null; then
    echo "Docker not found. Installing Docker..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker.io
    sudo systemctl enable --now docker 2>/dev/null || true
    if [ -n "$SUDO_USER" ]; then
      sudo usermod -aG docker "$SUDO_USER" 2>/dev/null || true
      echo "Added $SUDO_USER to group 'docker'. You may need to log out and back in for it to take effect."
    else
      echo "Run: sudo usermod -aG docker \$USER   then log out and back in."
    fi
  else
    echo "Docker not found. Install Docker first: https://docs.docker.com/engine/install/"
    exit 1
  fi
else
  echo "Docker: $(command -v docker || echo 'not found')"
fi

VERSION="${MINECTL_VERSION:-}"
REPO_PATH="${REPO#https://github.com/}"
REPO_PATH="${REPO_PATH#http://github.com/}"
REPO_PATH="${REPO_PATH%.git}"
if [ -z "$VERSION" ] && command -v curl &>/dev/null && [ -n "$REPO_PATH" ]; then
  VERSION=$(curl -sSLf "https://api.github.com/repos/${REPO_PATH}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/') || true
fi
if [ -z "$VERSION" ]; then
  # Never guess a version: an old release can't read a newer database.
  echo "Couldn't find the latest minectl release (GitHub may be rate-limiting this machine)."
  echo "Try again in a while, or pick a version: MINECTL_VERSION=v2.0.0 curl ... | bash"
  exit 1
fi

# Updating an install where the panel owns the binary: write to the real file
# the symlink points at, so the panel gets the new version too.
TARGET="${INSTALL_DIR}/${BINARY}"
if [ -L "$TARGET" ]; then
  TARGET=$(readlink -f "$TARGET")
fi

INSTALLED=
ARCHIVE="minectl_${VERSION#v}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${REPO}/releases/download/${VERSION}/${ARCHIVE}"
echo "Downloading minectl ${VERSION} ($OS/$ARCH)..."
if command -v curl &>/dev/null; then
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT
  if curl -sSLf -o "${TMP}/${ARCHIVE}" "$DOWNLOAD_URL"; then
    if curl -sSLf -o "${TMP}/checksums.txt" "${REPO}/releases/download/${VERSION}/checksums.txt"; then
      if ! (cd "$TMP" && grep " ${ARCHIVE}\$" checksums.txt | sha256sum -c --status -); then
        echo "The download doesn't match the release checksum. Not installing it."
        exit 1
      fi
      echo "Checksum OK."
    else
      echo "Warning: this release has no checksums.txt, so the download couldn't be verified."
    fi
    tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP" "$BINARY"
    sudo mkdir -p "$(dirname "$TARGET")"
    # Keep the owner of an existing binary (the panel may own it).
    OWNER=""
    [ -e "$TARGET" ] && OWNER=$(stat -c '%u:%g' "$TARGET")
    sudo install -m 0755 "${TMP}/${BINARY}" "${TARGET}.new"
    [ -n "$OWNER" ] && sudo chown "$OWNER" "${TARGET}.new"
    sudo mv -f "${TARGET}.new" "$TARGET"
    echo "Installed minectl ${VERSION} to ${TARGET}"
    INSTALLED=1
  fi
fi

if [ -z "${INSTALLED}" ]; then
  if command -v go &>/dev/null; then
    echo "Download failed. Installing from source with Go..."
    GOBIN=$(go env GOPATH 2>/dev/null)/bin
    (cd /tmp && go install "${REPO}/cmd/minectl@${VERSION}" 2>/dev/null) || (cd /tmp && go install "${REPO}/cmd/minectl@latest" 2>/dev/null)
    if [ -f "${GOBIN}/${BINARY}" ]; then
      sudo mkdir -p "$INSTALL_DIR"
      sudo cp "${GOBIN}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
      sudo chmod +x "${INSTALL_DIR}/${BINARY}"
      echo "Installed minectl to ${INSTALL_DIR}/${BINARY}"
    else
      echo "Built to ${GOBIN}/${BINARY}. Add ${GOBIN} to your PATH."
    fi
  else
    echo "Could not install minectl (no release for this arch or Go not installed)."
    echo "  Option 1: Build on this machine: go install ${REPO}/cmd/minectl@latest"
    echo "  Option 2: Download from ${REPO}/releases"
    exit 1
  fi
fi

DATA_USER="${SUDO_USER:-$(whoami)}"
if [ -d /opt/minectl ] 2>/dev/null; then
  sudo chown -R "${DATA_USER}:${DATA_USER}" /opt/minectl 2>/dev/null || true
fi
if [ ! -d /opt/minectl/servers ]; then
  echo "Creating /opt/minectl/servers (you can use --data-dir to use a different path)."
  sudo mkdir -p /opt/minectl/servers /opt/minectl/backups
  sudo chown -R "${DATA_USER}:${DATA_USER}" /opt/minectl 2>/dev/null || true
fi

if [ -z "$PANEL" ] && [ -n "$INSTALLED" ] && command -v systemctl &>/dev/null && systemctl is-active --quiet minectl 2>/dev/null; then
  echo "Restarting the panel onto the new version..."
  sudo systemctl restart minectl
fi

if [ -n "$PANEL" ]; then
  echo ""
  echo "Setting up the web panel..."
  PANEL_ARGS=""
  [ -n "$DOMAIN" ] && PANEL_ARGS="$PANEL_ARGS --domain $DOMAIN"
  [ -n "$EMAIL" ] && PANEL_ARGS="$PANEL_ARGS --email $EMAIL"
  # shellcheck disable=SC2086
  sudo "${INSTALL_DIR}/${BINARY}" serve install-service --user "$TARGET_USER" $PANEL_ARGS
  if [ -z "$DOMAIN" ]; then
    echo ""
    echo "The panel listens on 127.0.0.1:8080. From your computer, run:"
    echo "  ssh -L 8080:localhost:8080 ${TARGET_USER}@<this-server>   then open http://localhost:8080/setup"
    echo "Or re-run with --domain panel.example.com for automatic HTTPS."
  fi
fi

echo ""
echo "Next steps:"
echo "  1. Create a server:  minectl create -n myserver -t paper -m 4G"
echo "  2. With a modpack:   minectl create -n rlcraft -t forge -m 4G --modpack rl-craft"
echo "  3. List servers:      minectl list"
echo "  4. Open port 25565 in your firewall so players can connect."
echo ""

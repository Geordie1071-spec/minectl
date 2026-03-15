#!/usr/bin/env bash
# One-command install for minectl on a VPS (Ubuntu/Debian).
# Installs Docker if needed, then minectl. Usage: curl -sSL <url> | bash
# Optional: MINECTL_VERSION=v0.1.0 to pin version; MINECTL_SKIP_DOCKER=1 to skip Docker install.
set -e

REPO="${MINECTL_REPO:-https://github.com/Geordie1071-spec/minectl}"
INSTALL_DIR="${MINECTL_INSTALL_DIR:-/usr/local/bin}"
BINARY="minectl"

echo "minectl — VPS install"
echo "====================="

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
[ "$OS" = "linux" ] || { echo "This script is for Linux VPS."; exit 1; }

# 1. Install Docker if missing (Debian/Ubuntu)
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

# 2. Resolve version (latest release or pinned)
VERSION="${MINECTL_VERSION:-}"
if [ -z "$VERSION" ] && command -v curl &>/dev/null; then
  # e.g. https://github.com/minectl/minectl -> minectl/minectl
  REPO_PATH="${REPO#https://github.com/}"
  REPO_PATH="${REPO_PATH#http://github.com/}"
  REPO_PATH="${REPO_PATH%.git}"
  [ -n "$REPO_PATH" ] && VERSION=$(curl -sSLf "https://api.github.com/repos/${REPO_PATH}/releases/latest" 2>/dev/null | grep -oP '"tag_name":\s*"\K[^"]+' || true)
fi
[ -z "$VERSION" ] && VERSION="v0.1.0"

# 3. Download and install minectl
INSTALLED=
DOWNLOAD_URL="${REPO}/releases/download/${VERSION}/minectl_${VERSION#v}_${OS}_${ARCH}.tar.gz"
if command -v curl &>/dev/null; then
  if curl -sSLf -o "/tmp/${BINARY}.tar.gz" "$DOWNLOAD_URL" 2>/dev/null; then
    tar -xzf "/tmp/${BINARY}.tar.gz" -C /tmp
    sudo mkdir -p "$INSTALL_DIR"
    sudo mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    sudo chmod +x "${INSTALL_DIR}/${BINARY}"
    rm -f "/tmp/${BINARY}.tar.gz"
    echo "Installed minectl to ${INSTALL_DIR}/${BINARY}"
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

# 4. Prepare data directory (optional; minectl can create with correct perms if we run as same user)
DATA_USER="${SUDO_USER:-$(whoami)}"
if [ -d /opt/minectl ] 2>/dev/null; then
  sudo chown -R "${DATA_USER}:${DATA_USER}" /opt/minectl 2>/dev/null || true
fi
if [ ! -d /opt/minectl/servers ]; then
  echo "Creating /opt/minectl/servers (you can use --data-dir to use a different path)."
  sudo mkdir -p /opt/minectl/servers /opt/minectl/backups
  sudo chown -R "${DATA_USER}:${DATA_USER}" /opt/minectl 2>/dev/null || true
fi

echo ""
echo "Next steps:"
echo "  1. Create a server:  minectl create -n myserver -t paper -m 4G"
echo "  2. With a modpack:   minectl create -n rlcraft -t forge -m 4G --modpack rl-craft"
echo "  3. List servers:      minectl list"
echo "  4. Open port 25565 in your firewall so players can connect."
echo ""

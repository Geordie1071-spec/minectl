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

# add_to_docker_group lets the user run minectl without sudo. It applies
# from their next login, which the next steps remind them of.
RELOGIN=
add_to_docker_group() {
  if [ "$TARGET_USER" != "root" ]; then
    sudo usermod -aG docker "$TARGET_USER" 2>/dev/null || true
    echo "Added $TARGET_USER to the 'docker' group."
    RELOGIN=1
  fi
}

if [ -z "${MINECTL_SKIP_DOCKER}" ] && ! command -v docker &>/dev/null; then
  if command -v apt-get &>/dev/null; then
    echo "Docker not found. Installing Docker..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker.io
    sudo systemctl enable --now docker 2>/dev/null || true
    add_to_docker_group
  elif command -v dnf &>/dev/null && [ "$(. /etc/os-release && echo "$ID")" = "amzn" ]; then
    # Amazon Linux packages Docker itself; Docker's repositories don't support it.
    echo "Docker not found. Installing Docker..."
    sudo dnf install -y -q docker
    sudo systemctl enable --now docker 2>/dev/null || true
    add_to_docker_group
  elif command -v dnf &>/dev/null; then
    # Oracle Linux, Rocky, Alma, RHEL, CentOS Stream and Fedora: Docker's own
    # packages. --allowerasing swaps out the Podman packages some of these
    # ship, which clash with Docker.
    echo "Docker not found. Installing Docker..."
    OS_ID=$(. /etc/os-release && echo "$ID")
    DOCKER_REPO="https://download.docker.com/linux/centos/docker-ce.repo"
    [ "$OS_ID" = "fedora" ] && DOCKER_REPO="https://download.docker.com/linux/fedora/docker-ce.repo"
    sudo curl -fsSL "$DOCKER_REPO" -o /etc/yum.repos.d/docker-ce.repo
    sudo dnf install -y -q --allowerasing docker-ce docker-ce-cli containerd.io
    sudo systemctl enable --now docker 2>/dev/null || true
    add_to_docker_group
  else
    echo "Docker not found. Install Docker first: https://docs.docker.com/engine/install/"
    exit 1
  fi
else
  echo "Docker: $(command -v docker || echo 'not found')"
fi

# Open the firewall when the machine runs one: Minecraft's ports, and the
# panel's when it will be public. Cloud providers have their own firewall too.
open_ports() {
  local ports="25565-25575"
  [ -n "$DOMAIN" ] && ports="$ports 80 443"
  if command -v firewall-cmd &>/dev/null && sudo firewall-cmd --state &>/dev/null; then
    echo "Opening ports in firewalld: $ports"
    for p in $ports; do
      sudo firewall-cmd -q --permanent --add-port="${p}/tcp"
    done
    sudo firewall-cmd -q --reload
  elif command -v ufw &>/dev/null && sudo ufw status 2>/dev/null | grep -q "Status: active"; then
    echo "Opening ports in ufw: $ports"
    for p in $ports; do
      sudo ufw allow "${p/-/:}/tcp" >/dev/null
    done
  elif command -v iptables &>/dev/null && sudo iptables -S INPUT 2>/dev/null | grep -q -- "-j REJECT"; then
    # Oracle Cloud's Ubuntu images reject everything but SSH in plain iptables.
    echo "Opening ports in iptables: $ports"
    for p in $ports; do
      sudo iptables -C INPUT -p tcp --dport "${p/-/:}" -j ACCEPT 2>/dev/null ||
        sudo iptables -I INPUT 1 -p tcp --dport "${p/-/:}" -j ACCEPT
    done
    sudo netfilter-persistent save &>/dev/null ||
      echo "Warning: these rules last until the next reboot. To keep them: sudo apt install iptables-persistent"
  fi
}
open_ports

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
    (cd /tmp && go install "github.com/${REPO_PATH}/cmd/minectl@${VERSION}" 2>/dev/null) || (cd /tmp && go install "github.com/${REPO_PATH}/cmd/minectl@latest" 2>/dev/null)
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
    echo "  Option 1: Build on this machine: go install github.com/${REPO_PATH}/cmd/minectl@latest"
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
  # With SELinux enforcing, systemd only runs binaries labelled as such. The
  # folder is labelled too, so binaries "minectl update" writes there inherit it.
  if command -v getenforce &>/dev/null && [ "$(getenforce)" = "Enforcing" ] && [ -d /opt/minectl/bin ]; then
    echo "Labelling the panel's binary for SELinux..."
    sudo chcon -R -t bin_t /opt/minectl/bin
    if command -v semanage &>/dev/null; then
      sudo semanage fcontext -a -t bin_t '/opt/minectl/bin(/.*)?' 2>/dev/null || true
    fi
    sudo systemctl restart minectl
  fi
  if [ -z "$DOMAIN" ]; then
    echo ""
    echo "The panel listens on 127.0.0.1:8080. From your computer, run:"
    echo "  ssh -L 8080:localhost:8080 ${TARGET_USER}@<this-server>   then open http://localhost:8080/setup"
    echo "Or re-run with --domain panel.example.com for automatic HTTPS."
  fi
fi

echo ""
echo "Next steps:"
if [ -n "$RELOGIN" ]; then
  echo "  0. Log out and SSH back in (or run: newgrp docker), so you can use Docker without sudo."
fi
echo "  1. Create a server:  minectl create -n myserver -t paper -m 4G"
echo "  2. With a modpack:   minectl create -n rlcraft -t forge -m 4G --modpack rl-craft"
echo "  3. List servers:      minectl list"
CLOUD_ARGS=""
[ -n "$DOMAIN" ] && CLOUD_ARGS="--panel"
# shellcheck disable=SC2086
CLOUD=$("${INSTALL_DIR}/${BINARY}" cloud-check $CLOUD_ARGS 2>/dev/null || true)
if [ -n "$CLOUD" ]; then
  echo "  4. Let players in:"
  echo "$CLOUD" | sed 's/^/     /'
else
  echo "  4. On a cloud server, also open TCP 25565-25575 (and 80/443 for the panel) in your provider's firewall."
fi
echo ""

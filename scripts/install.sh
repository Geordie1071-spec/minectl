#!/usr/bin/env bash
# Install minectl — Minecraft server manager (Docker-based)
set -e

REPO="${MINECTL_REPO:-https://github.com/Geordie1071-spec/minectl}"
LATEST_URL="${REPO}/releases/latest"
API_URL="${REPO/releases\/latest/releases}"
INSTALL_DIR="${MINECTL_INSTALL_DIR:-/usr/local/bin}"
BINARY="minectl"

echo "minectl installer"
echo "================="

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
  linux)  ;;
  darwin) ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Resolve version: use MINECTL_VERSION, or latest release from GitHub, or default
VERSION="${MINECTL_VERSION:-}"
if [ -z "$VERSION" ] && command -v curl >/dev/null 2>&1; then
  REPO_PATH="${REPO#https://github.com/}"
  REPO_PATH="${REPO_PATH#http://github.com/}"
  REPO_PATH="${REPO_PATH%.git}"
  [ -n "$REPO_PATH" ] && VERSION=$(curl -sSLf "https://api.github.com/repos/${REPO_PATH}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/') || true
fi
[ -z "$VERSION" ] && VERSION="v0.1.0"
URL="${REPO}/releases/download/${VERSION}/minectl_${VERSION#v}_${OS}_${ARCH}.tar.gz"

echo "Downloading $BINARY $VERSION ($OS/$ARCH)..."
if command -v curl >/dev/null 2>&1; then
  curl -sSLf -o "/tmp/${BINARY}.tar.gz" "$URL" || true
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "/tmp/${BINARY}.tar.gz" "$URL" || true
else
  echo "Need curl or wget"; exit 1
fi

if [ ! -f "/tmp/${BINARY}.tar.gz" ]; then
  echo "Download failed (no release or wrong arch)."
  if command -v go >/dev/null 2>&1; then
    echo "Installing from source with Go..."
    go install "${REPO}/cmd/minectl@latest" 2>/dev/null || true
    GOBIN=$(go env GOPATH 2>/dev/null)/bin
    if [ -f "${GOBIN}/${BINARY}" ]; then
      mkdir -p "$INSTALL_DIR"
      cp "${GOBIN}/${BINARY}" "${INSTALL_DIR}/${BINARY}" 2>/dev/null || sudo cp "${GOBIN}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
      chmod +x "${INSTALL_DIR}/${BINARY}" 2>/dev/null || sudo chmod +x "${INSTALL_DIR}/${BINARY}"
      echo "Installed to ${INSTALL_DIR}/${BINARY}"
      exit 0
    fi
  fi
  echo "Try: go install ${REPO}/cmd/minectl@latest   or download from ${REPO}/releases"
  exit 1
fi

tar -xzf "/tmp/${BINARY}.tar.gz" -C /tmp
mkdir -p "$INSTALL_DIR"
mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"
rm -f "/tmp/${BINARY}.tar.gz"

echo "Installed to ${INSTALL_DIR}/${BINARY}"
if ! command -v docker >/dev/null 2>&1; then
  echo "Warning: Docker not found. Install Docker to use minectl."
else
  echo "Run: minectl create --name myserver --type paper --memory 2G"
fi

#!/usr/bin/env bash
set -euo pipefail

# vpnctl one-line bootstrap installer
# Designed for Ubuntu 24.04 LTS (amd64)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}=== vpnctl Bootstrap Installer ===${NC}\n"

# 1. Check Root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Error: This script must be run as root (use sudo).${NC}"
   exit 1
fi

# 2. Check Architecture
ARCH=$(uname -m)
if [[ "$ARCH" != "x86_64" ]]; then
    echo -e "${RED}Error: Architecture $ARCH is not supported in v0.1 (x86_64/amd64 required).${NC}"
    exit 1
fi

# 3. Check OS
if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    if [[ "$ID" != "ubuntu" ]]; then
        echo -e "${YELLOW}Warning: Detected non-Ubuntu distribution ($ID). vpnctl v0.1 officially supports Ubuntu 24.04.${NC}"
    fi
fi

# 4. Check dependencies (curl, ufw)
if ! command -v curl &> /dev/null; then
    echo -e "Installing curl..."
    apt-get update -qq && apt-get install -y -qq curl
fi

if ! command -v ufw &> /dev/null; then
    echo -e "Installing ufw..."
    apt-get update -qq && apt-get install -y -qq ufw
fi

INSTALL_DIR="/usr/local/bin"
TARGET="${INSTALL_DIR}/veilshard"
ALIAS="${INSTALL_DIR}/vpnctl"

# If local binary exists in current directory, copy it
if [[ -f "./veilshard" ]]; then
    echo -e "Installing local veilshard binary to ${TARGET}..."
    cp "./veilshard" "$TARGET"
    chmod +x "$TARGET"
elif [[ -f "./vpnctl" ]]; then
    echo -e "Installing local binary to ${TARGET}..."
    cp "./vpnctl" "$TARGET"
    chmod +x "$TARGET"
elif [[ -f "./cmd/veilshard/main.go" ]] && command -v go &> /dev/null; then
    echo -e "Building veilshard from local source..."
    go build -ldflags="-s -w" -o "$TARGET" ./cmd/veilshard
    chmod +x "$TARGET"
else
    # Download latest release from GitHub
    echo -e "Downloading latest veilshard binary..."
    LATEST_TAG=$(curl -s "https://api.github.com/repos/ghosttT0/veilshard/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "v0.2.0")
    if [[ -z "$LATEST_TAG" ]]; then
        LATEST_TAG="v0.2.0"
    fi
    DOWNLOAD_URL="https://github.com/ghosttT0/veilshard/releases/download/${LATEST_TAG}/veilshard-linux-amd64"
    echo -e "Fetching ${DOWNLOAD_URL}..."
    if curl -fSL "$DOWNLOAD_URL" -o "$TARGET"; then
        chmod +x "$TARGET"
    else
        echo -e "${YELLOW}Could not download remote binary. Please build using: go build -o veilshard ./cmd/veilshard${NC}"
        exit 1
    fi
fi

# Create backwards compatibility symlink
ln -sf "$TARGET" "$ALIAS"

echo -e "\n${GREEN}✓ Veilshard installed successfully to ${TARGET} (alias: ${ALIAS}).${NC}\n"

# Run install command
echo -e "Running initial deployment:\n"
"$TARGET" install "$@"

#!/bin/bash
set -e

# ElastiCat Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/elastic/elasticat/main/install.sh | bash

REPO="elastic/elasticat"
BINARY_NAME="elasticat"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}==>${NC} $1"
}

success() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}==>${NC} $1"
}

error() {
    echo -e "${RED}==>${NC} $1"
    exit 1
}

# Detect OS
detect_os() {
    local os
    os="$(uname -s)"
    case "$os" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) error "Windows is not supported by this installer. Please download from GitHub releases." ;;
        *) error "Unsupported operating system: $os" ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac
}

# Get the latest release tag from GitHub
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "Failed to fetch latest version from GitHub"
    fi
    echo "$version"
}

# Build the download URL
build_download_url() {
    local version="$1"
    local os="$2"
    local arch="$3"
    
    local binary_name="${BINARY_NAME}-${os}-${arch}"
    echo "https://github.com/${REPO}/releases/download/${version}/${binary_name}"
}

# Find a suitable install directory
find_install_dir() {
    # Try /usr/local/bin first (requires sudo usually)
    if [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
        return
    fi
    
    # Fall back to ~/.local/bin
    local local_bin="$HOME/.local/bin"
    mkdir -p "$local_bin"
    echo "$local_bin"
}

# Check if directory is in PATH
check_path() {
    local dir="$1"
    if [[ ":$PATH:" != *":$dir:"* ]]; then
        warn "$dir is not in your PATH"
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "  export PATH=\"$dir:\$PATH\""
        echo ""
    fi
}

main() {
    echo ""
    echo "  ╔═══════════════════════════════════════╗"
    echo "  ║         ElastiCat Installer           ║"
    echo "  ╚═══════════════════════════════════════╝"
    echo ""

    # Detect platform
    info "Detecting platform..."
    local os=$(detect_os)
    local arch=$(detect_arch)
    success "Detected: ${os}/${arch}"

    # Get latest version
    info "Fetching latest release..."
    local version=$(get_latest_version)
    success "Latest version: ${version}"

    # Build download URL
    local url=$(build_download_url "$version" "$os" "$arch")
    info "Downloading from: ${url}"

    # Find install directory
    local install_dir=$(find_install_dir)
    local install_path="${install_dir}/${BINARY_NAME}"
    info "Installing to: ${install_path}"

    # Download binary
    local tmp_file=$(mktemp)
    if ! curl -fsSL "$url" -o "$tmp_file"; then
        rm -f "$tmp_file"
        error "Failed to download binary. Check if the release exists for ${os}/${arch}"
    fi

    # Make executable and move to install location
    chmod +x "$tmp_file"
    
    if [ "$install_dir" = "/usr/local/bin" ] && [ ! -w "/usr/local/bin" ]; then
        info "Requesting sudo access to install to /usr/local/bin..."
        sudo mv "$tmp_file" "$install_path"
    else
        mv "$tmp_file" "$install_path"
    fi

    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        success "ElastiCat installed successfully!"
        echo ""
        echo "  Run 'elasticat --help' to get started"
        echo ""
    else
        success "Binary installed to ${install_path}"
        check_path "$install_dir"
    fi
}

main "$@"



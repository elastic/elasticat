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

# Find the data directory for elasticat assets (compose stack, etc.)
find_data_dir() {
    local os="$1"
    if [ "$os" = "darwin" ]; then
        echo "$HOME/Library/Application Support/elasticat"
        return
    fi

    # Linux and other unix-likes: use XDG
    local xdg_data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
    echo "${xdg_data_home}/elasticat"
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

# Build the download URL for the archive
build_download_url() {
    local version="$1"
    local os="$2"
    local arch="$3"
    
    local archive_name="${BINARY_NAME}-${version}-${os}-${arch}.tar.gz"
    echo "https://github.com/${REPO}/releases/download/${version}/${archive_name}"
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

# Find the doc directory based on install location
find_doc_dir() {
    local install_dir="$1"
    if [ "$install_dir" = "/usr/local/bin" ]; then
        echo "/usr/local/share/doc/elasticat"
    else
        echo "$HOME/.local/share/doc/elasticat"
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

    # Download archive
    local tmp_dir=$(mktemp -d)
    local archive_file="${tmp_dir}/elasticat.tar.gz"
    if ! curl -fsSL "$url" -o "$archive_file"; then
        rm -rf "$tmp_dir"
        error "Failed to download archive. Check if the release exists for ${os}/${arch}"
    fi

    # Extract archive
    info "Extracting archive..."
    tar -xzf "$archive_file" -C "$tmp_dir"
    
    # Find the binary in the extracted directory
    local binary_name="${BINARY_NAME}-${os}-${arch}"
    local extracted_dir="${tmp_dir}/${binary_name}"
    local binary_file="${extracted_dir}/${binary_name}"
    
    if [ ! -f "$binary_file" ]; then
        rm -rf "$tmp_dir"
        error "Binary not found in archive"
    fi

    # Make executable and move to install location
    chmod +x "$binary_file"
    
    # Determine if we need sudo
    local needs_sudo=false
    if [ "$install_dir" = "/usr/local/bin" ] && [ ! -w "/usr/local/bin" ]; then
        needs_sudo=true
        info "Requesting sudo access to install..."
    fi
    
    # Install binary
    if [ "$needs_sudo" = true ]; then
        sudo mv "$binary_file" "$install_path"
    else
        mv "$binary_file" "$install_path"
    fi
    
    # Install license files
    local doc_dir=$(find_doc_dir "$install_dir")
    info "Installing license files to: ${doc_dir}"
    
    if [ "$needs_sudo" = true ]; then
        sudo mkdir -p "$doc_dir"
        sudo cp "${extracted_dir}/LICENSE.txt" "${extracted_dir}/NOTICE.txt" "${extracted_dir}/README.md" "$doc_dir/"
    else
        mkdir -p "$doc_dir"
        cp "${extracted_dir}/LICENSE.txt" "${extracted_dir}/NOTICE.txt" "${extracted_dir}/README.md" "$doc_dir/"
    fi

    # Install docker compose assets (stack)
    local data_dir=$(find_data_dir "$os")
    local compose_src="${extracted_dir}/docker"
    local compose_dst="${data_dir}/docker"
    if [ -d "$compose_src" ]; then
        info "Installing docker compose stack to: ${compose_dst}"
        mkdir -p "$data_dir"
        rm -rf "$compose_dst"
        cp -R "$compose_src" "$compose_dst"
    else
        warn "docker compose stack not found in archive (missing ./docker). 'elasticat up' may fail unless you provide --dir."
    fi
    
    # Clean up
    rm -rf "$tmp_dir"

    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        success "ElastiCat installed successfully!"
        echo ""
        echo "  Run 'elasticat --help' to get started"
        echo ""
        echo "  License: Apache 2.0"
        echo "  License files installed to: ${doc_dir}"
        if [ -d "$compose_dst" ]; then
            echo "  Docker stack installed to: ${compose_dst}"
        fi
        echo ""
    else
        success "Binary installed to ${install_path}"
        success "License files installed to ${doc_dir}"
        if [ -d "$compose_dst" ]; then
            success "Docker stack installed to ${compose_dst}"
        fi
        check_path "$install_dir"
    fi
}

main "$@"



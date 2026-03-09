#!/bin/sh
set -eu

REPO="justinjdev/glamdring"
SHIRE_REPO="justinjdev/shire"
INSTALL_DIR="${GLAMDRING_INSTALL_DIR:-$HOME/.local/bin}"
BINARY="glamdring"

main() {
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) fatal "Unsupported architecture: $arch" ;;
    esac

    case "$os" in
        linux) os="linux" ;;
        darwin) os="darwin" ;;
        *) fatal "Unsupported OS: $os" ;;
    esac

    if [ -n "${GLAMDRING_VERSION:-}" ]; then
        version="$GLAMDRING_VERSION"
    else
        version="$(get_latest_version "$REPO")"
    fi

    printf "Installing glamdring %s (%s/%s)...\n" "$version" "$os" "$arch"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    install_binary "$REPO" "$BINARY" "$version" "$os" "$arch" "$tmpdir"

    printf "Installed glamdring to %s/%s\n" "$INSTALL_DIR" "$BINARY"

    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        printf "\nAdd %s to your PATH:\n" "$INSTALL_DIR"
        printf "  export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
    fi

    install_shire "$os" "$arch" "$tmpdir"
}

install_binary() {
    repo="$1"
    binary="$2"
    version="$3"
    os="$4"
    arch="$5"
    tmpdir="$6"

    tarball="${binary}_${version#v}_${os}_${arch}.tar.gz"
    url="https://github.com/${repo}/releases/download/${version}/${tarball}"
    extractdir="$tmpdir/$binary"

    printf "Downloading %s\n" "$url"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$tmpdir/$tarball"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$tmpdir/$tarball"
    else
        fatal "Neither curl nor wget found. Install one and try again."
    fi

    mkdir -p "$extractdir"
    tar -xzf "$tmpdir/$tarball" -C "$extractdir"

    mkdir -p "$INSTALL_DIR"
    mv "$extractdir/$binary" "$INSTALL_DIR/$binary"
    chmod +x "$INSTALL_DIR/$binary"
}

install_shire() {
    os="$1"
    arch="$2"
    tmpdir="$3"

    if command -v shire >/dev/null 2>&1; then
        printf "shire already installed, skipping.\n"
        return
    fi

    printf "Installing shire (code indexer)...\n"
    version="$(get_latest_version "$SHIRE_REPO")" || {
        printf "warning: could not determine latest shire version, skipping.\n"
        return
    }

    install_binary "$SHIRE_REPO" "shire" "$version" "$os" "$arch" "$tmpdir" || {
        printf "warning: shire install failed. Install manually: https://github.com/justinjdev/shire/releases\n"
        return
    }

    printf "Installed shire to %s/shire\n" "$INSTALL_DIR"
}

get_latest_version() {
    url="https://api.github.com/repos/${1}/releases/latest"
    if command -v curl >/dev/null 2>&1; then
        ver="$(curl -fsSL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')"
    elif command -v wget >/dev/null 2>&1; then
        ver="$(wget -qO- "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')"
    else
        fatal "Neither curl nor wget found."
    fi
    [ -z "$ver" ] && return 1
    printf '%s\n' "$ver"
}

fatal() {
    printf "Error: %s\n" "$1" >&2
    exit 1
}

main

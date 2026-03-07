#!/bin/sh
set -eu

REPO="justinjdev/glamdring"
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
        version="$(get_latest_version)"
    fi

    printf "Installing glamdring %s (%s/%s)...\n" "$version" "$os" "$arch"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    tarball="${BINARY}_${version#v}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${version}/${tarball}"

    printf "Downloading %s\n" "$url"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$tmpdir/$tarball"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$tmpdir/$tarball"
    else
        fatal "Neither curl nor wget found. Install one and try again."
    fi

    tar -xzf "$tmpdir/$tarball" -C "$tmpdir"

    mkdir -p "$INSTALL_DIR"
    mv "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"
    chmod +x "$INSTALL_DIR/$BINARY"

    printf "Installed glamdring to %s/%s\n" "$INSTALL_DIR" "$BINARY"

    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        printf "\nAdd %s to your PATH:\n" "$INSTALL_DIR"
        printf "  export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
    fi
}

get_latest_version() {
    url="https://api.github.com/repos/${REPO}/releases/latest"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'
    else
        fatal "Neither curl nor wget found."
    fi
}

fatal() {
    printf "Error: %s\n" "$1" >&2
    exit 1
}

main

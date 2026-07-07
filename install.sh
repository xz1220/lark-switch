#!/bin/sh
# lark-switch installer — downloads a prebuilt binary from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/xz1220/lark-switch/main/install.sh | sh
#
# Env overrides:
#   LARK_SWITCH_VERSION   tag to install (default: latest release)
#   LARK_SWITCH_BIN_DIR   install dir     (default: ~/.local/bin)
set -eu

REPO="xz1220/lark-switch"
BIN_DIR="${LARK_SWITCH_BIN_DIR:-$HOME/.local/bin}"

err() { echo "install: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# --- detect platform ---------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *) err "unsupported OS: $os (build from source: https://github.com/$REPO)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) err "unsupported arch: $arch" ;;
esac

# --- pick a downloader -------------------------------------------------------
if have curl; then
  dl() { curl -fsSL "$1"; }
  dlo() { curl -fsSL "$1" -o "$2"; }
elif have wget; then
  dl() { wget -qO- "$1"; }
  dlo() { wget -qO "$2" "$1"; }
else
  err "need curl or wget"
fi

# --- resolve version ---------------------------------------------------------
version="${LARK_SWITCH_VERSION:-}"
if [ -z "$version" ]; then
  version="$(dl "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d '"' -f 4)"
  [ -n "$version" ] || err "could not resolve latest version — set LARK_SWITCH_VERSION"
fi

asset="lark-switch_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/${version}/${asset}"

# --- download & install ------------------------------------------------------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
echo "install: fetching ${asset}"
dlo "$url" "$tmp/pkg.tar.gz" || err "download failed: $url"
tar -xzf "$tmp/pkg.tar.gz" -C "$tmp" || err "extract failed"

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/lark-switch" "$BIN_DIR/lark-switch" 2>/dev/null \
  || { cp "$tmp/lark-switch" "$BIN_DIR/lark-switch" && chmod 0755 "$BIN_DIR/lark-switch"; }

echo "install: lark-switch ${version} -> ${BIN_DIR}/lark-switch"

# --- post-install hints ------------------------------------------------------
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "install: add $BIN_DIR to your PATH:"
     echo "         export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

echo "install: for the per-shell 'use' command, add to ~/.zshrc or ~/.bashrc:"
echo "         eval \"\$(lark-switch shellenv)\""
echo "install: done — try: lark-switch ls"

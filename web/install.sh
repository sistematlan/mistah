#!/bin/sh
# mistah installer — single script that detects the user's macOS arch,
# downloads the matching release tarball, and installs the binary into
# /usr/local/bin (or $HOME/.local/bin if the former is not writable).
#
# Usage:
#   curl -fsSL https://mistah.sistematlan.com/install.sh | sh
#
# Optional environment variables:
#   MISTAH_VERSION   pin to a specific release (default: latest)
#   MISTAH_PREFIX    install location override (default: /usr/local/bin)
#
# Design notes:
#   - POSIX sh, no bashisms. Runs on macOS default shell.
#   - Fails loudly: every step exits on error (set -e + error()).
#   - Writes only to PREFIX. Never touches the rest of the system.
#   - No telemetry. No analytics. No phone-home. Read this file before running.

set -e

# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------

REPO="sistematlan/mistah"
DEFAULT_PREFIX="/usr/local/bin"

color() {
  # color CODE TEXT — only emits ANSI if stdout is a TTY.
  if [ -t 1 ]; then
    printf "\033[%sm%s\033[0m" "$1" "$2"
  else
    printf "%s" "$2"
  fi
}

info()  { printf "%s %s\n" "$(color "1;34" "==>")" "$1"; }
ok()    { printf "%s %s\n" "$(color "1;32" "✓")" "$1"; }
warn()  { printf "%s %s\n" "$(color "1;33" "!")" "$1" >&2; }
error() { printf "%s %s\n" "$(color "1;31" "✗")" "$1" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || error "missing required tool: $1"
}

# ----------------------------------------------------------------------------
# Pre-flight checks
# ----------------------------------------------------------------------------

require curl
require uname
require tar
require mkdir
require install

OS="$(uname -s)"
case "$OS" in
  Darwin) ;;
  *) error "mistah currently only supports macOS. Detected: $OS" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) error "unsupported architecture: $ARCH_RAW" ;;
esac

# ----------------------------------------------------------------------------
# Resolve target version
# ----------------------------------------------------------------------------

VERSION="${MISTAH_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "Resolving latest release..."
  # GitHub redirects /releases/latest to the actual tag URL. Read the
  # Location header to extract the version without parsing JSON.
  VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest" \
    | sed -E 's|.*/tag/(v[^/]+).*|\1|')"
  [ -n "$VERSION" ] || error "could not resolve latest version"
fi

# Strip leading 'v' so it matches the goreleaser archive naming.
VERSION_NO_V="${VERSION#v}"

ARCHIVE="mistah_${VERSION_NO_V}_darwin_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

# ----------------------------------------------------------------------------
# Resolve install prefix
# ----------------------------------------------------------------------------

PREFIX="${MISTAH_PREFIX:-$DEFAULT_PREFIX}"
SUDO=""

if [ ! -d "$PREFIX" ]; then
  warn "$PREFIX does not exist; will create"
  if ! mkdir -p "$PREFIX" 2>/dev/null; then
    SUDO="sudo"
    info "elevating to create $PREFIX"
    sudo mkdir -p "$PREFIX"
  fi
fi

if [ ! -w "$PREFIX" ]; then
  if [ "$PREFIX" = "$DEFAULT_PREFIX" ]; then
    SUDO="sudo"
    info "$PREFIX requires sudo; will prompt for password"
  else
    error "$PREFIX is not writable"
  fi
fi

# ----------------------------------------------------------------------------
# Download and install
# ----------------------------------------------------------------------------

TMP="$(mktemp -d -t mistah.XXXXXX)"
trap 'rm -rf "$TMP"' EXIT INT TERM

info "Downloading mistah ${VERSION} (${ARCH})..."
if ! curl -fsSL -o "$TMP/$ARCHIVE" "$URL"; then
  error "download failed: $URL"
fi

info "Verifying archive..."
tar -tzf "$TMP/$ARCHIVE" >/dev/null 2>&1 || error "archive is corrupt or empty"

info "Extracting..."
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

[ -f "$TMP/mistah" ] || error "binary 'mistah' not found in archive"

info "Installing to $PREFIX/mistah..."
# Use 'install' for atomic replacement (writes to a temp file first).
$SUDO install -m 0755 "$TMP/mistah" "$PREFIX/mistah"

# ----------------------------------------------------------------------------
# Post-install
# ----------------------------------------------------------------------------

# Strip the quarantine xattr that macOS adds to anything from the network,
# so the binary runs without the Gatekeeper "is from the internet" prompt.
# This is a one-time fix per install; it does NOT bypass code signing
# verification — only Gatekeeper's quarantine flag.
xattr -d com.apple.quarantine "$PREFIX/mistah" 2>/dev/null || true

ok "mistah ${VERSION} installed at $PREFIX/mistah"

# Report whether PREFIX is in PATH so the user knows what to do next.
case ":$PATH:" in
  *":$PREFIX:"*)
    printf "\n  Try it now:\n    %s\n\n" "$(color 1 "mistah")"
    ;;
  *)
    warn "$PREFIX is not in your PATH"
    printf "  Add this line to ~/.zshrc or ~/.bashrc:\n"
    printf "    %s\n\n" "$(color 1 "export PATH=\"$PREFIX:\$PATH\"")"
    ;;
esac

printf "  Docs:    https://mistah.sistematlan.com\n"
printf "  Issues:  https://github.com/${REPO}/issues\n"

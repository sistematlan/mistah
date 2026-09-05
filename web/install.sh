#!/bin/sh
# mistah installer — single script that detects the user's OS/arch,
# downloads the matching release tarball, and installs the binary into
# /usr/local/bin (or $HOME/.local/bin if the former is not writable).
#
# Supports macOS (arm64, amd64) and Linux (amd64). Windows users
# download the .zip from GitHub Releases directly — see
# mistah.sistematlan.com and README.md.
#
# Usage:
#   curl -fsSL https://mistah.sistematlan.com/install.sh | sh
#
# Optional environment variables:
#   MISTAH_VERSION   pin to a specific release (default: latest)
#   MISTAH_PREFIX    install location override (default: /usr/local/bin)
#   MISTAH_YES       skip the confirmation prompt (for CI/non-interactive use)
#
# Design notes:
#   - POSIX sh, no bashisms. Runs on macOS default shell and any Linux /bin/sh.
#   - Fails loudly: every step exits on error (set -e + error()).
#   - Writes only to PREFIX. Never touches the rest of the system.
#   - No telemetry. No analytics. No phone-home. Read this file before running.
#   - Explains exactly what it's about to do, then PAUSES for confirmation
#     before touching disk or requesting sudo — same pattern Homebrew's
#     installer uses. When run via `curl | sh`, stdin is occupied by the
#     script itself, so the confirmation prompt reads from /dev/tty
#     directly rather than $0's stdin (see confirm() below).

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

# confirm PROMPT — asks the user to press enter/y before proceeding.
#
# Reads from /dev/tty instead of stdin because this script is almost
# always invoked as `curl ... | sh`, where stdin is the pipe carrying
# the script's own source — there is no keyboard input to read there.
# /dev/tty bypasses the pipe and talks to the actual terminal, which is
# exactly how Homebrew's installer supports "explain then pause" under
# the same curl|sh invocation.
#
# If there is no controlling terminal at all (CI runners, some Docker
# build contexts, MISTAH_YES=1), there is nothing to prompt — proceed
# automatically rather than hanging forever waiting for input that can
# never arrive.
#
# Detecting "no controlling terminal" needs an actual open attempt, not
# just `test -r /dev/tty`: the device node can exist and pass a
# permission check while still having no controlling session behind it
# (true in most container/CI sandboxes), which raises "Device not
# configured" only once something tries to actually read from it.
confirm() {
  if [ -n "$MISTAH_YES" ]; then
    return 0
  fi
  if ! REPLY="$( (exec < /dev/tty && printf "%s [y/N] " "$1" >/dev/tty && read -r line && printf "%s" "$line") 2>/dev/null )"; then
    warn "no interactive terminal detected; proceeding without confirmation (set MISTAH_YES=1 to silence this)"
    return 0
  fi
  case "$REPLY" in
    y|Y|yes|YES) return 0 ;;
    *) error "aborted by user" ;;
  esac
}

# ----------------------------------------------------------------------------
# Pre-flight checks
# ----------------------------------------------------------------------------

require curl
require uname
require tar
require mkdir
require install

OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *) error "mistah supports macOS and Linux. Detected: $OS_RAW (Windows users: download the .zip from https://github.com/${REPO}/releases/latest)" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) error "unsupported architecture: $ARCH_RAW" ;;
esac

# Linux arm64 isn't published yet (see .goreleaser.yaml) — fail with a
# clear message instead of a confusing 404 further down.
if [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
  error "linux/arm64 is not built yet. Open an issue at https://github.com/${REPO}/issues if you need it."
fi

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

ARCHIVE="mistah_${VERSION_NO_V}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

# ----------------------------------------------------------------------------
# Resolve install prefix
# ----------------------------------------------------------------------------

PREFIX="${MISTAH_PREFIX:-$DEFAULT_PREFIX}"
NEEDS_SUDO=0
SUDO=""

if [ ! -d "$PREFIX" ]; then
  if ! mkdir -p "$PREFIX" 2>/dev/null; then
    NEEDS_SUDO=1
  else
    rmdir "$PREFIX" 2>/dev/null || true  # undo the probe; the real mkdir happens after confirmation
  fi
elif [ ! -w "$PREFIX" ]; then
  if [ "$PREFIX" = "$DEFAULT_PREFIX" ]; then
    NEEDS_SUDO=1
  else
    error "$PREFIX is not writable"
  fi
fi

# ----------------------------------------------------------------------------
# Explain the plan, then pause for confirmation — nothing above this
# point touched disk or ran sudo. Everything below this point does.
# ----------------------------------------------------------------------------

printf "\n"
info "mistah will:"
printf "    1. Download %s\n" "$(color 1 "$ARCHIVE")"
printf "       from %s\n" "$URL"
printf "    2. Verify the archive is a valid, non-empty tarball\n"
if [ "$NEEDS_SUDO" -eq 1 ]; then
  printf "    3. Install the %s binary to %s %s\n" \
    "$(color 1 "mistah")" "$(color 1 "$PREFIX/mistah")" "$(color "1;33" "(requires sudo)")"
else
  printf "    3. Install the %s binary to %s\n" "$(color 1 "mistah")" "$(color 1 "$PREFIX/mistah")"
fi
printf "\n"
printf "  No other files are touched. No telemetry is sent. Read this\n"
printf "  script yourself first if you'd rather not take our word for it:\n"
printf "    %s\n\n" "$(color 1 "https://mistah.sistematlan.com/install.sh")"

confirm "Proceed with installation?"

if [ "$NEEDS_SUDO" -eq 1 ]; then
  SUDO="sudo"
  info "requesting sudo to write to $PREFIX"
fi

mkdir -p "$PREFIX" 2>/dev/null || $SUDO mkdir -p "$PREFIX"

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
# verification — only Gatekeeper's quarantine flag. Linux has no
# equivalent mechanism, so this step is a no-op there.
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$PREFIX/mistah" 2>/dev/null || true
fi

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

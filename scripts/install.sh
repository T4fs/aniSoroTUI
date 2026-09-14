#!/usr/bin/env bash
#
# aniSoroTUI self-contained installer for a brand-new Linux/macOS machine.
# - No package manager, no git, no C compiler, no existing Go required.
# - Downloads a standalone Go toolchain, fetches the source as a tarball,
#   builds a static binary, and adds it to your PATH.
#
# Usage (copy-paste into a terminal):
#   curl -fsSL https://raw.githubusercontent.com/T4fs/aniSoroTUI/main/scripts/install.sh | bash
#
set -euo pipefail

RED=$'\033[31m'; GRN=$'\033[32m'; YLW=$'\033[33m'; CYAN=$'\033[36m'; BOLD=$'\033[1m'; RST=$'\033[0m'
say()  { printf '%b\n' "${GRN}==>${RST} $*"; }
warn() { printf '%b\n' "${YLW}warn:${RST} $*"; }
die()  { printf '%b\n' "${RED}error:${RST} $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 1. detect platform
# ---------------------------------------------------------------------------
UNAME_S=$(uname -s)
UNAME_M=$(uname -m)
case "$UNAME_S" in
  Linux)  GOOS=linux ;;
  Darwin) GOOS=darwin ;;
  *) die "unsupported OS: $UNAME_S (aniSoroTUI supports Linux and macOS)" ;;
esac
case "$UNAME_M" in
  x86_64|amd64)  GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) die "unsupported CPU: $UNAME_M (supported: amd64, arm64)" ;;
esac
say "Platform: $GOOS/$GOARCH"

# need something to download with (curl or wget)
if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
  die "need curl or wget to download things."
fi

fetch() { # fetch <url> <outfile>
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  else
    wget -qO "$2" "$1"
  fi
}

# ---------------------------------------------------------------------------
# 2. make sure Go is available (download a standalone toolchain if not)
# ---------------------------------------------------------------------------
HOOK=$(command -v go || true)
if [ -z "$HOOK" ]; then
  say "Go not found — downloading a standalone Go toolchain (no admin needed)..."
  GO_VER=$(curl -fsSL https://go.dev/VERSION?m=text 2>/dev/null | head -n1 || wget -qO- https://go.dev/VERSION?m=text 2>/dev/null | head -n1)
  [ -n "$GO_VER" ] || die "could not determine the latest Go version — is a download tool installed?"
  GO_BASE=${GO_VER#go}
  GO_PARENT="$HOME/.local"
  GO_ROOT="$GO_PARENT/go"
  ANITUI_HOME="$HOME/.anitui"
  mkdir -p "$ANITUI_HOME" "$GO_PARENT"

  say "Downloading Go $GO_BASE ($GOOS/$GOARCH)..."
  fetch "https://go.dev/dl/${GO_VER}.${GOOS}-${GOARCH}.tar.gz" "$ANITUI_HOME/go.tgz"
  say "Extracting Go to $GO_ROOT..."
  rm -rf "$GO_ROOT"
  tar -C "$GO_PARENT" -xzf "$ANITUI_HOME/go.tgz"
  rm -f "$ANITUI_HOME/go.tgz"
  export PATH="$GO_ROOT/bin:$PATH"
  say "Go $GO_BASE ready"
else
  say "using existing Go: $(go version)"
fi

command -v go >/dev/null 2>&1 || die "Go is still unavailable after setup."

# ---------------------------------------------------------------------------
# 3. fetch the aniSoroTUI source (tarball — no git needed)
# ---------------------------------------------------------------------------
ANITUI_HOME="${ANITUI_HOME:-$HOME/.anitui}"
mkdir -p "$ANITUI_HOME"
say "Downloading the aniSoroTUI source..."
fetch "https://codeload.github.com/T4fs/aniSoroTUI/tar.gz/refs/heads/main" "$ANITUI_HOME/aniSoro.tgz"
SRC_DIR="$ANITUI_HOME/src"
rm -rf "$SRC_DIR"; mkdir -p "$SRC_DIR"
tar -C "$SRC_DIR" -xzf "$ANITUI_HOME/aniSoro.tgz"
rm -f "$ANITUI_HOME/aniSoro.tgz"
PROJ=$(find "$SRC_DIR" -maxdepth 1 -type d -iname '*aniSoroTUI*' | head -n1)
[ -n "$PROJ" ] || die "could not locate the source after extraction."
cd "$PROJ/aniSoro"

# ---------------------------------------------------------------------------
# 4. build and install
# ---------------------------------------------------------------------------
BIN_DIR="$ANITUI_HOME/bin"
mkdir -p "$BIN_DIR"
say "Building aniSoroTUI (this can take a minute)..."
CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags="-s -w" -o "$BIN_DIR/aniSoro" ./cmd/anitui
say "Built $BIN_DIR/aniSoro"

# ---------------------------------------------------------------------------
# 5. add to PATH
# ---------------------------------------------------------------------------
BIN_LINE="export PATH=\"$BIN_DIR:\$PATH\""
PROFILE=""
[ -f "$HOME/.zshrc" ] && PROFILE="$HOME/.zshrc"
[ -z "$PROFILE" ] && [ -f "$HOME/.bashrc" ] && PROFILE="$HOME/.bashrc"
if [ -n "$PROFILE" ]; then
  if ! grep -qF "$BIN_DIR" "$PROFILE" 2>/dev/null; then
    printf '\n# added by the aniSoroTUI installer\n%s\n' "$BIN_LINE" >> "$PROFILE"
    say "Added $BIN_DIR to your PATH in $PROFILE"
  fi
fi
export PATH="$BIN_DIR:$PATH"

printf '%b\n' "${GRN}"
printf '%s\n' "──────────────────────────────────────────────────"
printf '%s\n' " aniSoroTUI installed successfully."
printf '%s\n' "──────────────────────────────────────────────────"
printf '%b\n' "${RST}"
printf '   Run it now, or open a new terminal and run:\n\n'
printf '     %b%s%b\n\n' "${BOLD}${CYAN}" "aniSoro" "${RST}"

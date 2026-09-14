#!/bin/sh
set -e

REPO="typechecks/anitui"
INSTALL_DIR="/usr/local/bin"

while [ $# -gt 0 ]; do
	case "$1" in
		--dir) INSTALL_DIR="$2"; shift 2 ;;
		*) echo "Unknown option: $1"; exit 1 ;;
	esac
done

if [ ! -d "$INSTALL_DIR" ]; then
	echo "Creating $INSTALL_DIR"
	mkdir -p "$INSTALL_DIR"
fi

if [ ! -w "$INSTALL_DIR" ] && [ "$(id -u)" -ne 0 ]; then
	echo "Cannot write to $INSTALL_DIR. Run with sudo or use --dir."
	exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
	x86_64) ARCH="amd64" ;;
	aarch64|arm64) ARCH="arm64" ;;
	*) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Detecting latest version..."
LATEST_TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
	echo "Could not find latest release tag"
	exit 1
fi

BINARY_NAME="anitui_${OS}_${ARCH}"
[ "$OS" = "darwin" ] && BINARY_NAME="anitui_darwin_${ARCH}"

URL="https://github.com/$REPO/releases/download/$LATEST_TAG/${BINARY_NAME}.tar.gz"

echo "Downloading anitui $LATEST_TAG for $OS/$ARCH..."
curl -fsSL "$URL" -o /tmp/anitui.tar.gz

echo "Installing to $INSTALL_DIR..."
tar -xzf /tmp/anitui.tar.gz -C /tmp

if [ -f "$INSTALL_DIR/anitui" ]; then
	mv "$INSTALL_DIR/anitui" "$INSTALL_DIR/anitui.old"
fi

mv /tmp/anitui "$INSTALL_DIR/anitui"
chmod +x "$INSTALL_DIR/anitui"

rm -f /tmp/anitui.tar.gz "$INSTALL_DIR/anitui.old"
echo "Done! Run 'anitui' to start."

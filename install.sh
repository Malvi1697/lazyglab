#!/bin/sh
set -e

REPO="Malvi1697/lazyglab"
BINARY="lazyglab"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version"
    exit 1
fi

echo "Installing ${BINARY} v${VERSION} (${OS}/${ARCH})..."

# Download and extract
TARBALL="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${TARBALL}"

TMPDIR=$(mktemp -d)
chmod 700 "$TMPDIR"
trap 'rm -rf "$TMPDIR"' EXIT

curl -sL "$URL" -o "${TMPDIR}/${TARBALL}"

# Verify checksum (mandatory)
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"
if ! curl -sL --fail "$CHECKSUMS_URL" -o "${TMPDIR}/checksums.txt" 2>/dev/null; then
    echo "Error: failed to download checksums file"
    exit 1
fi

EXPECTED=$(grep "${TARBALL}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
    echo "Error: no checksum found for ${TARBALL}"
    exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "${TMPDIR}/${TARBALL}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "${TMPDIR}/${TARBALL}" | awk '{print $1}')
else
    echo "Error: no sha256sum or shasum found, cannot verify checksum"
    exit 1
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
    echo "Error: checksum verification failed"
    echo "  Expected: $EXPECTED"
    echo "  Got:      $ACTUAL"
    exit 1
fi
echo "Checksum verified."

tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
    echo "Installing to ${INSTALL_DIR} requires elevated permissions."
    sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

chmod +x "${INSTALL_DIR}/${BINARY}"

echo "${BINARY} v${VERSION} installed to ${INSTALL_DIR}/${BINARY}"

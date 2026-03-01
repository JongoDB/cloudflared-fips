#!/bin/bash
# Build a Debian .deb package for cloudflared-fips.
# Usage: ./build-deb.sh <version> <arch> <binary-dir> <output-dir>
#
# arch: amd64 or arm64
# Requires: dpkg-deb

set -euo pipefail

VERSION="${1:?Usage: build-deb.sh <version> <arch> <binary-dir> <output-dir>}"
ARCH="${2:?Usage: build-deb.sh <version> <arch> <binary-dir> <output-dir>}"
BINARY_DIR="${3:?Usage: build-deb.sh <version> <arch> <binary-dir> <output-dir>}"
OUTPUT_DIR="${4:?Usage: build-deb.sh <version> <arch> <binary-dir> <output-dir>}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PKG_DIR="$(mktemp -d)"

cleanup() { rm -rf "$PKG_DIR"; }
trap cleanup EXIT

echo "=== Building DEB package for cloudflared-fips ${VERSION} (${ARCH}) ==="

# Create directory structure
mkdir -p "$PKG_DIR/DEBIAN"
mkdir -p "$PKG_DIR/usr/local/bin"
mkdir -p "$PKG_DIR/etc/cloudflared"
mkdir -p "$PKG_DIR/usr/share/cloudflared-fips"

# Copy DEBIAN control files with variable substitution
sed -e "s/\${VERSION}/${VERSION}/" \
    -e "s/\${ARCH}/${ARCH}/" \
    "$SCRIPT_DIR/DEBIAN/control" > "$PKG_DIR/DEBIAN/control"

cp "$SCRIPT_DIR/DEBIAN/postinst" "$PKG_DIR/DEBIAN/postinst"
cp "$SCRIPT_DIR/DEBIAN/prerm" "$PKG_DIR/DEBIAN/prerm"
chmod 755 "$PKG_DIR/DEBIAN/postinst" "$PKG_DIR/DEBIAN/prerm"

# Install fleet binaries
for bin in cloudflared-fips-selftest cloudflared-fips-dashboard cloudflared-fips-tui \
           cloudflared-fips-proxy cloudflared-fips-agent; do
    if [ -f "$BINARY_DIR/$bin" ]; then
        cp "$BINARY_DIR/$bin" "$PKG_DIR/usr/local/bin/$bin"
        chmod 755 "$PKG_DIR/usr/local/bin/$bin"
    else
        echo "WARNING: $bin not found in $BINARY_DIR"
    fi
done

# Install provision script
if [ -f "$BINARY_DIR/cloudflared-fips-provision" ]; then
    cp "$BINARY_DIR/cloudflared-fips-provision" "$PKG_DIR/usr/local/bin/cloudflared-fips-provision"
    chmod 755 "$PKG_DIR/usr/local/bin/cloudflared-fips-provision"
fi

# Install build manifest
if [ -f "$BINARY_DIR/build-manifest.json" ]; then
    cp "$BINARY_DIR/build-manifest.json" "$PKG_DIR/usr/share/cloudflared-fips/"
fi

# Install sample config
if [ -f "$BINARY_DIR/cloudflared-fips.yaml" ]; then
    cp "$BINARY_DIR/cloudflared-fips.yaml" "$PKG_DIR/etc/cloudflared/config.yaml.sample"
fi

# Build the package
mkdir -p "$OUTPUT_DIR"
dpkg-deb --build "$PKG_DIR" "$OUTPUT_DIR/cloudflared-fips_${VERSION}_${ARCH}.deb"

echo "=== Package built: $OUTPUT_DIR/cloudflared-fips_${VERSION}_${ARCH}.deb ==="

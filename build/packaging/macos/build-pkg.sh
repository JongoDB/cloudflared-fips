#!/bin/bash
# Build a macOS .pkg installer for cloudflared-fips.
# Usage: ./build-pkg.sh <version> <binary-dir> <output-dir>
#
# Requires: pkgbuild, productbuild (included in Xcode CLT)
# For code signing: Apple Developer ID Installer certificate

set -euo pipefail

VERSION="${1:?Usage: build-pkg.sh <version> <binary-dir> <output-dir>}"
BINARY_DIR="${2:?Usage: build-pkg.sh <version> <binary-dir> <output-dir>}"
OUTPUT_DIR="${3:?Usage: build-pkg.sh <version> <binary-dir> <output-dir>}"

PKG_ID="com.cloudflared-fips.cloudflared"
INSTALL_LOCATION="/usr/local"
STAGING_DIR="$(mktemp -d)"
SCRIPTS_DIR="$(mktemp -d)"

cleanup() { rm -rf "$STAGING_DIR" "$SCRIPTS_DIR"; }
trap cleanup EXIT

echo "=== Building macOS .pkg for cloudflared-fips ${VERSION} ==="

# Stage files
mkdir -p "$STAGING_DIR/bin"
mkdir -p "$STAGING_DIR/share/cloudflared-fips"
mkdir -p "$STAGING_DIR/etc/cloudflared"

cp "$BINARY_DIR/cloudflared" "$STAGING_DIR/bin/cloudflared"
chmod 755 "$STAGING_DIR/bin/cloudflared"

if [ -f "$BINARY_DIR/selftest" ]; then
    cp "$BINARY_DIR/selftest" "$STAGING_DIR/bin/cloudflared-selftest"
    chmod 755 "$STAGING_DIR/bin/cloudflared-selftest"
fi

if [ -f "$BINARY_DIR/build-manifest.json" ]; then
    cp "$BINARY_DIR/build-manifest.json" "$STAGING_DIR/share/cloudflared-fips/"
fi

if [ -f "$BINARY_DIR/cloudflared-fips.yaml" ]; then
    cp "$BINARY_DIR/cloudflared-fips.yaml" "$STAGING_DIR/etc/cloudflared/config.yaml.sample"
fi

# Post-install script (runs FIPS self-test)
cat > "$SCRIPTS_DIR/postinstall" <<'POSTINST'
#!/bin/bash
echo "Running FIPS self-test..."
/usr/local/bin/cloudflared-selftest 2>&1 || {
    echo "WARNING: FIPS self-test failed."
    echo "The binary may not be FIPS-compliant on this system."
}
echo "cloudflared-fips installed to /usr/local/bin/cloudflared"
POSTINST
chmod 755 "$SCRIPTS_DIR/postinstall"

# Build component package
mkdir -p "$OUTPUT_DIR"

pkgbuild \
    --root "$STAGING_DIR" \
    --identifier "$PKG_ID" \
    --version "$VERSION" \
    --install-location "$INSTALL_LOCATION" \
    --scripts "$SCRIPTS_DIR" \
    "$OUTPUT_DIR/cloudflared-fips-${VERSION}-component.pkg"

# Build product archive (distribution package)
cat > "$STAGING_DIR/Distribution.xml" <<DIST
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
    <title>cloudflared-fips ${VERSION}</title>
    <welcome file="welcome.html" />
    <license file="license.txt" />
    <options customize="never" require-scripts="false" />
    <choices-outline>
        <line choice="default">
            <line choice="cloudflared-fips" />
        </line>
    </choices-outline>
    <choice id="default" />
    <choice id="cloudflared-fips"
            visible="false"
            title="cloudflared-fips"
            description="FIPS 140-2 compliant Cloudflare Tunnel">
        <pkg-ref id="${PKG_ID}" />
    </choice>
    <pkg-ref id="${PKG_ID}"
             version="${VERSION}"
             onConclusion="none">cloudflared-fips-${VERSION}-component.pkg</pkg-ref>
</installer-gui-script>
DIST

productbuild \
    --distribution "$STAGING_DIR/Distribution.xml" \
    --package-path "$OUTPUT_DIR" \
    "$OUTPUT_DIR/cloudflared-fips-${VERSION}.pkg"

# Clean up component package
rm -f "$OUTPUT_DIR/cloudflared-fips-${VERSION}-component.pkg"

echo "=== Package built: $OUTPUT_DIR/cloudflared-fips-${VERSION}.pkg ==="

# Code signing (optional — requires Apple Developer ID)
if [ -n "${SIGNING_IDENTITY:-}" ]; then
    echo "Signing with identity: $SIGNING_IDENTITY"
    productsign \
        --sign "$SIGNING_IDENTITY" \
        "$OUTPUT_DIR/cloudflared-fips-${VERSION}.pkg" \
        "$OUTPUT_DIR/cloudflared-fips-${VERSION}-signed.pkg"
    mv "$OUTPUT_DIR/cloudflared-fips-${VERSION}-signed.pkg" \
       "$OUTPUT_DIR/cloudflared-fips-${VERSION}.pkg"
    echo "Package signed."
fi

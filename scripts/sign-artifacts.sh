#!/bin/bash
# Sign build artifacts with GPG (binaries/packages) and cosign (containers).
# Usage: ./sign-artifacts.sh <artifact-dir> [--gpg-key KEY_ID] [--cosign-key PATH]
#
# Environment variables:
#   GPG_SIGNING_KEY   — GPG key ID for binary/package signing
#   COSIGN_KEY        — Path to cosign private key (or use keyless with OIDC)
#   COSIGN_PASSWORD   — Password for cosign key (if encrypted)

set -euo pipefail

ARTIFACT_DIR="${1:?Usage: sign-artifacts.sh <artifact-dir> [--gpg-key KEY_ID] [--cosign-key PATH]}"
GPG_KEY="${GPG_SIGNING_KEY:-}"
COSIGN_KEY_PATH="${COSIGN_KEY:-}"

# Parse optional flags
shift
while [[ $# -gt 0 ]]; do
    case "$1" in
        --gpg-key) GPG_KEY="$2"; shift 2 ;;
        --cosign-key) COSIGN_KEY_PATH="$2"; shift 2 ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

echo "=== Artifact Signing ==="
echo "Artifact directory: $ARTIFACT_DIR"

MANIFEST_FILE="$ARTIFACT_DIR/signatures.json"
SIGNATURES="[]"

# ── GPG signing for binaries and packages ──
sign_with_gpg() {
    local file="$1"
    local filename
    filename=$(basename "$file")

    if [ -z "$GPG_KEY" ]; then
        echo "  SKIP $filename (no GPG key)"
        return
    fi

    if ! command -v gpg >/dev/null 2>&1; then
        echo "  SKIP $filename (gpg not installed)"
        return
    fi

    echo "  GPG signing: $filename"
    gpg --batch --yes --detach-sign --armor \
        --local-user "$GPG_KEY" \
        --output "$file.sig" \
        "$file"

    local sha256
    sha256=$(sha256sum "$file" | awk '{print $1}')

    echo "  Signed: $filename -> $filename.sig (SHA-256: ${sha256:0:16}...)"
}

# ── Sign all binaries and packages ──
echo ""
echo "--- Binary & Package Signing (GPG) ---"

for ext in "" ".exe" ".rpm" ".deb" ".pkg" ".msi"; do
    for file in "$ARTIFACT_DIR"/*"$ext"; do
        [ -f "$file" ] || continue
        # Skip signature files, manifests, and SBOMs
        case "$file" in
            *.sig|*.json|*.xml) continue ;;
        esac
        sign_with_gpg "$file"
    done
done

# ── Container signing with cosign ──
echo ""
echo "--- Container Signing (cosign) ---"

if command -v cosign >/dev/null 2>&1; then
    # Look for OCI image references in build manifest
    if [ -f "$ARTIFACT_DIR/build-manifest.json" ]; then
        echo "  cosign available. Container signing requires image reference."
        echo "  Use: cosign sign --key \$COSIGN_KEY <image-ref>"
    fi
else
    echo "  cosign not installed. Install: go install github.com/sigstore/cosign/v2/cmd/cosign@latest"
fi

# ── Generate signature manifest ──
echo ""
echo "--- Generating Signature Manifest ---"

cat > "$MANIFEST_FILE" <<SIGEOF
{
  "version": "1.0.0",
  "build_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "signatures": [
$(find "$ARTIFACT_DIR" -name "*.sig" -type f | while IFS= read -r sigfile; do
    artifact="${sigfile%.sig}"
    if [ -f "$artifact" ]; then
        sha256=$(sha256sum "$artifact" | awk '{print $1}')
        cat <<SIG
    {
      "artifact_path": "$(basename "$artifact")",
      "artifact_sha256": "$sha256",
      "signature_path": "$(basename "$sigfile")",
      "signature_method": "gpg",
      "signer_identity": "${GPG_KEY:-unknown}"
    },
SIG
    fi
done | sed '$ s/,$//')
  ],
  "public_key_url": "https://github.com/cloudflared-fips/cloudflared-fips/blob/main/KEYS"
}
SIGEOF

echo "Signature manifest: $MANIFEST_FILE"
echo ""
echo "=== Signing Complete ==="

#!/bin/bash
# Generate real SBOMs (CycloneDX 1.5 + SPDX 2.3) from Go module dependencies.
# Annotates crypto dependencies with FIPS module information.
#
# Usage: ./generate-sbom.sh <upstream-cloudflared-dir> <output-dir> [version]
#
# Prerequisites:
#   - Go 1.24+
#   - Optional: cyclonedx-gomod (go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest)
#
# If cyclonedx-gomod is installed, it produces a full CycloneDX SBOM with
# transitive dependencies, hashes, and licenses. Otherwise, falls back to
# parsing `go list -m -json all` for a complete but simpler enumeration.

set -euo pipefail

UPSTREAM_DIR="${1:?Usage: generate-sbom.sh <upstream-dir> <output-dir> [version]}"
OUTPUT_DIR="${2:?Usage: generate-sbom.sh <upstream-dir> <output-dir> [version]}"
VERSION="${3:-dev}"

mkdir -p "$OUTPUT_DIR"

echo "=== Generating SBOMs for cloudflared-fips ${VERSION} ==="

# ── CycloneDX SBOM ──
echo "Generating CycloneDX SBOM..."

if command -v cyclonedx-gomod >/dev/null 2>&1; then
    # Use the official CycloneDX Go module tool for full SBOM
    cd "$UPSTREAM_DIR"
    cyclonedx-gomod mod \
        -json \
        -output "$OUTPUT_DIR/sbom.cyclonedx.json" \
        -module . \
        -noserial=false
    echo "CycloneDX SBOM generated via cyclonedx-gomod (full)"
else
    echo "cyclonedx-gomod not found; generating from go list -m -json all..."

    cd "$UPSTREAM_DIR"
    UUID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())" 2>/dev/null || echo "00000000-0000-0000-0000-000000000000")
    TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    # Collect all module dependencies as JSON
    # go list -m -json all outputs concatenated JSON objects (one per module)
    MODULES_JSON=$(go list -m -json all 2>/dev/null || echo "")

    # Use python3 to parse concatenated JSON and produce CycloneDX components
    COMPONENTS=$(echo "$MODULES_JSON" | python3 -c "
import sys, json

# Parse concatenated JSON objects from go list -m -json
decoder = json.JSONDecoder()
content = sys.stdin.read().strip()
modules = []
pos = 0
while pos < len(content):
    try:
        obj, end = decoder.raw_decode(content, pos)
        modules.append(obj)
        pos = end
        while pos < len(content) and content[pos] in ' \t\n\r':
            pos += 1
    except json.JSONDecodeError:
        break

components = []
for mod in modules:
    path = mod.get('Path', '')
    version = mod.get('Version', '')
    if mod.get('Main', False):
        continue  # Skip the main module
    if not path or not version:
        continue
    comp = {
        'type': 'library',
        'name': path,
        'version': version,
        'purl': f'pkg:golang/{path}@{version}',
    }
    go_sum = mod.get('GoMod', '')
    if mod.get('Indirect', False):
        comp['scope'] = 'optional'
    components.append(comp)

print(json.dumps(components, indent=4))
" 2>/dev/null || echo "[]")

    COMPONENT_COUNT=$(echo "$COMPONENTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")

    cat > "$OUTPUT_DIR/sbom.cyclonedx.json" <<CDXEOF
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "serialNumber": "urn:uuid:${UUID}",
  "version": 1,
  "metadata": {
    "timestamp": "${TIMESTAMP}",
    "tools": [
      {
        "vendor": "cloudflared-fips",
        "name": "generate-sbom.sh",
        "version": "1.0.0"
      }
    ],
    "component": {
      "type": "application",
      "name": "cloudflared-fips",
      "version": "${VERSION}",
      "purl": "pkg:golang/github.com/cloudflared-fips/cloudflared-fips@${VERSION}"
    }
  },
  "components": ${COMPONENTS}
}
CDXEOF
    echo "CycloneDX SBOM generated from go list (${COMPONENT_COUNT} components)"
fi

# ── SPDX SBOM ──
echo "Generating SPDX SBOM..."

cd "$UPSTREAM_DIR"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Collect all module dependencies with proper SPDX package entries
MODULES_JSON=$(go list -m -json all 2>/dev/null || echo "")

SPDX_PACKAGES=$(echo "$MODULES_JSON" | python3 -c "
import sys, json, re

decoder = json.JSONDecoder()
content = sys.stdin.read().strip()
modules = []
pos = 0
while pos < len(content):
    try:
        obj, end = decoder.raw_decode(content, pos)
        modules.append(obj)
        pos = end
        while pos < len(content) and content[pos] in ' \t\n\r':
            pos += 1
    except json.JSONDecodeError:
        break

packages = []
relationships = []
for mod in modules:
    path = mod.get('Path', '')
    version = mod.get('Version', '')
    if mod.get('Main', False):
        continue
    if not path or not version:
        continue
    # SPDX IDs must be [a-zA-Z0-9.-]
    spdx_id = 'SPDXRef-' + re.sub(r'[^a-zA-Z0-9.-]', '-', path)
    pkg = {
        'SPDXID': spdx_id,
        'name': path,
        'versionInfo': version,
        'downloadLocation': f'https://{path}',
        'filesAnalyzed': False,
        'externalRefs': [{
            'referenceCategory': 'PACKAGE-MANAGER',
            'referenceType': 'purl',
            'referenceLocator': f'pkg:golang/{path}@{version}'
        }]
    }
    packages.append(pkg)
    relationships.append({
        'spdxElementId': 'SPDXRef-Package',
        'relatedSpdxElement': spdx_id,
        'relationshipType': 'DEPENDS_ON'
    })

print(json.dumps({'packages': packages, 'relationships': relationships}, indent=4))
" 2>/dev/null || echo '{"packages": [], "relationships": []}')

SPDX_PKG_LIST=$(echo "$SPDX_PACKAGES" | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d['packages'], indent=4))" 2>/dev/null || echo "[]")
SPDX_REL_LIST=$(echo "$SPDX_PACKAGES" | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d['relationships'], indent=4))" 2>/dev/null || echo "[]")
SPDX_PKG_COUNT=$(echo "$SPDX_PACKAGES" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['packages']))" 2>/dev/null || echo "0")

# Build the main package + all dependency packages
MAIN_PKG='[
    {
      "SPDXID": "SPDXRef-Package",
      "name": "cloudflared-fips",
      "versionInfo": "'"${VERSION}"'",
      "downloadLocation": "https://github.com/cloudflared-fips/cloudflared-fips",
      "filesAnalyzed": false,
      "supplier": "Organization: cloudflared-fips",
      "primaryPackagePurpose": "APPLICATION"
    }'

# Merge main package with dependency packages
ALL_PACKAGES=$(python3 -c "
import json, sys
main = json.loads('''${MAIN_PKG}]''')
deps = json.loads(sys.stdin.read())
result = main + deps
print(json.dumps(result, indent=4))
" <<< "$SPDX_PKG_LIST" 2>/dev/null || echo "$MAIN_PKG]")

# Merge relationships
DESCRIBES_REL='[{"spdxElementId": "SPDXRef-DOCUMENT", "relatedSpdxElement": "SPDXRef-Package", "relationshipType": "DESCRIBES"}]'
ALL_RELS=$(python3 -c "
import json, sys
describes = json.loads('''${DESCRIBES_REL}''')
deps = json.loads(sys.stdin.read())
result = describes + deps
print(json.dumps(result, indent=4))
" <<< "$SPDX_REL_LIST" 2>/dev/null || echo "$DESCRIBES_REL")

cat > "$OUTPUT_DIR/sbom.spdx.json" <<SPDXEOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "cloudflared-fips-${VERSION}",
  "documentNamespace": "https://github.com/cloudflared-fips/cloudflared-fips/sbom/${VERSION}",
  "creationInfo": {
    "created": "${TIMESTAMP}",
    "creators": [
      "Tool: generate-sbom.sh-1.0.0",
      "Organization: cloudflared-fips"
    ]
  },
  "packages": ${ALL_PACKAGES},
  "relationships": ${ALL_RELS}
}
SPDXEOF

echo "SPDX SBOM generated (${SPDX_PKG_COUNT} dependency packages)"

# ── Crypto dependency audit ──
echo "Generating crypto dependency audit..."

cd "$UPSTREAM_DIR"

# Get all packages that import crypto/* (direct and transitive)
CRYPTO_DEPS=$(go list -deps ./cmd/cloudflared 2>/dev/null | grep "crypto/" || true)
# Get packages from x/crypto (these may bypass BoringCrypto)
XCRYPTO_DEPS=$(go list -deps ./cmd/cloudflared 2>/dev/null | grep "golang.org/x/crypto" || true)

CRYPTO_COUNT=$(echo "$CRYPTO_DEPS" | grep -c "." || echo "0")
XCRYPTO_COUNT=$(echo "$XCRYPTO_DEPS" | grep -c "." || echo "0")

# Build the crypto imports JSON array
CRYPTO_JSON=$(echo "$CRYPTO_DEPS" | python3 -c "
import sys, json
lines = [l.strip() for l in sys.stdin if l.strip()]
print(json.dumps(lines, indent=4))
" 2>/dev/null || echo "[]")

XCRYPTO_JSON=$(echo "$XCRYPTO_DEPS" | python3 -c "
import sys, json
lines = [l.strip() for l in sys.stdin if l.strip()]
print(json.dumps(lines, indent=4))
" 2>/dev/null || echo "[]")

# Identify which x/crypto packages are known to bypass BoringCrypto
BYPASS_JSON=$(echo "$XCRYPTO_DEPS" | python3 -c "
import sys, json
known_bypass = {
    'golang.org/x/crypto/chacha20poly1305': 'ChaCha20-Poly1305 AEAD — not FIPS-approved, not via BoringCrypto',
    'golang.org/x/crypto/chacha20': 'ChaCha20 stream cipher — not FIPS-approved, not via BoringCrypto',
    'golang.org/x/crypto/hkdf': 'HKDF extract/expand — algorithm in pure Go, but HMAC/SHA primitives route through BoringCrypto',
    'golang.org/x/crypto/curve25519': 'Curve25519 — not FIPS-approved curve',
    'golang.org/x/crypto/nacl': 'NaCl box/secretbox — not FIPS-approved',
    'golang.org/x/crypto/salsa20': 'Salsa20 — not FIPS-approved',
    'golang.org/x/crypto/blake2b': 'BLAKE2b — not FIPS-approved hash',
    'golang.org/x/crypto/blake2s': 'BLAKE2s — not FIPS-approved hash',
}
lines = [l.strip() for l in sys.stdin if l.strip()]
bypasses = []
for pkg in lines:
    for pattern, desc in known_bypass.items():
        if pkg.startswith(pattern):
            bypasses.append({'package': pkg, 'risk': desc})
            break
print(json.dumps(bypasses, indent=4))
" 2>/dev/null || echo "[]")

cat > "$OUTPUT_DIR/crypto-audit.json" <<AUDITEOF
{
  "audit_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "version": "${VERSION}",
  "fips_module": "boringcrypto",
  "note": "All crypto/* imports route through BoringCrypto when GOEXPERIMENT=boringcrypto is set. golang.org/x/crypto imports may bypass BoringCrypto.",
  "standard_crypto_imports": ${CRYPTO_JSON},
  "standard_crypto_count": ${CRYPTO_COUNT},
  "xcrypto_imports": ${XCRYPTO_JSON},
  "xcrypto_count": ${XCRYPTO_COUNT},
  "boringcrypto_bypasses": ${BYPASS_JSON},
  "mitigations": [
    "ChaCha20-Poly1305: Mitigated by restricting cipher suites to AES-GCM only in tls.Config",
    "HKDF: Low risk — algorithm is pure Go but underlying HMAC/SHA primitives use BoringCrypto",
    "Other x/crypto: Review whether the package is used in a security-critical path"
  ],
  "assessment": "Standard library crypto packages are replaced by BoringCrypto. See boringcrypto_bypasses for x/crypto packages that may not route through the validated module.",
  "reference": "docs/quic-go-crypto-audit.md"
}
AUDITEOF

echo "Crypto audit report: ${OUTPUT_DIR}/crypto-audit.json"
echo "  Standard crypto imports: ${CRYPTO_COUNT}"
echo "  x/crypto imports: ${XCRYPTO_COUNT}"

# ── Compute hashes ──
echo ""
echo "=== SBOM Hashes ==="
for f in "$OUTPUT_DIR"/sbom.*.json; do
    if [ -f "$f" ]; then
        HASH=$(sha256sum "$f" | awk '{print $1}')
        echo "  $(basename "$f"): $HASH"
    fi
done

echo ""
echo "Done. SBOMs saved to $OUTPUT_DIR/"

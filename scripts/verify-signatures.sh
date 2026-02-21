#!/bin/bash
# Verify GPG signatures for cloudflared-fips artifacts.
#
# Usage:
#   ./verify-signatures.sh <artifact-dir>
#   ./verify-signatures.sh /path/to/cloudflared-fips-linux-amd64/
#
# This script:
# 1. Imports the project's public key (if not already imported)
# 2. Verifies each .sig file against its artifact
# 3. Validates the signatures.json manifest
# 4. Reports results

set -euo pipefail

ARTIFACT_DIR="${1:?Usage: verify-signatures.sh <artifact-dir>}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PUBLIC_KEY_PATH="${PROJECT_ROOT}/configs/public-key.asc"

echo "=== cloudflared-fips Signature Verification ==="
echo "Artifact directory: ${ARTIFACT_DIR}"
echo ""

# ── Step 1: Import public key ──
if [ -f "$PUBLIC_KEY_PATH" ]; then
    echo "--- Importing project public key ---"
    FINGERPRINT=$(gpg --with-colons --import-options show-only --import "$PUBLIC_KEY_PATH" 2>/dev/null \
        | grep '^fpr:' | head -1 | cut -d: -f10 || echo "unknown")
    echo "Key fingerprint: ${FINGERPRINT}"

    # Check if already imported
    if gpg --list-keys "$FINGERPRINT" >/dev/null 2>&1; then
        echo "Key already in keyring."
    else
        gpg --batch --import "$PUBLIC_KEY_PATH" 2>/dev/null
        echo "Key imported successfully."
    fi
else
    echo "WARNING: Project public key not found at ${PUBLIC_KEY_PATH}"
    echo "Signature verification will use your existing GPG keyring."
    echo ""
    echo "To obtain the public key:"
    echo "  1. Download from the project's GitHub Releases page"
    echo "  2. Or: curl -fsSL https://github.com/cloudflared-fips/cloudflared-fips/releases/latest/download/public-key.asc -o ${PUBLIC_KEY_PATH}"
fi

echo ""

# ── Step 2: Verify signatures.json manifest ──
MANIFEST="${ARTIFACT_DIR}/signatures.json"
if [ -f "$MANIFEST" ]; then
    echo "--- Signatures Manifest ---"
    echo "Found: ${MANIFEST}"

    ARTIFACT_COUNT=$(python3 -c "import json; d=json.load(open('${MANIFEST}')); print(len(d.get('signatures',[])))" 2>/dev/null || echo "?")
    echo "Artifacts listed: ${ARTIFACT_COUNT}"
    echo ""
else
    echo "No signatures.json found — verifying .sig files directly."
    echo ""
fi

# ── Step 3: Verify each .sig file ──
echo "--- Signature Verification ---"
VERIFIED=0
FAILED=0
MISSING=0

for sig_file in "${ARTIFACT_DIR}"/*.sig; do
    [ -f "$sig_file" ] || continue

    # Derive the artifact path from the .sig path
    artifact="${sig_file%.sig}"
    basename_sig=$(basename "$sig_file")
    basename_art=$(basename "$artifact")

    if [ ! -f "$artifact" ]; then
        echo "  [MISSING] ${basename_art} (artifact not found for ${basename_sig})"
        MISSING=$((MISSING + 1))
        continue
    fi

    # Verify GPG signature
    if gpg --batch --verify "$sig_file" "$artifact" 2>/dev/null; then
        echo "  [PASS] ${basename_art}"
        VERIFIED=$((VERIFIED + 1))
    else
        echo "  [FAIL] ${basename_art}"
        FAILED=$((FAILED + 1))
    fi
done

# Also check for any artifacts WITHOUT signatures
echo ""
echo "--- Unsigned Artifact Check ---"
UNSIGNED=0
for artifact in "${ARTIFACT_DIR}"/cloudflared* "${ARTIFACT_DIR}"/selftest* "${ARTIFACT_DIR}"/*.rpm "${ARTIFACT_DIR}"/*.deb "${ARTIFACT_DIR}"/*.pkg "${ARTIFACT_DIR}"/*.msi; do
    [ -f "$artifact" ] || continue
    [[ "$artifact" == *.sig ]] && continue
    [[ "$artifact" == *.json ]] && continue

    if [ ! -f "${artifact}.sig" ]; then
        echo "  [UNSIGNED] $(basename "$artifact")"
        UNSIGNED=$((UNSIGNED + 1))
    fi
done

if [ "$UNSIGNED" -eq 0 ]; then
    echo "  All artifacts are signed."
fi

# ── Step 4: Hash verification (if signatures.json present) ──
if [ -f "$MANIFEST" ]; then
    echo ""
    echo "--- Hash Verification (signatures.json) ---"
    python3 -c "
import json, hashlib, sys, os

manifest_path = '${MANIFEST}'
artifact_dir = '${ARTIFACT_DIR}'

with open(manifest_path) as f:
    manifest = json.load(f)

verified = 0
failed = 0
for sig in manifest.get('signatures', []):
    path = sig.get('artifact_path', '')
    expected_hash = sig.get('artifact_sha256', '')
    if not path or not expected_hash:
        continue

    # Try relative to artifact dir
    full_path = os.path.join(artifact_dir, os.path.basename(path))
    if not os.path.exists(full_path):
        full_path = path  # Try absolute

    if not os.path.exists(full_path):
        print(f'  [SKIP] {os.path.basename(path)} (not found)')
        continue

    with open(full_path, 'rb') as f:
        actual_hash = hashlib.sha256(f.read()).hexdigest()

    if actual_hash == expected_hash:
        print(f'  [PASS] {os.path.basename(path)} SHA-256 matches')
        verified += 1
    else:
        print(f'  [FAIL] {os.path.basename(path)} SHA-256 MISMATCH')
        print(f'         Expected: {expected_hash}')
        print(f'         Actual:   {actual_hash}')
        failed += 1

print(f'\nHash verification: {verified} passed, {failed} failed')
" 2>/dev/null || echo "  (requires python3 for hash verification)"
fi

# ── Summary ──
echo ""
echo "=== Verification Summary ==="
echo "  Signatures verified: ${VERIFIED}"
echo "  Signatures failed:   ${FAILED}"
echo "  Artifacts missing:   ${MISSING}"
echo "  Unsigned artifacts:  ${UNSIGNED}"

if [ "$FAILED" -gt 0 ]; then
    echo ""
    echo "WARNING: ${FAILED} signature(s) FAILED verification."
    echo "Do NOT use these artifacts in a production environment."
    exit 1
fi

if [ "$VERIFIED" -eq 0 ] && [ "$UNSIGNED" -gt 0 ]; then
    echo ""
    echo "WARNING: No signatures were verified. Artifacts are unsigned."
    exit 2
fi

echo ""
echo "All verified signatures are valid."

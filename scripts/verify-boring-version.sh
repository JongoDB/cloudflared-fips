#!/bin/bash
# Verify which version of BoringCrypto (140-2 or 140-3) is linked.
#
# Checks:
# 1. The Go toolchain's BoringCrypto .syso file hash
# 2. BoringSSL version strings in a compiled binary
# 3. Known CMVP certificate mapping
#
# Usage: ./verify-boring-version.sh [binary-path]
#
# If binary-path is provided, inspects the binary directly.
# Otherwise, inspects the Go toolchain's .syso files.

set -euo pipefail

BINARY="${1:-}"

# Known BoringCrypto .syso SHA-256 hashes mapped to FIPS certificates
# These hashes are for the pre-compiled .syso object files distributed with Go.
# When Google updates the BoringSSL FIPS module, new hashes appear.
declare -A KNOWN_SYSO_HASHES=(
    # Go 1.22.x BoringCrypto — FIPS 140-2 cert #4407
    # Module: BoringCrypto, based on BoringSSL fips-20220613
    ["fips-20220613"]="140-2 (#4407)"

    # Go 1.24.x BoringCrypto — FIPS 140-3 cert #4735
    # Module: BoringCrypto, based on BoringSSL fips-20230428 or later
    ["fips-20230428"]="140-3 (#4735)"
)

# Known BoringSSL version strings mapped to FIPS standard
declare -A KNOWN_VERSIONS=(
    ["BoringSSL FIPS 20220613"]="FIPS 140-2 (CMVP #3678, #4407)"
    ["BoringSSL FIPS 20230428"]="FIPS 140-3 (CMVP #4735)"
    ["BoringCrypto"]="BoringCrypto detected (version unknown — check symbol details)"
)

echo "=== BoringCrypto FIPS Version Verification ==="
echo ""

# ── Step 1: Check Go toolchain .syso files ──
echo "--- Go Toolchain Analysis ---"

if command -v go >/dev/null 2>&1; then
    GOROOT=$(go env GOROOT 2>/dev/null || echo "")
    GOVERSION=$(go version 2>/dev/null | awk '{print $3}')
    echo "Go version: ${GOVERSION}"
    echo "GOROOT: ${GOROOT}"

    # Look for BoringCrypto .syso files
    SYSO_DIR="${GOROOT}/src/crypto/internal/boring/syso"
    if [ -d "$SYSO_DIR" ]; then
        echo "BoringCrypto .syso directory found: ${SYSO_DIR}"
        for f in "${SYSO_DIR}"/*.syso; do
            if [ -f "$f" ]; then
                HASH=$(sha256sum "$f" | awk '{print $1}')
                echo "  $(basename "$f"): ${HASH}"
            fi
        done
    else
        echo "No .syso directory at ${SYSO_DIR}"
        # Go 1.24+ may use a different path
        BORING_DIR="${GOROOT}/src/crypto/internal/boring"
        if [ -d "$BORING_DIR" ]; then
            echo "BoringCrypto package found at: ${BORING_DIR}"
            ls -la "${BORING_DIR}"/*.syso 2>/dev/null || echo "  No .syso files found"
        fi
    fi

    # Check Go experiment setting
    GOEXPERIMENT=$(go env GOEXPERIMENT 2>/dev/null || echo "")
    echo "GOEXPERIMENT: ${GOEXPERIMENT:-<not set>}"

    if [[ "$GOEXPERIMENT" == *"boringcrypto"* ]]; then
        echo "  BoringCrypto experiment is ENABLED"
    else
        echo "  BoringCrypto experiment is NOT enabled"
        echo "  Set GOEXPERIMENT=boringcrypto to enable"
    fi
else
    echo "Go not found in PATH — skipping toolchain analysis"
fi

echo ""

# ── Step 2: Inspect binary for BoringSSL version strings ──
if [ -n "$BINARY" ]; then
    echo "--- Binary Analysis ---"
    echo "Binary: ${BINARY}"

    if [ ! -f "$BINARY" ]; then
        echo "[FAIL] Binary not found: ${BINARY}"
        exit 1
    fi

    BINARY_SIZE=$(stat -c%s "$BINARY" 2>/dev/null || stat -f%z "$BINARY" 2>/dev/null || echo "unknown")
    BINARY_SHA256=$(sha256sum "$BINARY" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$BINARY" | awk '{print $1}')
    echo "Size: ${BINARY_SIZE} bytes"
    echo "SHA-256: ${BINARY_SHA256}"
    echo ""

    # Search for BoringSSL version strings
    echo "Searching for BoringSSL version strings..."
    VERSION_FOUND=false

    for pattern in "${!KNOWN_VERSIONS[@]}"; do
        if strings "$BINARY" 2>/dev/null | grep -q "$pattern"; then
            echo "  [FOUND] ${pattern} → ${KNOWN_VERSIONS[$pattern]}"
            VERSION_FOUND=true
        fi
    done

    if [ "$VERSION_FOUND" = false ]; then
        # Broader search
        BORING_STRINGS=$(strings "$BINARY" 2>/dev/null | grep -i "boring" | head -10 || true)
        if [ -n "$BORING_STRINGS" ]; then
            echo "  BoringCrypto references found:"
            echo "$BORING_STRINGS" | while read -r line; do
                echo "    $line"
            done
        else
            echo "  No BoringSSL/BoringCrypto version strings found in binary"
            echo "  This may indicate the binary was NOT built with GOEXPERIMENT=boringcrypto"
        fi
    fi

    # Check for BoringCrypto symbols
    echo ""
    echo "Checking BoringCrypto symbols..."
    if command -v go >/dev/null 2>&1; then
        SYMBOL_COUNT=$(go tool nm "$BINARY" 2>/dev/null | grep -c '_Cfunc__goboringcrypto_' || echo "0")
    elif command -v nm >/dev/null 2>&1; then
        SYMBOL_COUNT=$(nm "$BINARY" 2>/dev/null | grep -c 'goboringcrypto' || echo "0")
    else
        SYMBOL_COUNT="unknown"
    fi
    echo "  BoringCrypto symbols: ${SYMBOL_COUNT}"

    # Check for FIPS self-test indicators
    echo ""
    echo "Checking for FIPS self-test indicators..."
    FIPS_STRINGS=$(strings "$BINARY" 2>/dev/null | grep -i "FIPS\|fips_enabled\|fips140" | head -10 || true)
    if [ -n "$FIPS_STRINGS" ]; then
        echo "  FIPS-related strings found:"
        echo "$FIPS_STRINGS" | while read -r line; do
            echo "    $line"
        done
    fi
fi

echo ""

# ── Step 3: Summary and recommendation ──
echo "--- Recommendation ---"
echo ""
echo "FIPS 140-2 Sunset: September 21, 2026"
echo ""
echo "To ensure FIPS 140-3 compliance:"
echo "  1. Use Go 1.24+ with GOEXPERIMENT=boringcrypto"
echo "  2. Verify the BoringSSL version is fips-20230428 or later (CMVP #4735)"
echo "  3. If using older Go (1.22/1.23), the .syso files contain the 140-2 module"
echo "  4. Alternative: use GODEBUG=fips140=on for Go native FIPS 140-3 (CAVP A6650)"
echo ""
echo "CMVP references:"
echo "  140-2 #4407: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4407"
echo "  140-3 #4735: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4735"

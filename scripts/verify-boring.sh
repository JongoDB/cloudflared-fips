#!/usr/bin/env bash
# Verify BoringCrypto symbols are present in the binary.
# Uses go tool nm, objdump, or nm depending on availability.
set -euo pipefail

BINARY="${1:?Usage: verify-boring.sh <binary-path>}"

echo "=== BoringCrypto Symbol Verification ==="
echo "Binary: ${BINARY}"
echo ""

if [[ ! -f "${BINARY}" ]]; then
    echo "[FAIL] Binary not found: ${BINARY}"
    exit 1
fi

FOUND=false

# Method 1: go tool nm
if command -v go &>/dev/null; then
    echo "--- Using go tool nm ---"
    SYMBOLS=$(go tool nm "${BINARY}" 2>/dev/null | grep '_Cfunc__goboringcrypto_' || true)
    if [[ -n "${SYMBOLS}" ]]; then
        COUNT=$(echo "${SYMBOLS}" | wc -l | tr -d ' ')
        echo "[PASS] Found ${COUNT} BoringCrypto symbols via go tool nm"
        echo ""
        echo "Sample symbols:"
        echo "${SYMBOLS}" | head -20
        FOUND=true
    else
        echo "[INFO] No symbols found via go tool nm"
    fi
fi

# Method 2: objdump
if [[ "${FOUND}" == "false" ]] && command -v objdump &>/dev/null; then
    echo "--- Using objdump ---"
    SYMBOLS=$(objdump -t "${BINARY}" 2>/dev/null | grep 'goboringcrypto' || true)
    if [[ -n "${SYMBOLS}" ]]; then
        COUNT=$(echo "${SYMBOLS}" | wc -l | tr -d ' ')
        echo "[PASS] Found ${COUNT} BoringCrypto symbols via objdump"
        FOUND=true
    else
        echo "[INFO] No symbols found via objdump"
    fi
fi

# Method 3: nm
if [[ "${FOUND}" == "false" ]] && command -v nm &>/dev/null; then
    echo "--- Using nm ---"
    SYMBOLS=$(nm "${BINARY}" 2>/dev/null | grep 'goboringcrypto' || true)
    if [[ -n "${SYMBOLS}" ]]; then
        COUNT=$(echo "${SYMBOLS}" | wc -l | tr -d ' ')
        echo "[PASS] Found ${COUNT} BoringCrypto symbols via nm"
        FOUND=true
    else
        echo "[INFO] No symbols found via nm"
    fi
fi

echo ""
if [[ "${FOUND}" == "true" ]]; then
    echo "=== BoringCrypto Verification PASSED ==="
else
    echo "=== BoringCrypto Verification FAILED ==="
    echo "The binary does not appear to contain BoringCrypto symbols."
    echo "Ensure the build used GOEXPERIMENT=boringcrypto with CGO_ENABLED=1."
    exit 1
fi

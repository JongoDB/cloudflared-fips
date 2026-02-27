#!/usr/bin/env bash
# Post-build FIPS validation script
# Verifies that the binary meets FIPS 140-2 requirements.
set -euo pipefail

BINARY="${1:?Usage: check-fips.sh <binary-path>}"
ERRORS=0

echo "=== FIPS Validation: ${BINARY} ==="
echo ""

# Check 1: Binary exists and is executable
if [[ ! -f "${BINARY}" ]]; then
    echo "[FAIL] Binary not found: ${BINARY}"
    exit 1
fi

if [[ ! -x "${BINARY}" ]]; then
    echo "[WARN] Binary is not executable, setting +x"
    chmod +x "${BINARY}"
fi

# Check 2: BoringCrypto present (build info or symbols)
echo "--- Checking BoringCrypto ---"
BORING_FOUND=false
# Primary: go version -m reads embedded build info (works on stripped binaries)
if command -v go &>/dev/null; then
    BUILD_INFO=$(go version -m "${BINARY}" 2>/dev/null || true)
    if echo "${BUILD_INFO}" | grep -q 'GOEXPERIMENT=boringcrypto'; then
        echo "[PASS] Binary built with GOEXPERIMENT=boringcrypto (via build info)"
        BORING_FOUND=true
    fi
fi
# Fallback: symbol table inspection (requires unstripped binary)
if [[ "${BORING_FOUND}" == "false" ]]; then
    if command -v go &>/dev/null; then
        if go tool nm "${BINARY}" 2>/dev/null | grep -q '_Cfunc__goboringcrypto_'; then
            SYMBOL_COUNT=$(go tool nm "${BINARY}" 2>/dev/null | grep -c '_Cfunc__goboringcrypto_' || true)
            echo "[PASS] Found ${SYMBOL_COUNT} BoringCrypto symbols"
            BORING_FOUND=true
        fi
    fi
fi
if [[ "${BORING_FOUND}" == "false" ]]; then
    if command -v objdump &>/dev/null && objdump -t "${BINARY}" 2>/dev/null | grep -q 'goboringcrypto'; then
        echo "[PASS] BoringCrypto symbols found (via objdump)"
        BORING_FOUND=true
    elif command -v nm &>/dev/null && nm "${BINARY}" 2>/dev/null | grep -q 'goboringcrypto'; then
        echo "[PASS] BoringCrypto symbols found (via nm)"
        BORING_FOUND=true
    fi
fi
if [[ "${BORING_FOUND}" == "false" ]]; then
    echo "[FAIL] No BoringCrypto detected (checked build info and symbol tables)"
    ERRORS=$((ERRORS + 1))
fi

# Check 3: No banned ciphers in strings
echo ""
echo "--- Checking for banned cipher references ---"
BANNED_PATTERNS=("RC4" "DES-CBC" "3DES" "EXPORT" "NULL")
for pattern in "${BANNED_PATTERNS[@]}"; do
    if strings "${BINARY}" 2>/dev/null | grep -qi "cipher.*${pattern}"; then
        echo "[WARN] Found reference to banned cipher pattern: ${pattern}"
    else
        echo "[PASS] No reference to ${pattern}"
    fi
done

# Check 4: Static linking check
echo ""
echo "--- Checking linking ---"
if command -v ldd &>/dev/null; then
    if ldd "${BINARY}" 2>&1 | grep -q "not a dynamic executable"; then
        echo "[PASS] Binary is statically linked"
    elif ldd "${BINARY}" 2>&1 | grep -q "statically linked"; then
        echo "[PASS] Binary is statically linked"
    else
        echo "[INFO] Binary is dynamically linked (expected for CGO_ENABLED=1)"
        ldd "${BINARY}" 2>/dev/null | head -10
    fi
elif command -v file &>/dev/null; then
    FILE_OUT=$(file "${BINARY}")
    if echo "${FILE_OUT}" | grep -q "statically linked"; then
        echo "[PASS] Binary is statically linked"
    else
        echo "[INFO] ${FILE_OUT}"
    fi
fi

# Check 5: Binary hash
echo ""
echo "--- Binary hash ---"
if command -v sha256sum &>/dev/null; then
    sha256sum "${BINARY}"
elif command -v shasum &>/dev/null; then
    shasum -a 256 "${BINARY}"
fi

echo ""
if [[ ${ERRORS} -gt 0 ]]; then
    echo "=== FIPS Validation FAILED (${ERRORS} errors) ==="
    exit 1
else
    echo "=== FIPS Validation PASSED ==="
fi

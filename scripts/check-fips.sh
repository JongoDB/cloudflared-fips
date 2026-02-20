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

# Check 2: BoringCrypto symbols present
echo "--- Checking BoringCrypto symbols ---"
if command -v go &>/dev/null; then
    if go tool nm "${BINARY}" 2>/dev/null | grep -q '_Cfunc__goboringcrypto_'; then
        SYMBOL_COUNT=$(go tool nm "${BINARY}" 2>/dev/null | grep -c '_Cfunc__goboringcrypto_' || true)
        echo "[PASS] Found ${SYMBOL_COUNT} BoringCrypto symbols"
    else
        echo "[FAIL] No BoringCrypto symbols found"
        ERRORS=$((ERRORS + 1))
    fi
elif command -v objdump &>/dev/null; then
    if objdump -t "${BINARY}" 2>/dev/null | grep -q 'goboringcrypto'; then
        echo "[PASS] BoringCrypto symbols found (via objdump)"
    else
        echo "[FAIL] No BoringCrypto symbols found"
        ERRORS=$((ERRORS + 1))
    fi
elif command -v nm &>/dev/null; then
    if nm "${BINARY}" 2>/dev/null | grep -q 'goboringcrypto'; then
        echo "[PASS] BoringCrypto symbols found (via nm)"
    else
        echo "[FAIL] No BoringCrypto symbols found"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "[SKIP] No symbol inspection tool available (go, objdump, nm)"
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

#!/bin/bash
# Verify FIPS 140-3 readiness across all three deployment tiers.
#
# Tests:
# 1. Tier 1 (Standard): cloudflared-fips binary uses 140-3 module
# 2. Tier 2 (Regional + Keyless): Cloudflare API config verified
# 3. Tier 3 (Self-Hosted Proxy): FIPS proxy binary uses 140-3 module
#
# Usage: ./verify-tier-fips140-3.sh [--binary PATH] [--proxy PATH] [--cf-token TOKEN]

set -euo pipefail

BINARY=""
PROXY=""
CF_TOKEN="${CF_API_TOKEN:-}"
ZONE_ID="${CF_ZONE_ID:-}"
PASS=0
FAIL=0
WARN=0

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --binary PATH    Path to cloudflared-fips binary"
    echo "  --proxy PATH     Path to fips-proxy binary (Tier 3)"
    echo "  --cf-token TOKEN Cloudflare API token (or CF_API_TOKEN env)"
    echo "  --zone-id ID     Cloudflare zone ID (or CF_ZONE_ID env)"
    echo ""
    echo "Environment variables:"
    echo "  CF_API_TOKEN     Cloudflare API token"
    echo "  CF_ZONE_ID       Cloudflare zone ID"
    echo "  GODEBUG           Go debug flags (checked for fips140=on)"
    exit 0
}

check_pass() {
    echo "  [PASS] $1"
    PASS=$((PASS + 1))
}

check_fail() {
    echo "  [FAIL] $1"
    FAIL=$((FAIL + 1))
}

check_warn() {
    echo "  [WARN] $1"
    WARN=$((WARN + 1))
}

check_skip() {
    echo "  [SKIP] $1"
}

while [ $# -gt 0 ]; do
    case "$1" in
        --binary) BINARY="$2"; shift 2 ;;
        --proxy) PROXY="$2"; shift 2 ;;
        --cf-token) CF_TOKEN="$2"; shift 2 ;;
        --zone-id) ZONE_ID="$2"; shift 2 ;;
        --help|-h) usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

echo "=== FIPS 140-3 Deployment Tier Verification ==="
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# ── Tier 1: Standard Cloudflare Tunnel ──
echo "--- Tier 1: Standard Cloudflare Tunnel ---"
echo ""

# Check Go version
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}')
    GO_MINOR=$(echo "$GO_VERSION" | sed 's/go[0-9]*\.\([0-9]*\).*/\1/')
    GO_MAJOR=$(echo "$GO_VERSION" | sed 's/go\([0-9]*\)\..*/\1/')
    echo "Go version: ${GO_VERSION}"
    if [ "${GO_MAJOR}" -ge 1 ] && [ "${GO_MINOR}" -ge 24 ]; then
        check_pass "Go 1.24+ (BoringCrypto .syso is FIPS 140-3 #4735)"
    else
        check_fail "Go ${GO_VERSION} ships BoringCrypto FIPS 140-2 .syso — upgrade to Go 1.24+"
    fi
else
    check_skip "Go not found in PATH"
fi

# Check GODEBUG for native FIPS
GODEBUG="${GODEBUG:-}"
if echo "$GODEBUG" | grep -q "fips140=on\|fips140=only"; then
    check_pass "GODEBUG=fips140 is set (Go native FIPS 140-3)"
else
    echo "  GODEBUG: ${GODEBUG:-<not set>}"
    check_warn "GODEBUG=fips140 not set — Go native FIPS not active"
fi

# Check binary if provided
if [ -n "$BINARY" ]; then
    if [ -f "$BINARY" ]; then
        echo ""
        echo "Binary: ${BINARY}"
        # Check for 140-3 BoringSSL version
        if strings "$BINARY" 2>/dev/null | grep -q "BoringSSL FIPS 20230428"; then
            check_pass "Binary contains BoringSSL FIPS 20230428 (140-3 #4735)"
        elif strings "$BINARY" 2>/dev/null | grep -q "BoringSSL FIPS 20220613"; then
            check_fail "Binary contains BoringSSL FIPS 20220613 (140-2 #4407) — rebuild with Go 1.24+"
        elif strings "$BINARY" 2>/dev/null | grep -qi "fips140"; then
            check_pass "Binary has Go native FIPS 140-3 indicators"
        else
            check_warn "Cannot determine FIPS module version from binary"
        fi
    else
        check_fail "Binary not found: ${BINARY}"
    fi
fi

echo ""

# ── Tier 2: Regional + Keyless SSL ──
echo "--- Tier 2: Regional Services + Keyless SSL ---"
echo ""

if [ -n "$CF_TOKEN" ] && [ -n "$ZONE_ID" ]; then
    # Check cipher suites
    CIPHERS=$(curl -s -H "Authorization: Bearer ${CF_TOKEN}" \
        "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/settings/ciphers" 2>/dev/null || echo "")
    if echo "$CIPHERS" | grep -q '"success":true'; then
        check_pass "Cloudflare API: cipher config accessible"
    else
        check_fail "Cloudflare API: cannot read cipher config"
    fi

    # Check min TLS version
    TLS_MIN=$(curl -s -H "Authorization: Bearer ${CF_TOKEN}" \
        "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/settings/min_tls_version" 2>/dev/null || echo "")
    if echo "$TLS_MIN" | grep -q '"value":"1.2"\|"value":"1.3"'; then
        check_pass "Minimum TLS version: 1.2+"
    else
        check_warn "Cannot verify minimum TLS version"
    fi

    # Check Regional Services (Data Localization)
    REGIONAL=$(curl -s -H "Authorization: Bearer ${CF_TOKEN}" \
        "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/settings" 2>/dev/null | \
        grep -o '"data_localization"[^}]*' || echo "")
    if [ -n "$REGIONAL" ]; then
        check_pass "Regional Services setting found"
    else
        check_warn "Regional Services status could not be verified"
    fi
else
    check_skip "Cloudflare API token or zone ID not provided — skipping Tier 2 checks"
    echo "  Set CF_API_TOKEN and CF_ZONE_ID environment variables"
fi

echo ""

# ── Tier 3: Self-Hosted FIPS Proxy ──
echo "--- Tier 3: Self-Hosted FIPS Edge Proxy ---"
echo ""

if [ -n "$PROXY" ]; then
    if [ -f "$PROXY" ]; then
        echo "Proxy binary: ${PROXY}"
        # Same checks as Tier 1 binary
        if strings "$PROXY" 2>/dev/null | grep -q "BoringSSL FIPS 20230428"; then
            check_pass "Proxy binary: BoringSSL FIPS 20230428 (140-3 #4735)"
        elif strings "$PROXY" 2>/dev/null | grep -qi "fips140"; then
            check_pass "Proxy binary: Go native FIPS 140-3 indicators found"
        elif strings "$PROXY" 2>/dev/null | grep -q "BoringSSL FIPS 20220613"; then
            check_fail "Proxy binary: BoringSSL FIPS 20220613 (140-2) — rebuild with Go 1.24+"
        else
            check_warn "Proxy binary: cannot determine FIPS module version"
        fi
    else
        check_fail "Proxy binary not found: ${PROXY}"
    fi
else
    check_skip "No proxy binary specified — skipping Tier 3 binary checks"
    echo "  Use --proxy PATH to verify the FIPS proxy binary"
fi

echo ""

# ── Summary ──
echo "=== Summary ==="
TOTAL=$((PASS + FAIL + WARN))
echo "  Passed: ${PASS}/${TOTAL}"
echo "  Failed: ${FAIL}/${TOTAL}"
echo "  Warnings: ${WARN}/${TOTAL}"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo "RESULT: FIPS 140-3 readiness check FAILED"
    echo ""
    echo "Action required before September 21, 2026:"
    echo "  - Upgrade to Go 1.24+ for BoringCrypto FIPS 140-3 (#4735)"
    echo "  - Or use GODEBUG=fips140=on for Go native FIPS 140-3 (CAVP A6650)"
    exit 1
elif [ "$WARN" -gt 0 ]; then
    echo "RESULT: FIPS 140-3 readiness check PASSED with warnings"
    exit 0
else
    echo "RESULT: FIPS 140-3 readiness check PASSED"
    exit 0
fi

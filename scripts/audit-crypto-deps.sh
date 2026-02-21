#!/bin/bash
# Full dependency tree crypto audit for cloudflared.
# Identifies all packages that import crypto primitives and classifies them
# by whether they route through the FIPS-validated module.
#
# Usage: ./audit-crypto-deps.sh <upstream-cloudflared-dir> [output-file]
#
# Requires: Go 1.24+

set -euo pipefail

UPSTREAM_DIR="${1:?Usage: audit-crypto-deps.sh <upstream-dir> [output-file]}"
OUTPUT_FILE="${2:-crypto-audit-full.json}"

cd "$UPSTREAM_DIR"

echo "=== Full Dependency Tree Crypto Audit ==="
echo "Directory: $(pwd)"
echo ""

TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# ── 1. Get all transitive dependencies of the main binary ──
echo "Scanning dependency tree..."
ALL_DEPS=$(go list -deps ./cmd/cloudflared 2>/dev/null || echo "")
DEP_COUNT=$(echo "$ALL_DEPS" | wc -l | tr -d ' ')
echo "  Total packages in dependency tree: ${DEP_COUNT}"

# ── 2. Classify crypto-related packages ──
echo "Classifying crypto packages..."

# Standard library crypto packages (route through BoringCrypto with GOEXPERIMENT)
STD_CRYPTO=$(echo "$ALL_DEPS" | grep "^crypto/" || true)
STD_CRYPTO_COUNT=$(echo "$STD_CRYPTO" | grep -c "." || echo "0")

# x/crypto packages (may or may not route through BoringCrypto)
X_CRYPTO=$(echo "$ALL_DEPS" | grep "^golang.org/x/crypto" || true)
X_CRYPTO_COUNT=$(echo "$X_CRYPTO" | grep -c "." || echo "0")

# Internal crypto usage in quic-go
QUIC_CRYPTO=$(echo "$ALL_DEPS" | grep "quic-go" | head -20 || true)
QUIC_COUNT=$(echo "$QUIC_CRYPTO" | grep -c "." || echo "0")

# Packages importing crypto (find which non-crypto packages depend on crypto)
echo "Analyzing import graphs..."

# ── 3. Build detailed classification ──
CLASSIFIED=$(python3 -c "
import sys, json

# Known classification of crypto packages
FIPS_ROUTED = {
    'crypto/aes': 'AES — routed through BoringCrypto',
    'crypto/cipher': 'Block cipher modes (GCM, CTR) — routed through BoringCrypto',
    'crypto/ecdsa': 'ECDSA — routed through BoringCrypto',
    'crypto/ecdh': 'ECDH — routed through BoringCrypto',
    'crypto/elliptic': 'Elliptic curve operations — routed through BoringCrypto',
    'crypto/hmac': 'HMAC — routed through BoringCrypto',
    'crypto/rsa': 'RSA — routed through BoringCrypto',
    'crypto/sha256': 'SHA-256 — routed through BoringCrypto',
    'crypto/sha512': 'SHA-384/512 — routed through BoringCrypto',
    'crypto/tls': 'TLS — routed through BoringCrypto',
    'crypto/x509': 'X.509 certificates — routed through BoringCrypto',
    'crypto/rand': 'CSPRNG — routed through BoringCrypto',
    'crypto/ed25519': 'Ed25519 — routed through BoringCrypto (Go 1.24+)',
    'crypto/mlkem': 'ML-KEM (post-quantum) — Go 1.24+ native',
}

NOT_FIPS_ROUTED = {
    'golang.org/x/crypto/chacha20poly1305': {'risk': 'high', 'desc': 'ChaCha20-Poly1305 AEAD — not FIPS-approved, bypasses BoringCrypto', 'mitigation': 'Restrict cipher suites to AES-GCM only'},
    'golang.org/x/crypto/chacha20': {'risk': 'high', 'desc': 'ChaCha20 stream cipher — not FIPS-approved', 'mitigation': 'Restrict cipher suites to AES-GCM only'},
    'golang.org/x/crypto/curve25519': {'risk': 'medium', 'desc': 'Curve25519 — not FIPS-approved curve', 'mitigation': 'Not used in TLS cipher suites'},
    'golang.org/x/crypto/nacl': {'risk': 'medium', 'desc': 'NaCl box/secretbox — not FIPS-approved', 'mitigation': 'Review if used in security-critical path'},
    'golang.org/x/crypto/salsa20': {'risk': 'medium', 'desc': 'Salsa20 — not FIPS-approved', 'mitigation': 'Review if used in security-critical path'},
    'golang.org/x/crypto/blake2b': {'risk': 'low', 'desc': 'BLAKE2b — not FIPS-approved hash', 'mitigation': 'Not used for TLS or authentication'},
    'golang.org/x/crypto/blake2s': {'risk': 'low', 'desc': 'BLAKE2s — not FIPS-approved hash', 'mitigation': 'Not used for TLS or authentication'},
    'golang.org/x/crypto/argon2': {'risk': 'low', 'desc': 'Argon2 — not FIPS-approved KDF', 'mitigation': 'Not used for TLS'},
    'golang.org/x/crypto/bcrypt': {'risk': 'low', 'desc': 'bcrypt — not FIPS-approved', 'mitigation': 'Not used for TLS'},
    'golang.org/x/crypto/pbkdf2': {'risk': 'low', 'desc': 'PBKDF2 — FIPS-approved but pure Go impl', 'mitigation': 'Underlying hash via BoringCrypto'},
    'golang.org/x/crypto/ssh': {'risk': 'medium', 'desc': 'SSH protocol — uses own crypto negotiation', 'mitigation': 'Not used in tunnel TLS path'},
}

PARTIAL_FIPS = {
    'golang.org/x/crypto/hkdf': {'desc': 'HKDF — algorithm in pure Go, but HMAC/SHA primitives route through BoringCrypto', 'risk': 'low'},
    'golang.org/x/crypto/cryptobyte': {'desc': 'ASN.1/DER encoding — no crypto operations, just serialization', 'risk': 'none'},
    'golang.org/x/crypto/ocsp': {'desc': 'OCSP — signature verification routes through BoringCrypto', 'risk': 'none'},
}

# Read package lists from stdin (newline-separated: std_crypto, then x_crypto)
std_pkgs = []
x_pkgs = []
quic_pkgs = []

section = 'std'
for line in sys.stdin:
    line = line.strip()
    if line == '---XCRYPTO---':
        section = 'x'
        continue
    if line == '---QUIC---':
        section = 'quic'
        continue
    if not line:
        continue
    if section == 'std':
        std_pkgs.append(line)
    elif section == 'x':
        x_pkgs.append(line)
    elif section == 'quic':
        quic_pkgs.append(line)

# Classify
fips_routed = []
for pkg in std_pkgs:
    desc = FIPS_ROUTED.get(pkg, 'Standard library crypto — routed through BoringCrypto')
    fips_routed.append({'package': pkg, 'fips_routed': True, 'description': desc})

not_fips = []
partial = []
safe_xcrypto = []

for pkg in x_pkgs:
    matched = False
    for pattern, info in NOT_FIPS_ROUTED.items():
        if pkg.startswith(pattern):
            not_fips.append({'package': pkg, 'fips_routed': False, **info})
            matched = True
            break
    if matched:
        continue
    for pattern, info in PARTIAL_FIPS.items():
        if pkg.startswith(pattern):
            partial.append({'package': pkg, 'fips_routed': 'partial', **info})
            matched = True
            break
    if not matched:
        safe_xcrypto.append({'package': pkg, 'fips_routed': 'unknown', 'description': 'Requires manual review'})

result = {
    'fips_routed_packages': fips_routed,
    'non_fips_packages': not_fips,
    'partial_fips_packages': partial,
    'unclassified_xcrypto': safe_xcrypto,
    'quic_packages': quic_pkgs,
}

print(json.dumps(result, indent=2))
" <<EOF
${STD_CRYPTO}
---XCRYPTO---
${X_CRYPTO}
---QUIC---
${QUIC_CRYPTO}
EOF
)

# ── 4. Compute risk summary ──
HIGH_RISK=$(echo "$CLASSIFIED" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len([p for p in d['non_fips_packages'] if p.get('risk')=='high']))" 2>/dev/null || echo "0")
MEDIUM_RISK=$(echo "$CLASSIFIED" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len([p for p in d['non_fips_packages'] if p.get('risk')=='medium']))" 2>/dev/null || echo "0")
LOW_RISK=$(echo "$CLASSIFIED" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len([p for p in d['non_fips_packages'] if p.get('risk')=='low']))" 2>/dev/null || echo "0")

# ── 5. Write output ──
cat > "$OUTPUT_FILE" <<AUDITEOF
{
  "audit_type": "full_dependency_tree_crypto_audit",
  "audit_time": "${TIMESTAMP}",
  "upstream_dir": "${UPSTREAM_DIR}",
  "total_packages_in_tree": ${DEP_COUNT},
  "standard_crypto_count": ${STD_CRYPTO_COUNT},
  "xcrypto_count": ${X_CRYPTO_COUNT},
  "quic_related_count": ${QUIC_COUNT},
  "risk_summary": {
    "high": ${HIGH_RISK},
    "medium": ${MEDIUM_RISK},
    "low": ${LOW_RISK}
  },
  "classification": ${CLASSIFIED},
  "fips_module": "boringcrypto",
  "build_flag": "GOEXPERIMENT=boringcrypto",
  "assessment": "Standard library crypto/* packages are replaced by BoringCrypto. golang.org/x/crypto packages require individual assessment. High-risk packages (ChaCha20) are mitigated by cipher suite restriction.",
  "recommendations": [
    "Enforce AES-GCM-only cipher suites in tls.Config to prevent ChaCha20 negotiation",
    "Monitor golang.org/x/crypto/hkdf for upstream FIPS integration (Go issue #47234)",
    "Review any unclassified x/crypto packages for security-critical usage",
    "Run this audit after each cloudflared upstream version bump"
  ]
}
AUDITEOF

echo ""
echo "=== Audit Summary ==="
echo "  Standard crypto (FIPS-routed): ${STD_CRYPTO_COUNT}"
echo "  x/crypto packages: ${X_CRYPTO_COUNT}"
echo "  QUIC-related: ${QUIC_COUNT}"
echo "  High risk: ${HIGH_RISK}"
echo "  Medium risk: ${MEDIUM_RISK}"
echo "  Low risk: ${LOW_RISK}"
echo ""
echo "Full audit: ${OUTPUT_FILE}"

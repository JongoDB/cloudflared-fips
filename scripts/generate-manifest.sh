#!/usr/bin/env bash
# Generate build-manifest.json matching the cloudflared-fips spec schema.
# Reads env vars set by build.sh and binary metadata.
set -euo pipefail

OUTPUT_DIR="${OUTPUT_DIR:-build-output}"
MANIFEST_PATH="${OUTPUT_DIR}/build-manifest.json"
BINARY_PATH="${OUTPUT_DIR}/cloudflared"
SBOM_PATH="${OUTPUT_DIR}/sbom.spdx.json"

VERSION="${VERSION:-dev}"
GIT_COMMIT="${GIT_COMMIT:-unknown}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
PLATFORM="${PLATFORM:-linux/amd64}"
PACKAGE_FORMAT="${PACKAGE_FORMAT:-binary}"
UPSTREAM_VERSION="${UPSTREAM_VERSION:-unknown}"
UPSTREAM_COMMIT="${UPSTREAM_COMMIT:-unknown}"

# Compute binary hash
BINARY_SHA256=""
if [[ -f "${BINARY_PATH}" ]]; then
    if command -v sha256sum &>/dev/null; then
        BINARY_SHA256=$(sha256sum "${BINARY_PATH}" | awk '{print $1}')
    elif command -v shasum &>/dev/null; then
        BINARY_SHA256=$(shasum -a 256 "${BINARY_PATH}" | awk '{print $1}')
    fi
fi

# Compute SBOM hash
SBOM_SHA256=""
if [[ -f "${SBOM_PATH}" ]]; then
    if command -v sha256sum &>/dev/null; then
        SBOM_SHA256=$(sha256sum "${SBOM_PATH}" | awk '{print $1}')
    elif command -v shasum &>/dev/null; then
        SBOM_SHA256=$(shasum -a 256 "${SBOM_PATH}" | awk '{print $1}')
    fi
fi

# Generate manifest JSON matching the spec schema
cat > "${MANIFEST_PATH}" <<EOJSON
{
  "version": "${VERSION}",
  "commit": "${GIT_COMMIT}",
  "build_time": "${BUILD_DATE}",
  "cloudflared_upstream_version": "${UPSTREAM_VERSION}",
  "cloudflared_upstream_commit": "${UPSTREAM_COMMIT}",
  "crypto_engine": "boringcrypto",
  "boringssl_version": "fips-20220613",
  "fips_certificates": [
    {
      "module": "BoringSSL",
      "certificate": "#3678",
      "algorithms": [
        "AES-GCM-128", "AES-GCM-256", "SHA-256", "SHA-384", "SHA-512",
        "HMAC-SHA-256", "ECDSA-P256", "ECDSA-P384", "RSA-2048", "RSA-4096",
        "ECDH-P256", "ECDH-P384", "HKDF", "TLS-1.2-KDF", "TLS-1.3-KDF"
      ]
    },
    {
      "module": "RHEL OpenSSL",
      "certificate": "#4349",
      "algorithms": [
        "AES-CBC-128", "AES-CBC-256", "AES-GCM-128", "AES-GCM-256",
        "SHA-1", "SHA-256", "SHA-384", "SHA-512", "HMAC-SHA-1", "HMAC-SHA-256",
        "RSA-2048", "RSA-4096", "ECDSA-P256", "ECDSA-P384",
        "ECDH-P256", "ECDH-P384", "TLS-1.2-KDF", "TLS-1.3-KDF"
      ]
    }
  ],
  "target_platform": "${PLATFORM}",
  "package_format": "${PACKAGE_FORMAT}",
  "sbom_sha256": "${SBOM_SHA256}",
  "binary_sha256": "${BINARY_SHA256}"
}
EOJSON

echo "Manifest written to ${MANIFEST_PATH}"

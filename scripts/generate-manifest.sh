#!/usr/bin/env bash
# Generate build-manifest.json from environment variables and binary metadata.
set -euo pipefail

OUTPUT_DIR="${OUTPUT_DIR:-build-output}"
MANIFEST_PATH="${OUTPUT_DIR}/build-manifest.json"
BINARY_PATH="${OUTPUT_DIR}/cloudflared"
SBOM_PATH="${OUTPUT_DIR}/sbom.spdx.json"

VERSION="${VERSION:-dev}"
GIT_COMMIT="${GIT_COMMIT:-unknown}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
PLATFORM="${PLATFORM:-linux/amd64}"

# Parse platform
TARGET_OS="${PLATFORM%%/*}"
TARGET_ARCH="${PLATFORM##*/}"
BUILD_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
BUILD_ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"

# Compute binary hash
BINARY_SHA256=""
BINARY_SIZE=0
if [[ -f "${BINARY_PATH}" ]]; then
    if command -v sha256sum &>/dev/null; then
        BINARY_SHA256=$(sha256sum "${BINARY_PATH}" | awk '{print $1}')
    elif command -v shasum &>/dev/null; then
        BINARY_SHA256=$(shasum -a 256 "${BINARY_PATH}" | awk '{print $1}')
    fi
    BINARY_SIZE=$(stat -c%s "${BINARY_PATH}" 2>/dev/null || stat -f%z "${BINARY_PATH}" 2>/dev/null || echo 0)
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

# Check for BoringCrypto symbols
BORING_SYMBOLS=false
if [[ -f "${BINARY_PATH}" ]] && command -v go &>/dev/null; then
    if go tool nm "${BINARY_PATH}" 2>/dev/null | grep -q '_Cfunc__goboringcrypto_'; then
        BORING_SYMBOLS=true
    fi
fi

# Generate manifest JSON
if ! command -v jq &>/dev/null; then
    echo "Warning: jq not found, generating manifest with cat" >&2
    cat > "${MANIFEST_PATH}" <<EOJSON
{
  "schema_version": "1.0.0",
  "build_id": "${VERSION}-${GIT_COMMIT}",
  "timestamp": "${BUILD_DATE}",
  "source": {
    "repository": "https://github.com/cloudflare/cloudflared",
    "branch": "main",
    "commit": "${GIT_COMMIT}",
    "tag": "${VERSION}"
  },
  "platform": {
    "build_os": "${BUILD_OS}",
    "build_arch": "${BUILD_ARCH}",
    "target_os": "${TARGET_OS}",
    "target_arch": "${TARGET_ARCH}",
    "builder": "rhel-ubi9-fips"
  },
  "go_version": "1.24.0",
  "go_experiment": "boringcrypto",
  "cgo_enabled": true,
  "fips_certificates": [
    {
      "module": "BoringCrypto",
      "cert_number": "4407",
      "level": "FIPS 140-2 Level 1",
      "validated_on": "2023-11-01",
      "expires_on": "2028-11-01",
      "cmvp_url": "https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4407"
    }
  ],
  "binary": {
    "name": "cloudflared",
    "path": "${BINARY_PATH}",
    "sha256": "${BINARY_SHA256}",
    "size": ${BINARY_SIZE},
    "stripped": false
  },
  "sbom": {
    "format": "spdx-json",
    "path": "${SBOM_PATH}",
    "sha256": "${SBOM_SHA256}"
  },
  "verification": {
    "boring_crypto_symbols": ${BORING_SYMBOLS},
    "self_test_passed": false,
    "banned_ciphers": [],
    "static_linked": false
  }
}
EOJSON
else
    jq -n \
        --arg sv "1.0.0" \
        --arg bid "${VERSION}-${GIT_COMMIT}" \
        --arg ts "${BUILD_DATE}" \
        --arg repo "https://github.com/cloudflare/cloudflared" \
        --arg branch "main" \
        --arg commit "${GIT_COMMIT}" \
        --arg tag "${VERSION}" \
        --arg bos "${BUILD_OS}" \
        --arg ba "${BUILD_ARCH}" \
        --arg tos "${TARGET_OS}" \
        --arg ta "${TARGET_ARCH}" \
        --arg bsha "${BINARY_SHA256}" \
        --argjson bsize "${BINARY_SIZE}" \
        --arg ssha "${SBOM_SHA256}" \
        --argjson boring "${BORING_SYMBOLS}" \
        '{
            schema_version: $sv,
            build_id: $bid,
            timestamp: $ts,
            source: { repository: $repo, branch: $branch, commit: $commit, tag: $tag },
            platform: { build_os: $bos, build_arch: $ba, target_os: $tos, target_arch: $ta, builder: "rhel-ubi9-fips" },
            go_version: "1.24.0",
            go_experiment: "boringcrypto",
            cgo_enabled: true,
            fips_certificates: [{
                module: "BoringCrypto",
                cert_number: "4407",
                level: "FIPS 140-2 Level 1",
                validated_on: "2023-11-01",
                expires_on: "2028-11-01",
                cmvp_url: "https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4407"
            }],
            binary: { name: "cloudflared", path: "build-output/cloudflared", sha256: $bsha, size: $bsize, stripped: false },
            sbom: { format: "spdx-json", path: "build-output/sbom.spdx.json", sha256: $ssha },
            verification: { boring_crypto_symbols: $boring, self_test_passed: false, banned_ciphers: [], static_linked: false }
        }' > "${MANIFEST_PATH}"
fi

echo "Manifest written to ${MANIFEST_PATH}"

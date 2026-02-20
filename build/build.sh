#!/usr/bin/env bash
# Build orchestrator for cloudflared-fips
# Drives Docker-based FIPS builds, runs verification, and generates manifests.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Defaults
VERSION="${VERSION:-dev}"
PLATFORM="${PLATFORM:-linux/amd64}"
OUTPUT_DIR="${OUTPUT_DIR:-${PROJECT_ROOT}/build-output}"
DOCKER_IMAGE="cloudflared-fips"

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
    --version VERSION       Set build version (default: dev)
    --platform PLATFORM     Target platform, e.g., linux/amd64 (default: linux/amd64)
    --output-dir DIR        Output directory (default: ./build-output)
    --help                  Show this help message

Examples:
    $(basename "$0") --version 2024.12.2-fips --platform linux/amd64
    $(basename "$0") --version 2024.12.2-fips --platform linux/arm64
EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

GIT_COMMIT="$(git -C "${PROJECT_ROOT}" rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "=== cloudflared-fips Build ==="
echo "Version:    ${VERSION}"
echo "Platform:   ${PLATFORM}"
echo "Commit:     ${GIT_COMMIT}"
echo "Output:     ${OUTPUT_DIR}"
echo ""

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Build Docker image
echo "--- Building FIPS Docker image ---"
docker build \
    --platform "${PLATFORM}" \
    -f "${PROJECT_ROOT}/build/Dockerfile.fips" \
    --build-arg VERSION="${VERSION}" \
    --build-arg GIT_COMMIT="${GIT_COMMIT}" \
    -t "${DOCKER_IMAGE}:${VERSION}" \
    "${PROJECT_ROOT}"

# Extract binary from container
echo "--- Extracting binary ---"
CONTAINER_ID=$(docker create --platform "${PLATFORM}" "${DOCKER_IMAGE}:${VERSION}")
docker cp "${CONTAINER_ID}:/usr/local/bin/cloudflared" "${OUTPUT_DIR}/cloudflared"
docker cp "${CONTAINER_ID}:/usr/local/bin/selftest" "${OUTPUT_DIR}/selftest"
docker rm "${CONTAINER_ID}"

# Run verification scripts
echo "--- Running verification ---"
if [[ -x "${PROJECT_ROOT}/scripts/verify-boring.sh" ]]; then
    "${PROJECT_ROOT}/scripts/verify-boring.sh" "${OUTPUT_DIR}/cloudflared" || true
fi

if [[ -x "${PROJECT_ROOT}/scripts/check-fips.sh" ]]; then
    "${PROJECT_ROOT}/scripts/check-fips.sh" "${OUTPUT_DIR}/cloudflared" || true
fi

# Generate manifest
echo "--- Generating build manifest ---"
export VERSION GIT_COMMIT BUILD_DATE PLATFORM OUTPUT_DIR
if [[ -x "${PROJECT_ROOT}/scripts/generate-manifest.sh" ]]; then
    "${PROJECT_ROOT}/scripts/generate-manifest.sh"
fi

echo ""
echo "=== Build Complete ==="
echo "Binary:   ${OUTPUT_DIR}/cloudflared"
echo "Selftest: ${OUTPUT_DIR}/selftest"
echo "Manifest: ${OUTPUT_DIR}/build-manifest.json"

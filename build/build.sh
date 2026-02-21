#!/usr/bin/env bash
# Build orchestrator for cloudflared-fips
# Clones upstream cloudflared, applies FIPS build flags, runs verification,
# and generates build manifest with full provenance.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Defaults
VERSION="${VERSION:-dev}"
PLATFORM="${PLATFORM:-linux/amd64}"
OUTPUT_DIR="${OUTPUT_DIR:-${PROJECT_ROOT}/build-output}"
DOCKER_IMAGE="cloudflared-fips"
UPSTREAM_DIR="${PROJECT_ROOT}/upstream-cloudflared"
CLOUDFLARED_REPO="https://github.com/cloudflare/cloudflared.git"
CLOUDFLARED_REF="${CLOUDFLARED_REF:-main}"

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
    --version VERSION          Set build version (default: dev)
    --platform PLATFORM        Target platform, e.g., linux/amd64 (default: linux/amd64)
    --output-dir DIR           Output directory (default: ./build-output)
    --cloudflared-ref REF      Upstream cloudflared git ref/tag (default: main)
    --help                     Show this help message

Examples:
    $(basename "$0") --version 2025.2.0-fips --platform linux/amd64
    $(basename "$0") --version 2025.2.0-fips --platform linux/arm64 --cloudflared-ref 2025.2.0
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
        --cloudflared-ref)
            CLOUDFLARED_REF="$2"
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
SOURCE_DATE_EPOCH="$(date +%s)"
export SOURCE_DATE_EPOCH

echo "=== cloudflared-fips Build ==="
echo "Version:           ${VERSION}"
echo "Platform:          ${PLATFORM}"
echo "Commit:            ${GIT_COMMIT}"
echo "Upstream ref:      ${CLOUDFLARED_REF}"
echo "Output:            ${OUTPUT_DIR}"
echo ""

# Step 1: Clone or update upstream cloudflared
echo "--- Syncing upstream cloudflared ---"
if [[ -d "${UPSTREAM_DIR}/.git" ]]; then
    echo "Updating existing clone..."
    git -C "${UPSTREAM_DIR}" fetch origin
    git -C "${UPSTREAM_DIR}" checkout "${CLOUDFLARED_REF}" 2>/dev/null || \
        git -C "${UPSTREAM_DIR}" checkout "origin/${CLOUDFLARED_REF}"
else
    echo "Cloning cloudflared from ${CLOUDFLARED_REPO}..."
    git clone "${CLOUDFLARED_REPO}" "${UPSTREAM_DIR}"
    git -C "${UPSTREAM_DIR}" checkout "${CLOUDFLARED_REF}" 2>/dev/null || true
fi

UPSTREAM_COMMIT="$(git -C "${UPSTREAM_DIR}" rev-parse --short HEAD)"
UPSTREAM_VERSION="$(git -C "${UPSTREAM_DIR}" describe --tags 2>/dev/null || echo "${CLOUDFLARED_REF}")"
echo "Upstream commit: ${UPSTREAM_COMMIT}"
echo "Upstream version: ${UPSTREAM_VERSION}"

# Step 2: Apply patches if any
echo ""
echo "--- Applying patches ---"
if ls "${PROJECT_ROOT}/build/patches/"*.patch 1>/dev/null 2>&1; then
    for p in "${PROJECT_ROOT}/build/patches/"*.patch; do
        echo "Applying: $(basename "$p")"
        git -C "${UPSTREAM_DIR}" apply "$p"
    done
else
    echo "No patches to apply."
fi

# Step 3: Create output directory
mkdir -p "${OUTPUT_DIR}"

# Step 4: Build via Docker
echo ""
echo "--- Building FIPS Docker image ---"
docker build \
    --platform "${PLATFORM}" \
    -f "${PROJECT_ROOT}/build/Dockerfile.fips" \
    --build-arg VERSION="${VERSION}" \
    --build-arg GIT_COMMIT="${GIT_COMMIT}" \
    --build-arg UPSTREAM_COMMIT="${UPSTREAM_COMMIT}" \
    -t "${DOCKER_IMAGE}:${VERSION}" \
    "${PROJECT_ROOT}"

# Step 5: Extract binaries from container
echo ""
echo "--- Extracting binaries ---"
CONTAINER_ID=$(docker create --platform "${PLATFORM}" "${DOCKER_IMAGE}:${VERSION}")
docker cp "${CONTAINER_ID}:/usr/local/bin/cloudflared" "${OUTPUT_DIR}/cloudflared"
docker cp "${CONTAINER_ID}:/usr/local/bin/selftest" "${OUTPUT_DIR}/selftest"
docker rm "${CONTAINER_ID}"

# Step 6: Run verification scripts
echo ""
echo "--- Running verification ---"
if [[ -x "${PROJECT_ROOT}/scripts/verify-boring.sh" ]]; then
    "${PROJECT_ROOT}/scripts/verify-boring.sh" "${OUTPUT_DIR}/cloudflared" || true
fi

if [[ -x "${PROJECT_ROOT}/scripts/check-fips.sh" ]]; then
    "${PROJECT_ROOT}/scripts/check-fips.sh" "${OUTPUT_DIR}/cloudflared" || true
fi

# Step 7: Generate manifest
echo ""
echo "--- Generating build manifest ---"
export VERSION GIT_COMMIT BUILD_DATE PLATFORM OUTPUT_DIR
export UPSTREAM_COMMIT UPSTREAM_VERSION
if [[ -x "${PROJECT_ROOT}/scripts/generate-manifest.sh" ]]; then
    "${PROJECT_ROOT}/scripts/generate-manifest.sh"
fi

echo ""
echo "=== Build Complete ==="
echo "Binary:   ${OUTPUT_DIR}/cloudflared"
echo "Selftest: ${OUTPUT_DIR}/selftest"
echo "Manifest: ${OUTPUT_DIR}/build-manifest.json"
echo "Upstream: cloudflared ${UPSTREAM_VERSION} (${UPSTREAM_COMMIT})"

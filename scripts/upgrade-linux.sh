#!/usr/bin/env bash
# upgrade-linux.sh — Upgrade cloudflared-fips binaries while preserving config
#
# Usage:
#   sudo ./scripts/upgrade-linux.sh                    # upgrade to latest release
#   sudo ./scripts/upgrade-linux.sh --version v0.7.1   # upgrade to specific version
#   sudo ./scripts/upgrade-linux.sh --from-source      # rebuild from local repo
#   sudo ./scripts/upgrade-linux.sh --dry-run           # show what would happen
#
# This script upgrades binaries and restarts services WITHOUT touching:
#   - /etc/cloudflared-fips/ (config, env, certs, tunnel token)
#   - /var/lib/cloudflared-fips/ (fleet DB, state)
#   - systemd unit files (tunnel tokens, flags)
#
# Also checks for cloudflared (upstream) updates and offers to upgrade.
# Use --skip-cloudflared to skip the upstream check.

set -euo pipefail

# ---------------------------------------------------------------------------
# Config — must match provision-linux.sh
# ---------------------------------------------------------------------------
REPO_OWNER="JongoDB"
REPO_NAME="cloudflared-fips"
INSTALL_DIR="/opt/cloudflared-fips"
CONFIG_DIR="/etc/cloudflared-fips"
BIN_DIR="/usr/local/bin"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
TARGET_VERSION=""
FROM_SOURCE=false
DRY_RUN=false
YES=false
SKIP_CLOUDFLARED=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)          TARGET_VERSION="$2"; shift 2 ;;
        --from-source)      FROM_SOURCE=true; shift ;;
        --dry-run)          DRY_RUN=true; shift ;;
        --yes)              YES=true; shift ;;
        --skip-cloudflared) SKIP_CLOUDFLARED=true; shift ;;
        --help|-h)
            echo "Usage: sudo $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version TAG       Upgrade cloudflared-fips to specific version (e.g., v0.7.1)"
            echo "  --from-source       Rebuild from local repo instead of downloading RPM"
            echo "  --dry-run           Show what would happen without making changes"
            echo "  --yes               Skip confirmation prompt"
            echo "  --skip-cloudflared  Don't check/update the upstream cloudflared binary"
            echo "  --help, -h          Show this help"
            echo ""
            echo "Upgrades cloudflared-fips binaries and optionally the upstream cloudflared"
            echo "binary. Preserves: config, data, systemd units, tunnel tokens, fleet DB."
            exit 0
            ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
fail() { echo -e "${RED}[x]${NC} $*"; exit 1; }
info() { echo -e "${BLUE}[i]${NC} $*"; }

run() {
    if $DRY_RUN; then
        info "Would run: $*"
    else
        "$@"
    fi
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        fail "This script must be run as root (sudo)"
    fi
}

# ---------------------------------------------------------------------------
# Detect current version
# ---------------------------------------------------------------------------
detect_current() {
    CURRENT_VERSION=""
    if [[ -x "${BIN_DIR}/cloudflared-fips-selftest" ]]; then
        # Try to get version from binary
        CURRENT_VERSION=$("${BIN_DIR}/cloudflared-fips-selftest" --version 2>/dev/null || echo "unknown")
    elif command -v rpm &>/dev/null; then
        CURRENT_VERSION=$(rpm -q --qf '%{VERSION}' cloudflared-fips 2>/dev/null || echo "unknown")
    fi

    if [[ -z "$CURRENT_VERSION" || "$CURRENT_VERSION" == "unknown" ]]; then
        # Fall back to build manifest
        if [[ -f "${CONFIG_DIR}/build-manifest.json" ]]; then
            CURRENT_VERSION=$(grep -oP '"version"\s*:\s*"\K[^"]+' "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || echo "unknown")
        fi
    fi

    info "Current version: ${CURRENT_VERSION:-unknown}"
}

# ---------------------------------------------------------------------------
# Detect latest release from GitHub
# ---------------------------------------------------------------------------
detect_latest() {
    if [[ -n "$TARGET_VERSION" ]]; then
        info "Target version: $TARGET_VERSION"
        return
    fi

    if ! command -v curl &>/dev/null; then
        fail "curl is required to check for updates"
    fi

    log "Checking latest release..."
    local api_url="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
    local release_json
    release_json=$(curl -sf "$api_url" 2>/dev/null || echo "")

    if [[ -z "$release_json" ]]; then
        fail "Could not fetch latest release from GitHub. Specify version with --version"
    fi

    TARGET_VERSION=$(echo "$release_json" | grep -oP '"tag_name"\s*:\s*"\K[^"]+' | head -1)

    if [[ -z "$TARGET_VERSION" ]]; then
        fail "Could not parse latest release tag. Specify version with --version"
    fi

    info "Latest release: $TARGET_VERSION"
}

# ---------------------------------------------------------------------------
# Download and install RPM
# ---------------------------------------------------------------------------
upgrade_rpm() {
    local version_num="${TARGET_VERSION#v}"
    local arch
    arch=$(uname -m)

    local rpm_name="cloudflared-fips-${version_num}-1.el9.${arch}.rpm"
    local download_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${TARGET_VERSION}/${rpm_name}"

    log "Downloading: $rpm_name"
    local tmp_rpm="/tmp/${rpm_name}"

    if $DRY_RUN; then
        info "Would download: $download_url"
        info "Would run: rpm -Uvh $tmp_rpm"
        return
    fi

    if ! curl -sLf -o "$tmp_rpm" "$download_url"; then
        # Try without .el9 suffix
        rpm_name="cloudflared-fips-${version_num}-1.${arch}.rpm"
        download_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${TARGET_VERSION}/${rpm_name}"
        log "Retrying: $rpm_name"
        if ! curl -sLf -o "$tmp_rpm" "$download_url"; then
            fail "Could not download RPM from: $download_url"
        fi
    fi

    log "Stopping services..."
    systemctl stop cloudflared-fips.target 2>/dev/null || true
    for svc in dashboard tunnel proxy agent; do
        systemctl stop "cloudflared-fips-${svc}.service" 2>/dev/null || true
    done

    log "Installing RPM (upgrade mode — preserves config)..."
    rpm -Uvh --force "$tmp_rpm"

    rm -f "$tmp_rpm"
}

# ---------------------------------------------------------------------------
# Upgrade from source (git pull + rebuild)
# ---------------------------------------------------------------------------
upgrade_source() {
    if [[ ! -d "${INSTALL_DIR}/.git" ]]; then
        fail "No git repo at ${INSTALL_DIR}. Use --version to download RPM instead."
    fi

    log "Updating source repo..."
    if $DRY_RUN; then
        info "Would run: cd ${INSTALL_DIR} && git pull --ff-only"
        info "Would rebuild binaries and install"
        return
    fi

    cd "$INSTALL_DIR"

    # Fetch and update
    git fetch origin
    if [[ -n "$TARGET_VERSION" ]]; then
        git checkout "$TARGET_VERSION"
    else
        git pull --ff-only
    fi

    # Detect role from env file
    local role="controller"
    if [[ -f "${CONFIG_DIR}/env" ]]; then
        role=$(grep -oP '(?<=^ROLE=).*' "${CONFIG_DIR}/env" 2>/dev/null || echo "controller")
    fi

    # Build dashboard frontend (controller only)
    if [[ "$role" == "controller" ]]; then
        log "Building dashboard frontend..."
        cd dashboard && npm install && npm run build && cd ..
        mkdir -p internal/dashboard/static
        cp -r dashboard/dist/* internal/dashboard/static/
    fi

    # Build binaries
    log "Building binaries for role: ${role}..."
    case "$role" in
        controller) make selftest-bin dashboard-bin agent-bin ;;
        server)     make selftest-bin agent-bin ;;
        proxy)      make selftest-bin fips-proxy-bin agent-bin ;;
        client)     make selftest-bin agent-bin ;;
        *)          make selftest-bin agent-bin ;;
    esac

    # Generate manifest
    make manifest

    # Stop services
    log "Stopping services..."
    systemctl stop cloudflared-fips.target 2>/dev/null || true
    for svc in dashboard tunnel proxy agent; do
        systemctl stop "cloudflared-fips-${svc}.service" 2>/dev/null || true
    done

    # Install binaries (don't touch config or systemd units)
    log "Installing binaries..."
    install -m 0755 build-output/cloudflared-fips-selftest "${BIN_DIR}/"
    case "$role" in
        controller)
            install -m 0755 build-output/cloudflared-fips-dashboard "${BIN_DIR}/"
            install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
            [[ -f build-output/cloudflared-fips-proxy ]] && install -m 0755 build-output/cloudflared-fips-proxy "${BIN_DIR}/"
            ;;
        server)
            install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
            ;;
        proxy)
            install -m 0755 build-output/cloudflared-fips-proxy "${BIN_DIR}/"
            install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
            ;;
        client)
            install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
            ;;
    esac

    # Update build manifest only (not cloudflared-fips.yaml)
    if [[ -f build-output/build-manifest.json ]]; then
        cp build-output/build-manifest.json "${CONFIG_DIR}/build-manifest.json"
    fi
}

# ---------------------------------------------------------------------------
# cloudflared (upstream) — version detection and upgrade
# ---------------------------------------------------------------------------
CF_CURRENT_VERSION=""
CF_LATEST_VERSION=""

detect_cloudflared_current() {
    CF_CURRENT_VERSION=""
    if command -v cloudflared &>/dev/null; then
        # cloudflared version outputs: "cloudflared version 2025.2.1 (built 2025-02-18-...)"
        CF_CURRENT_VERSION=$(cloudflared version 2>/dev/null | grep -oP 'version \K[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "")
    fi

    if [[ -z "$CF_CURRENT_VERSION" ]]; then
        info "cloudflared: not installed"
    else
        info "cloudflared: ${CF_CURRENT_VERSION}"
    fi
}

detect_cloudflared_latest() {
    if [[ -z "$CF_CURRENT_VERSION" ]]; then
        return  # not installed, nothing to check
    fi

    log "Checking latest cloudflared release..."
    local api_url="https://api.github.com/repos/cloudflare/cloudflared/releases/latest"
    local release_json
    release_json=$(curl -sf "$api_url" 2>/dev/null || echo "")

    if [[ -z "$release_json" ]]; then
        warn "Could not check for cloudflared updates (GitHub API unreachable)"
        return
    fi

    CF_LATEST_VERSION=$(echo "$release_json" | grep -oP '"tag_name"\s*:\s*"\K[^"]+' | head -1)

    if [[ -z "$CF_LATEST_VERSION" ]]; then
        warn "Could not parse cloudflared latest release tag"
        return
    fi

    info "cloudflared latest: ${CF_LATEST_VERSION}"
}

upgrade_cloudflared() {
    if [[ -z "$CF_CURRENT_VERSION" || -z "$CF_LATEST_VERSION" ]]; then
        return
    fi

    # Compare versions (strip leading 'v' if present)
    local current="${CF_CURRENT_VERSION#v}"
    local latest="${CF_LATEST_VERSION#v}"

    if [[ "$current" == "$latest" ]]; then
        info "cloudflared is up to date (${current})"
        return
    fi

    echo ""
    warn "cloudflared update available: ${current} → ${latest}"

    if ! $YES && ! $DRY_RUN; then
        read -rp "Update cloudflared to ${latest}? [y/N] " answer
        case "$answer" in
            [yY]|[yY][eE][sS]) ;;
            *) info "Skipping cloudflared update."; return ;;
        esac
    fi

    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
    esac

    # Try Cloudflare's package repo first (RPM-based systems)
    if command -v dnf &>/dev/null; then
        log "Updating cloudflared via dnf..."
        if $DRY_RUN; then
            info "Would run: dnf update -y cloudflared"
        else
            if dnf update -y cloudflared 2>/dev/null; then
                log "cloudflared updated via dnf"
                return
            fi
            warn "dnf update failed, falling back to direct download"
        fi
    elif command -v yum &>/dev/null; then
        log "Updating cloudflared via yum..."
        if $DRY_RUN; then
            info "Would run: yum update -y cloudflared"
        else
            if yum update -y cloudflared 2>/dev/null; then
                log "cloudflared updated via yum"
                return
            fi
            warn "yum update failed, falling back to direct download"
        fi
    fi

    # Direct binary download from GitHub
    local download_url="https://github.com/cloudflare/cloudflared/releases/download/${CF_LATEST_VERSION}/cloudflared-linux-${arch}"
    local tmp_bin="/tmp/cloudflared-upgrade"
    local cf_path
    cf_path=$(command -v cloudflared)

    log "Downloading cloudflared ${CF_LATEST_VERSION}..."
    if $DRY_RUN; then
        info "Would download: ${download_url}"
        info "Would replace: ${cf_path}"
        return
    fi

    if ! curl -sLf -o "$tmp_bin" "$download_url"; then
        warn "Could not download cloudflared from ${download_url}"
        warn "Update cloudflared manually: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
        return
    fi

    chmod +x "$tmp_bin"

    # Verify the download looks like a cloudflared binary
    if ! "$tmp_bin" version &>/dev/null; then
        warn "Downloaded binary does not appear to be valid cloudflared"
        rm -f "$tmp_bin"
        return
    fi

    # Stop tunnel service before replacing binary
    if systemctl is-active cloudflared-fips-tunnel.service &>/dev/null; then
        log "Stopping tunnel service..."
        systemctl stop cloudflared-fips-tunnel.service 2>/dev/null || true
    fi

    # Replace binary
    cp "$tmp_bin" "$cf_path"
    rm -f "$tmp_bin"

    local new_ver
    new_ver=$(cloudflared version 2>/dev/null | grep -oP 'version \K[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    log "cloudflared updated: ${CF_CURRENT_VERSION} → ${new_ver}"
}

# ---------------------------------------------------------------------------
# Restart services
# ---------------------------------------------------------------------------
restart_services() {
    if $DRY_RUN; then
        info "Would restart services"
        return
    fi

    log "Restarting services..."
    systemctl daemon-reload

    # Restart only services that have unit files
    for svc in dashboard tunnel proxy agent; do
        if [[ -f "/etc/systemd/system/cloudflared-fips-${svc}.service" ]]; then
            systemctl start "cloudflared-fips-${svc}.service" 2>/dev/null || {
                warn "Failed to start cloudflared-fips-${svc}"
            }
        fi
    done

    if [[ -f "/etc/systemd/system/cloudflared-fips.target" ]]; then
        systemctl start cloudflared-fips.target 2>/dev/null || true
    fi
}

# ---------------------------------------------------------------------------
# Verify
# ---------------------------------------------------------------------------
verify() {
    if $DRY_RUN; then
        return
    fi

    log "Running self-test..."
    "${BIN_DIR}/cloudflared-fips-selftest" 2>/dev/null || {
        warn "Self-test failed — check logs"
    }
    echo ""

    # Show new version
    local new_version="unknown"
    if [[ -f "${CONFIG_DIR}/build-manifest.json" ]]; then
        new_version=$(grep -oP '"version"\s*:\s*"\K[^"]+' "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || echo "unknown")
    fi

    # Check service status
    log "Service status:"
    for svc in dashboard tunnel proxy agent; do
        if [[ -f "/etc/systemd/system/cloudflared-fips-${svc}.service" ]]; then
            local status
            status=$(systemctl is-active "cloudflared-fips-${svc}.service" 2>/dev/null || echo "inactive")
            if [[ "$status" == "active" ]]; then
                echo -e "  ${GREEN}●${NC} cloudflared-fips-${svc}: active"
            else
                echo -e "  ${RED}●${NC} cloudflared-fips-${svc}: ${status}"
            fi
        fi
    done
    echo ""

    # Show cloudflared version
    if command -v cloudflared &>/dev/null; then
        local cf_ver
        cf_ver=$(cloudflared version 2>/dev/null | grep -oP 'version \K[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
        info "cloudflared version: ${cf_ver}"
    fi

    log "Upgrade complete: ${CURRENT_VERSION:-unknown} → ${new_version}"
}

# ---------------------------------------------------------------------------
# Confirm
# ---------------------------------------------------------------------------
confirm() {
    echo ""
    echo "============================================"
    echo "  cloudflared-fips — Upgrade"
    echo "============================================"
    echo ""
    info "Current:  ${CURRENT_VERSION:-unknown}"
    info "Target:   ${TARGET_VERSION}"
    if $FROM_SOURCE; then
        info "Method:   Rebuild from source (${INSTALL_DIR})"
    else
        info "Method:   RPM upgrade (rpm -Uvh)"
    fi
    echo ""
    if [[ -n "$CF_CURRENT_VERSION" && -n "$CF_LATEST_VERSION" && "$CF_CURRENT_VERSION" != "$CF_LATEST_VERSION" ]]; then
        echo ""
        info "cloudflared: ${CF_CURRENT_VERSION} → ${CF_LATEST_VERSION} (will prompt)"
    fi
    echo ""
    info "Preserved:"
    info "  - Config:       ${CONFIG_DIR}/"
    info "  - Data/DB:      /var/lib/cloudflared-fips/"
    info "  - Systemd units (tunnel tokens, flags)"
    info "  - Env file (API tokens, node ID, etc.)"
    echo ""
    info "Replaced:"
    info "  - Binaries:     ${BIN_DIR}/cloudflared-fips-*"
    info "  - Manifest:     ${CONFIG_DIR}/build-manifest.json"
    echo ""

    if $YES || $DRY_RUN; then
        return 0
    fi

    read -rp "Continue with upgrade? [y/N] " answer
    case "$answer" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) echo "Aborted."; exit 0 ;;
    esac
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    check_root
    detect_current

    if $FROM_SOURCE; then
        if [[ -z "$TARGET_VERSION" ]]; then
            TARGET_VERSION="latest (HEAD)"
        fi
    else
        detect_latest
    fi

    # Detect cloudflared (upstream) version
    if ! $SKIP_CLOUDFLARED; then
        detect_cloudflared_current
        detect_cloudflared_latest
    fi

    # Check if already on target version
    local target_num="${TARGET_VERSION#v}"
    if [[ "$CURRENT_VERSION" == "$target_num" || "$CURRENT_VERSION" == "$TARGET_VERSION" ]]; then
        info "Already on version ${TARGET_VERSION}. Nothing to do."
        info "Use --from-source to force rebuild, or specify a different --version."
        # Still check cloudflared even if our binaries are current
        if ! $SKIP_CLOUDFLARED; then
            upgrade_cloudflared
        fi
        exit 0
    fi

    confirm

    if $FROM_SOURCE; then
        upgrade_source
    else
        upgrade_rpm
    fi

    # Offer cloudflared update after our binaries are upgraded
    if ! $SKIP_CLOUDFLARED; then
        upgrade_cloudflared
    fi

    restart_services
    verify
}

main

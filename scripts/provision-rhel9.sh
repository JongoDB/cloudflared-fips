#!/usr/bin/env bash
# provision-rhel9.sh — Full RHEL 9 / Rocky 9 / Alma 9 provisioning for cloudflared-fips
#
# Usage:
#   curl -sL <raw-url> | sudo bash                  # one-liner
#   sudo ./scripts/provision-rhel9.sh                # from repo
#   sudo ./scripts/provision-rhel9.sh --no-fips      # skip FIPS mode (dev/test only)
#   sudo ./scripts/provision-rhel9.sh --with-cf      # prompt for Cloudflare API creds
#
# This script is idempotent — safe to re-run after reboot.
# After enabling FIPS mode it will reboot automatically. Run again after reboot
# to continue from where it left off.

set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
GO_VERSION="1.24.0"
REPO_URL="https://github.com/JongoDB/cloudflared-fips.git"
INSTALL_DIR="/opt/cloudflared-fips"
CONFIG_DIR="/etc/cloudflared-fips"
BIN_DIR="/usr/local/bin"
SERVICE_USER="cloudflared"
DASHBOARD_ADDR="127.0.0.1:8080"
MARKER="/var/tmp/.cloudflared-fips-provision-phase"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
SKIP_FIPS=false
WITH_CF=false
for arg in "$@"; do
    case "$arg" in
        --no-fips)  SKIP_FIPS=true ;;
        --with-cf)  WITH_CF=true ;;
        --help|-h)
            echo "Usage: sudo $0 [--no-fips] [--with-cf]"
            echo "  --no-fips   Skip FIPS mode enablement (dev/test only)"
            echo "  --with-cf   Prompt for Cloudflare API credentials"
            exit 0
            ;;
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
fail() { echo -e "${RED}[✗]${NC} $*"; exit 1; }
info() { echo -e "${BLUE}[i]${NC} $*"; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        fail "This script must be run as root (sudo)"
    fi
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *)       fail "Unsupported architecture: $(uname -m)" ;;
    esac
}

# ---------------------------------------------------------------------------
# Phase 1: FIPS mode + reboot
# ---------------------------------------------------------------------------
phase1_fips() {
    if [[ "$SKIP_FIPS" == "true" ]]; then
        warn "Skipping FIPS mode (--no-fips). NOT suitable for production."
        return 0
    fi

    # Already enabled?
    if [[ -f /proc/sys/crypto/fips_enabled ]] && [[ "$(cat /proc/sys/crypto/fips_enabled)" == "1" ]]; then
        log "FIPS mode already enabled"
        return 0
    fi

    log "Installing crypto-policies and enabling FIPS mode..."
    dnf install -y crypto-policies-scripts

    log "Enabling FIPS mode (requires reboot)..."
    fips-mode-setup --enable

    # Mark that we need to continue after reboot
    echo "2" > "$MARKER"

    warn "============================================"
    warn "  FIPS mode enabled. Rebooting in 5 seconds."
    warn "  Re-run this script after reboot to continue."
    warn "============================================"
    sleep 5
    reboot
}

# ---------------------------------------------------------------------------
# Phase 2: Install dependencies, build, deploy
# ---------------------------------------------------------------------------
phase2_build() {
    local ARCH
    ARCH=$(detect_arch)

    # --- Verify FIPS if expected ---
    if [[ "$SKIP_FIPS" == "false" ]]; then
        if [[ -f /proc/sys/crypto/fips_enabled ]] && [[ "$(cat /proc/sys/crypto/fips_enabled)" == "1" ]]; then
            log "FIPS mode verified: enabled"
        else
            fail "FIPS mode is NOT enabled. Run: sudo fips-mode-setup --enable && sudo reboot"
        fi
    fi

    # --- Build dependencies ---
    log "Installing build dependencies..."
    dnf install -y gcc gcc-c++ make git

    # --- Go 1.24 ---
    if command -v go &>/dev/null && go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
        log "Go ${GO_VERSION} already installed"
    else
        log "Installing Go ${GO_VERSION} (${ARCH})..."
        curl -sLO "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz"
        rm -rf /usr/local/go
        tar -C /usr/local -xzf "go${GO_VERSION}.linux-${ARCH}.tar.gz"
        rm -f "go${GO_VERSION}.linux-${ARCH}.tar.gz"
    fi
    export PATH="/usr/local/go/bin:$PATH"
    log "Go version: $(go version)"

    # --- Node.js 20 LTS (Vite/Tailwind require Node 20+; RHEL ships v16) ---
    NODE_VERSION="20.19.0"
    if command -v node &>/dev/null && node --version 2>/dev/null | grep -qE '^v(2[0-9]|[3-9][0-9])\.'; then
        log "Node.js already installed: $(node --version)"
    else
        log "Installing Node.js ${NODE_VERSION} (${ARCH})..."
        # Remove old RHEL node if present
        dnf remove -y nodejs npm 2>/dev/null || true
        curl -sLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz"
        rm -rf /usr/local/node
        mkdir -p /usr/local/node
        tar -C /usr/local/node --strip-components=1 -xJf "node-v${NODE_VERSION}-linux-x64.tar.xz"
        rm -f "node-v${NODE_VERSION}-linux-x64.tar.xz"
        # Symlink so node/npm are on PATH
        ln -sf /usr/local/node/bin/node /usr/local/bin/node
        ln -sf /usr/local/node/bin/npm /usr/local/bin/npm
        ln -sf /usr/local/node/bin/npx /usr/local/bin/npx
    fi
    export PATH="/usr/local/node/bin:$PATH"
    log "Node.js version: $(node --version)"

    # --- Clone or update repo ---
    if [[ -d "${INSTALL_DIR}/.git" ]]; then
        log "Updating existing repo in ${INSTALL_DIR}..."
        cd "$INSTALL_DIR"
        git pull --ff-only
    else
        log "Cloning repo to ${INSTALL_DIR}..."
        git clone "$REPO_URL" "$INSTALL_DIR"
        cd "$INSTALL_DIR"
    fi

    # --- Build dashboard frontend first (required for go:embed in dashboard binary) ---
    log "Building dashboard frontend..."
    cd dashboard && npm install && npm run build && cd ..
    mkdir -p internal/dashboard/static
    cp -r dashboard/dist/* internal/dashboard/static/

    # --- Build all binaries (dashboard binary now has embedded frontend) ---
    log "Building all binaries (GOEXPERIMENT=boringcrypto CGO_ENABLED=1)..."
    make build-all

    # --- Generate build manifest ---
    log "Generating build manifest..."
    make manifest

    # --- Validate ---
    log "Running FIPS validation..."
    ./scripts/verify-boring.sh ./build-output/cloudflared-fips-selftest
    echo ""
    ./scripts/check-fips.sh ./build-output/cloudflared-fips-selftest

    log "Running self-test..."
    ./build-output/cloudflared-fips-selftest 2>/dev/null
    echo ""

    # --- Run Go tests ---
    log "Running Go test suite..."
    make test
    echo ""

    log "Build and validation complete."
}

# ---------------------------------------------------------------------------
# Phase 3: Install and configure
# ---------------------------------------------------------------------------
phase3_install() {
    cd "$INSTALL_DIR"

    # --- Create service user ---
    if id "$SERVICE_USER" &>/dev/null; then
        log "Service user '${SERVICE_USER}' already exists"
    else
        log "Creating service user '${SERVICE_USER}'..."
        useradd -r -s /sbin/nologin "$SERVICE_USER"
    fi

    # --- Install binaries ---
    log "Installing binaries to ${BIN_DIR}..."
    install -m 0755 build-output/cloudflared-fips-selftest  "${BIN_DIR}/"
    install -m 0755 build-output/cloudflared-fips-dashboard "${BIN_DIR}/"
    install -m 0755 build-output/cloudflared-fips-tui       "${BIN_DIR}/"
    install -m 0755 build-output/cloudflared-fips-proxy     "${BIN_DIR}/"

    # --- Config directory ---
    mkdir -p "$CONFIG_DIR"
    cp -n build-output/build-manifest.json "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || true
    cp -n configs/cloudflared-fips.yaml "${CONFIG_DIR}/cloudflared-fips.yaml" 2>/dev/null || true

    # --- Populate binary_sha256 in build manifest ---
    DASHBOARD_HASH=$(sha256sum "${BIN_DIR}/cloudflared-fips-dashboard" | awk '{print $1}')
    if command -v python3 &>/dev/null && [[ -f "${CONFIG_DIR}/build-manifest.json" ]]; then
        python3 -c "
import json, sys
with open('${CONFIG_DIR}/build-manifest.json', 'r') as f:
    m = json.load(f)
m['binary_sha256'] = '${DASHBOARD_HASH}'
with open('${CONFIG_DIR}/build-manifest.json', 'w') as f:
    json.dump(m, f, indent=2)
"
        log "Binary SHA-256 written to manifest: ${DASHBOARD_HASH:0:16}..."
    fi

    # --- Generate SBOM ---
    if [[ -f "${INSTALL_DIR}/scripts/generate-sbom.sh" ]]; then
        log "Generating SBOM..."
        cd "$INSTALL_DIR"
        bash scripts/generate-sbom.sh 2>/dev/null || true
        for f in sbom.cyclonedx.json sbom.spdx.json; do
            [[ -f "$f" ]] && cp "$f" "${CONFIG_DIR}/"
        done
        cd - >/dev/null
    fi

    chown -R "${SERVICE_USER}:" "$CONFIG_DIR"

    # --- Environment file for Cloudflare API (optional) ---
    ENV_FILE="${CONFIG_DIR}/env"
    if [[ "$WITH_CF" == "true" && ! -f "$ENV_FILE" ]]; then
        log "Setting up Cloudflare API credentials..."
        echo ""
        read -rp "  CF_API_TOKEN: " CF_API_TOKEN
        read -rp "  CF_ZONE_ID: " CF_ZONE_ID
        read -rp "  CF_ACCOUNT_ID: " CF_ACCOUNT_ID
        read -rp "  CF_TUNNEL_ID: " CF_TUNNEL_ID
        echo ""

        cat > "$ENV_FILE" <<ENVEOF
CF_API_TOKEN=${CF_API_TOKEN}
CF_ZONE_ID=${CF_ZONE_ID}
CF_ACCOUNT_ID=${CF_ACCOUNT_ID}
CF_TUNNEL_ID=${CF_TUNNEL_ID}
ENVEOF
        chmod 0600 "$ENV_FILE"
        chown "${SERVICE_USER}:" "$ENV_FILE"
        log "Credentials written to ${ENV_FILE} (mode 0600)"
    elif [[ -f "$ENV_FILE" ]]; then
        info "Existing env file found at ${ENV_FILE}, skipping"
    else
        # Create empty env file so systemd doesn't complain
        touch "$ENV_FILE"
        chmod 0600 "$ENV_FILE"
        chown "${SERVICE_USER}:" "$ENV_FILE"
    fi

    # --- systemd unit ---
    log "Installing systemd service..."
    cat > /etc/systemd/system/cloudflared-fips-dashboard.service <<'UNITEOF'
[Unit]
Description=cloudflared-fips FIPS 140-3 Compliance Dashboard
Documentation=https://github.com/JongoDB/cloudflared-fips
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=cloudflared
Group=cloudflared
EnvironmentFile=-/etc/cloudflared-fips/env
ExecStartPre=/usr/local/bin/cloudflared-fips-selftest
ExecStart=/usr/local/bin/cloudflared-fips-dashboard \
  --addr 127.0.0.1:8080 \
  --manifest /etc/cloudflared-fips/build-manifest.json \
  --config /etc/cloudflared-fips/cloudflared-fips.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-fips

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadOnlyPaths=/etc/cloudflared-fips
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes

[Install]
WantedBy=multi-user.target
UNITEOF

    systemctl daemon-reload
    systemctl enable cloudflared-fips-dashboard

    # --- Firewall (allow 8080 if user wants external access later) ---
    if command -v firewall-cmd &>/dev/null; then
        info "To expose the dashboard externally (not recommended for prod):"
        info "  sudo firewall-cmd --add-port=8080/tcp --permanent && sudo firewall-cmd --reload"
    fi

    log "Installation complete."
}

# ---------------------------------------------------------------------------
# Phase 4: Start and verify
# ---------------------------------------------------------------------------
phase4_start() {
    log "Starting cloudflared-fips-dashboard service..."
    systemctl start cloudflared-fips-dashboard
    sleep 2

    if systemctl is-active --quiet cloudflared-fips-dashboard; then
        log "Service is running"
    else
        fail "Service failed to start. Check: journalctl -u cloudflared-fips-dashboard -n 50"
    fi

    log "Verifying endpoints..."
    echo ""

    info "Health check:"
    curl -s http://127.0.0.1:8080/api/v1/health 2>/dev/null || warn "Could not reach health endpoint"
    echo ""

    info "FIPS backend:"
    curl -s http://127.0.0.1:8080/api/v1/backend 2>/dev/null || warn "Could not reach backend endpoint"
    echo ""

    info "Migration status:"
    curl -s http://127.0.0.1:8080/api/v1/migration 2>/dev/null || warn "Could not reach migration endpoint"
    echo ""

    # --- Add PATH for all users ---
    if ! grep -q '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
        echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/go.sh
        chmod 0644 /etc/profile.d/go.sh
    fi

    # --- Cleanup marker ---
    rm -f "$MARKER"

    echo ""
    log "============================================"
    log "  cloudflared-fips deployed successfully!"
    log "============================================"
    echo ""
    info "Dashboard:    http://127.0.0.1:8080"
    info "Self-test:    cloudflared-fips-selftest"
    info "TUI monitor:  cloudflared-fips-tui status"
    info "Service logs: journalctl -u cloudflared-fips-dashboard -f"
    echo ""
    if [[ ! -s "${CONFIG_DIR}/env" ]]; then
        info "To add Cloudflare API integration later:"
        info "  1. Edit ${CONFIG_DIR}/env with your CF_API_TOKEN, CF_ZONE_ID, etc."
        info "  2. sudo systemctl restart cloudflared-fips-dashboard"
    fi
    echo ""
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    check_root

    echo ""
    echo "============================================"
    echo "  cloudflared-fips provisioning"
    echo "  Target: RHEL 9 / Rocky 9 / AlmaLinux 9"
    echo "============================================"
    echo ""

    # Detect if we're resuming after a FIPS reboot
    if [[ -f "$MARKER" ]]; then
        info "Resuming after reboot..."
    fi

    phase1_fips
    phase2_build
    phase3_install
    phase4_start
}

main "$@"

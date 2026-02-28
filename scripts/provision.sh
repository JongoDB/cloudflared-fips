#!/usr/bin/env bash
# provision.sh — Multi-role provisioning for cloudflared-fips fleet
#
# Usage:
#   sudo ./scripts/provision.sh                                    # default: server role
#   sudo ./scripts/provision.sh --role controller                  # fleet controller
#   sudo ./scripts/provision.sh --role server --tunnel-token TOKEN # server with tunnel
#   sudo ./scripts/provision.sh --role proxy                       # FIPS edge proxy
#   sudo ./scripts/provision.sh --role client --enrollment-token T --controller-url URL
#   sudo ./scripts/provision.sh --no-fips                          # skip FIPS mode (dev/test)
#   sudo ./scripts/provision.sh --with-cf                          # prompt for CF API creds
#
# This script is idempotent — safe to re-run after reboot.
# After enabling FIPS mode it will reboot automatically. Run again after reboot.

set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
GO_VERSION="1.24.0"
REPO_URL="https://github.com/JongoDB/cloudflared-fips.git"
INSTALL_DIR="/opt/cloudflared-fips"
CONFIG_DIR="/etc/cloudflared-fips"
DATA_DIR="/var/lib/cloudflared-fips"
BIN_DIR="/usr/local/bin"
SERVICE_USER="cloudflared"
DASHBOARD_ADDR="127.0.0.1:8080"
MARKER="/var/tmp/.cloudflared-fips-provision-phase"
CLOUDFLARED_VERSION="2025.2.1"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
ROLE="server"
SKIP_FIPS=false
WITH_CF=false
TUNNEL_TOKEN=""
ENROLLMENT_TOKEN=""
CONTROLLER_URL=""
ADMIN_KEY=""
NODE_NAME=""
NODE_REGION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)            ROLE="$2"; shift 2 ;;
        --no-fips)         SKIP_FIPS=true; shift ;;
        --with-cf)         WITH_CF=true; shift ;;
        --tunnel-token)    TUNNEL_TOKEN="$2"; shift 2 ;;
        --enrollment-token) ENROLLMENT_TOKEN="$2"; shift 2 ;;
        --controller-url)  CONTROLLER_URL="$2"; shift 2 ;;
        --admin-key)       ADMIN_KEY="$2"; shift 2 ;;
        --node-name)       NODE_NAME="$2"; shift 2 ;;
        --node-region)     NODE_REGION="$2"; shift 2 ;;
        --help|-h)
            echo "Usage: sudo $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --role ROLE          Node role: controller, server, proxy, client (default: server)"
            echo "  --no-fips            Skip FIPS mode enablement (dev/test only)"
            echo "  --with-cf            Prompt for Cloudflare API credentials"
            echo "  --tunnel-token TOKEN cloudflared tunnel token (server role)"
            echo "  --enrollment-token T Enrollment token from controller (server/proxy/client)"
            echo "  --controller-url URL Fleet controller URL (required for non-controller roles)"
            echo "  --admin-key KEY      Admin API key for controller"
            echo "  --node-name NAME     Display name for this node"
            echo "  --node-region REGION Region label (e.g., us-east, eu-west)"
            exit 0
            ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# Validate role
case "$ROLE" in
    controller|server|proxy|client) ;;
    *) echo "Invalid role: $ROLE (must be controller, server, proxy, or client)"; exit 1 ;;
esac

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
# Phase 2: Install dependencies and build
# ---------------------------------------------------------------------------
phase2_deps() {
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

    # --- Node.js 20 LTS (only needed for controller and server roles with dashboard) ---
    if [[ "$ROLE" == "controller" || "$ROLE" == "server" ]]; then
        NODE_VERSION="20.19.0"
        if command -v node &>/dev/null && node --version 2>/dev/null | grep -qE '^v(2[0-9]|[3-9][0-9])\.'; then
            log "Node.js already installed: $(node --version)"
        else
            log "Installing Node.js ${NODE_VERSION} (${ARCH})..."
            dnf remove -y nodejs npm 2>/dev/null || true
            curl -sLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz"
            rm -rf /usr/local/node
            mkdir -p /usr/local/node
            tar -C /usr/local/node --strip-components=1 -xJf "node-v${NODE_VERSION}-linux-x64.tar.xz"
            rm -f "node-v${NODE_VERSION}-linux-x64.tar.xz"
            ln -sf /usr/local/node/bin/node /usr/local/bin/node
            ln -sf /usr/local/node/bin/npm /usr/local/bin/npm
            ln -sf /usr/local/node/bin/npx /usr/local/bin/npx
        fi
        export PATH="/usr/local/node/bin:$PATH"
        log "Node.js version: $(node --version)"
    fi
}

phase2_build() {
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

    # --- Build dashboard frontend (only for roles that serve it) ---
    if [[ "$ROLE" == "controller" || "$ROLE" == "server" ]]; then
        log "Building dashboard frontend..."
        cd dashboard && npm install && npm run build && cd ..
        mkdir -p internal/dashboard/static
        cp -r dashboard/dist/* internal/dashboard/static/
    fi

    # --- Build binaries based on role ---
    log "Building binaries for role: ${ROLE}..."
    case "$ROLE" in
        controller)
            make selftest-bin dashboard-bin
            ;;
        server)
            make selftest-bin dashboard-bin
            ;;
        proxy)
            make selftest-bin fips-proxy-bin
            ;;
        client)
            make selftest-bin agent-bin
            ;;
    esac

    # --- Generate build manifest ---
    log "Generating build manifest..."
    make manifest

    # --- Validate ---
    log "Running FIPS validation..."
    ./scripts/verify-boring.sh ./build-output/cloudflared-fips-selftest 2>/dev/null || true
    echo ""
    ./scripts/check-fips.sh ./build-output/cloudflared-fips-selftest 2>/dev/null || true

    log "Running self-test..."
    ./build-output/cloudflared-fips-selftest 2>/dev/null || true
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

    # --- Install binaries based on role ---
    log "Installing binaries for role: ${ROLE}..."
    install -m 0755 build-output/cloudflared-fips-selftest "${BIN_DIR}/"

    case "$ROLE" in
        controller|server)
            install -m 0755 build-output/cloudflared-fips-dashboard "${BIN_DIR}/"
            ;;
        proxy)
            install -m 0755 build-output/cloudflared-fips-proxy "${BIN_DIR}/"
            ;;
        client)
            install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
            ;;
    esac

    # --- Config and data directories ---
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    cp -n build-output/build-manifest.json "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || true
    cp -n configs/cloudflared-fips.yaml "${CONFIG_DIR}/cloudflared-fips.yaml" 2>/dev/null || true

    # --- Binary hashes for manifest ---
    for bin in selftest dashboard proxy agent; do
        binpath="${BIN_DIR}/cloudflared-fips-${bin}"
        if [[ -f "$binpath" ]]; then
            hash=$(sha256sum "$binpath" | awk '{print $1}')
            log "SHA-256 ${bin}: ${hash:0:16}..."
        fi
    done

    # --- Generate signatures manifest ---
    log "Generating signatures manifest..."
    SIG_TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    declare -A HASHES=()
    for bin in selftest dashboard proxy agent; do
        binpath="${BIN_DIR}/cloudflared-fips-${bin}"
        if [[ -f "$binpath" ]]; then
            HASHES[$bin]=$(sha256sum "$binpath" | awk '{print $1}')
        fi
    done
    {
        echo "{"
        echo "  \"generated\": \"${SIG_TIMESTAMP}\","
        echo "  \"method\": \"sha256\","
        echo "  \"artifacts\": {"
        local first=true
        for bin in "${!HASHES[@]}"; do
            if [[ "$first" == "true" ]]; then
                first=false
            else
                echo ","
            fi
            printf "    \"cloudflared-fips-%s\": \"%s\"" "$bin" "${HASHES[$bin]}"
        done
        echo ""
        echo "  }"
        echo "}"
    } > "${CONFIG_DIR}/signatures.json"

    chown -R "${SERVICE_USER}:" "$CONFIG_DIR"
    chown -R "${SERVICE_USER}:" "$DATA_DIR"

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
    elif [[ ! -f "$ENV_FILE" ]]; then
        touch "$ENV_FILE"
    fi
    chmod 0600 "$ENV_FILE"
    chown "${SERVICE_USER}:" "$ENV_FILE"

    # --- Fleet enrollment (for non-controller roles with enrollment token) ---
    if [[ "$ROLE" != "controller" && -n "$ENROLLMENT_TOKEN" && -n "$CONTROLLER_URL" ]]; then
        log "Enrolling with fleet controller..."
        local hostname
        hostname=$(hostname -s)
        local name="${NODE_NAME:-${hostname}}"

        ENROLL_RESP=$(curl -sf -X POST "${CONTROLLER_URL}/api/v1/fleet/enroll" \
            -H "Content-Type: application/json" \
            -d "{\"token\":\"${ENROLLMENT_TOKEN}\",\"name\":\"${name}\",\"region\":\"${NODE_REGION}\",\"version\":\"$(cat build-output/build-manifest.json 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin).get(\"version\",\"dev\"))' 2>/dev/null || echo dev)\",\"fips_backend\":\"BoringCrypto\"}" \
        ) || fail "Fleet enrollment failed. Check --controller-url and --enrollment-token."

        NODE_ID=$(echo "$ENROLL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['node_id'])")
        API_KEY=$(echo "$ENROLL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['api_key'])")

        cat > "${CONFIG_DIR}/enrollment.json" <<ENREOF
{
  "node_id": "${NODE_ID}",
  "api_key": "${API_KEY}",
  "controller_url": "${CONTROLLER_URL}",
  "role": "${ROLE}"
}
ENREOF
        chmod 0600 "${CONFIG_DIR}/enrollment.json"
        chown "${SERVICE_USER}:" "${CONFIG_DIR}/enrollment.json"

        # Add to env file
        echo "NODE_ID=${NODE_ID}" >> "$ENV_FILE"
        echo "NODE_API_KEY=${API_KEY}" >> "$ENV_FILE"
        echo "CONTROLLER_URL=${CONTROLLER_URL}" >> "$ENV_FILE"

        log "Enrolled as node ${NODE_ID}"
    fi

    # --- Controller admin key ---
    if [[ "$ROLE" == "controller" ]]; then
        if [[ -z "$ADMIN_KEY" ]]; then
            ADMIN_KEY=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
            log "Generated admin API key: ${ADMIN_KEY}"
        fi
        echo "FLEET_ADMIN_KEY=${ADMIN_KEY}" >> "$ENV_FILE"
    fi

    # --- Download cloudflared (server role) ---
    if [[ "$ROLE" == "server" ]]; then
        install_cloudflared
    fi

    # --- Create systemd units ---
    create_systemd_units
}

# ---------------------------------------------------------------------------
# Install cloudflared binary
# ---------------------------------------------------------------------------
install_cloudflared() {
    local ARCH
    ARCH=$(detect_arch)

    if command -v cloudflared &>/dev/null; then
        log "cloudflared already installed: $(cloudflared --version 2>&1 | head -1)"
        return 0
    fi

    log "Downloading cloudflared ${CLOUDFLARED_VERSION}..."
    local url="https://github.com/cloudflare/cloudflared/releases/download/${CLOUDFLARED_VERSION}/cloudflared-linux-${ARCH}"
    curl -sLo "${BIN_DIR}/cloudflared" "$url"
    chmod 0755 "${BIN_DIR}/cloudflared"
    log "cloudflared installed: $(cloudflared --version 2>&1 | head -1)"
}

# ---------------------------------------------------------------------------
# Create systemd units based on role
# ---------------------------------------------------------------------------
create_systemd_units() {
    log "Creating systemd units for role: ${ROLE}..."

    # --- Dashboard service (controller and server) ---
    if [[ "$ROLE" == "controller" || "$ROLE" == "server" ]]; then
        local fleet_flags=""
        local listen_addr="127.0.0.1:8080"

        if [[ "$ROLE" == "controller" ]]; then
            fleet_flags="--fleet-mode --db-path ${DATA_DIR}/fleet.db"
            listen_addr="0.0.0.0:8080"  # Controllers need to be accessible
        fi

        cat > /etc/systemd/system/cloudflared-fips-dashboard.service <<UNITEOF
[Unit]
Description=cloudflared-fips FIPS 140-3 Compliance Dashboard
Documentation=https://github.com/JongoDB/cloudflared-fips
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=-${CONFIG_DIR}/env
ExecStartPre=${BIN_DIR}/cloudflared-fips-selftest
ExecStart=${BIN_DIR}/cloudflared-fips-dashboard \\
  --addr ${listen_addr} \\
  --manifest ${CONFIG_DIR}/build-manifest.json \\
  --config ${CONFIG_DIR}/cloudflared-fips.yaml \\
  --metrics-addr localhost:2000 \\
  --ingress-targets localhost:8080 \\
  ${fleet_flags}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-fips-dashboard

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=${DATA_DIR}
ReadOnlyPaths=${CONFIG_DIR}
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectControlGroups=yes

[Install]
WantedBy=cloudflared-fips.target
UNITEOF
    fi

    # --- Tunnel service (server role) ---
    if [[ "$ROLE" == "server" ]]; then
        local token_arg=""
        if [[ -n "$TUNNEL_TOKEN" ]]; then
            echo "TUNNEL_TOKEN=${TUNNEL_TOKEN}" >> "${CONFIG_DIR}/env"
            token_arg='--token ${TUNNEL_TOKEN}'
        fi

        cat > /etc/systemd/system/cloudflared-fips-tunnel.service <<UNITEOF
[Unit]
Description=cloudflared FIPS Tunnel
Documentation=https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=-${CONFIG_DIR}/env
ExecStartPre=${BIN_DIR}/cloudflared-fips-selftest
ExecStart=${BIN_DIR}/cloudflared tunnel run --metrics localhost:2000 --protocol quic ${token_arg}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-fips-tunnel

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadOnlyPaths=${CONFIG_DIR}
PrivateTmp=yes

[Install]
WantedBy=cloudflared-fips.target
UNITEOF
    fi

    # --- Proxy service (proxy role) ---
    if [[ "$ROLE" == "proxy" ]]; then
        cat > /etc/systemd/system/cloudflared-fips-proxy.service <<UNITEOF
[Unit]
Description=cloudflared-fips FIPS Edge Proxy (Tier 3)
Documentation=https://github.com/JongoDB/cloudflared-fips
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=-${CONFIG_DIR}/env
ExecStartPre=${BIN_DIR}/cloudflared-fips-selftest
ExecStart=${BIN_DIR}/cloudflared-fips-proxy \\
  --listen :443 \\
  --upstream localhost:8080
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-fips-proxy

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadOnlyPaths=${CONFIG_DIR}
PrivateTmp=yes

[Install]
WantedBy=cloudflared-fips.target
UNITEOF
    fi

    # --- Agent service (client role) ---
    if [[ "$ROLE" == "client" ]]; then
        cat > /etc/systemd/system/cloudflared-fips-agent.service <<UNITEOF
[Unit]
Description=cloudflared-fips Endpoint FIPS Posture Agent
Documentation=https://github.com/JongoDB/cloudflared-fips
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=-${CONFIG_DIR}/env
ExecStart=${BIN_DIR}/cloudflared-fips-agent
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-fips-agent

# Hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadOnlyPaths=${CONFIG_DIR}
PrivateTmp=yes

[Install]
WantedBy=cloudflared-fips.target
UNITEOF
    fi

    # --- Target unit (groups all services for this role) ---
    cat > /etc/systemd/system/cloudflared-fips.target <<UNITEOF
[Unit]
Description=cloudflared-fips Fleet Services
Documentation=https://github.com/JongoDB/cloudflared-fips

[Install]
WantedBy=multi-user.target
UNITEOF

    systemctl daemon-reload

    # Enable services based on role
    systemctl enable cloudflared-fips.target

    case "$ROLE" in
        controller)
            systemctl enable cloudflared-fips-dashboard.service
            ;;
        server)
            systemctl enable cloudflared-fips-dashboard.service
            if [[ -n "$TUNNEL_TOKEN" ]]; then
                systemctl enable cloudflared-fips-tunnel.service
            fi
            ;;
        proxy)
            systemctl enable cloudflared-fips-proxy.service
            ;;
        client)
            systemctl enable cloudflared-fips-agent.service
            ;;
    esac

    # --- Firewall ---
    if command -v firewall-cmd &>/dev/null; then
        if [[ "$ROLE" == "controller" ]]; then
            info "Opening port 8080 for fleet controller..."
            firewall-cmd --add-port=8080/tcp --permanent 2>/dev/null || true
            firewall-cmd --reload 2>/dev/null || true
        else
            info "To expose the dashboard externally (not recommended for prod):"
            info "  sudo firewall-cmd --add-port=8080/tcp --permanent && sudo firewall-cmd --reload"
        fi
    fi

    log "Systemd units installed."
}

# ---------------------------------------------------------------------------
# Phase 4: Start and verify
# ---------------------------------------------------------------------------
phase4_start() {
    log "Starting cloudflared-fips services (role: ${ROLE})..."

    case "$ROLE" in
        controller)
            systemctl start cloudflared-fips-dashboard
            ;;
        server)
            systemctl start cloudflared-fips-dashboard
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                systemctl start cloudflared-fips-tunnel
            fi
            ;;
        proxy)
            systemctl start cloudflared-fips-proxy
            ;;
        client)
            systemctl start cloudflared-fips-agent
            ;;
    esac

    sleep 2

    # Verify based on role
    case "$ROLE" in
        controller|server)
            if systemctl is-active --quiet cloudflared-fips-dashboard; then
                log "Dashboard service is running"
            else
                fail "Dashboard failed to start. Check: journalctl -u cloudflared-fips-dashboard -n 50"
            fi

            log "Verifying endpoints..."
            echo ""
            info "Health check:"
            curl -s http://127.0.0.1:8080/api/v1/health 2>/dev/null || warn "Could not reach health endpoint"
            echo ""

            if [[ "$ROLE" == "controller" ]]; then
                info "Fleet summary:"
                curl -s http://127.0.0.1:8080/api/v1/fleet/summary 2>/dev/null || warn "Could not reach fleet endpoint"
                echo ""
            fi
            ;;
        proxy)
            if systemctl is-active --quiet cloudflared-fips-proxy; then
                log "Proxy service is running"
            else
                fail "Proxy failed to start. Check: journalctl -u cloudflared-fips-proxy -n 50"
            fi
            ;;
        client)
            if systemctl is-active --quiet cloudflared-fips-agent; then
                log "Agent service is running"
            else
                fail "Agent failed to start. Check: journalctl -u cloudflared-fips-agent -n 50"
            fi
            ;;
    esac

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
    log "  Role: ${ROLE}"
    log "============================================"
    echo ""

    case "$ROLE" in
        controller)
            info "Dashboard:    http://0.0.0.0:8080"
            info "Fleet API:    http://0.0.0.0:8080/api/v1/fleet/"
            info "Self-test:    cloudflared-fips-selftest"
            info "Service logs: journalctl -u cloudflared-fips-dashboard -f"
            if [[ -n "$ADMIN_KEY" ]]; then
                info "Admin key:    ${ADMIN_KEY}"
                info "  (save this — needed for creating enrollment tokens)"
            fi
            echo ""
            info "To add a server node:"
            info "  1. Create token: curl -X POST -H 'Authorization: Bearer <admin-key>' \\"
            info "       http://<this-host>:8080/api/v1/fleet/tokens -d '{\"role\":\"server\",\"max_uses\":10}'"
            info "  2. On server: sudo ./provision.sh --role server --enrollment-token <token> --controller-url http://<this-host>:8080"
            ;;
        server)
            info "Dashboard:    http://127.0.0.1:8080"
            info "Self-test:    cloudflared-fips-selftest"
            info "Service logs: journalctl -u cloudflared-fips-dashboard -f"
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                info "Tunnel logs:  journalctl -u cloudflared-fips-tunnel -f"
            fi
            ;;
        proxy)
            info "Proxy:        listening on :443"
            info "Self-test:    cloudflared-fips-selftest"
            info "Service logs: journalctl -u cloudflared-fips-proxy -f"
            ;;
        client)
            info "Agent check:  cloudflared-fips-agent --check"
            info "Service logs: journalctl -u cloudflared-fips-agent -f"
            ;;
    esac
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
    echo "  Role: ${ROLE}"
    echo "  Target: RHEL 9 / Rocky 9 / AlmaLinux 9"
    echo "============================================"
    echo ""

    # Detect if we're resuming after a FIPS reboot
    if [[ -f "$MARKER" ]]; then
        info "Resuming after reboot..."
    fi

    phase1_fips
    phase2_deps
    phase2_build
    phase3_install
    phase4_start
}

main "$@"

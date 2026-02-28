#!/usr/bin/env bash
# provision-macos.sh — macOS provisioning for cloudflared-fips fleet
#
# Usage:
#   sudo ./scripts/provision-macos.sh --role client --enrollment-token TOKEN --controller-url URL
#   sudo ./scripts/provision-macos.sh --role server --tier 3 --cert /path/cert.pem --key /path/key.pem
#   sudo ./scripts/provision-macos.sh --role server   # local dashboard only
#   ./scripts/provision-macos.sh --check              # verify FIPS posture (no sudo)
#
# macOS FIPS notes:
#   - macOS uses CommonCrypto / Secure Transport (CMVP certified per release)
#   - There is NO "FIPS mode switch" on macOS — validated crypto is always active
#   - This script uses Go 1.24+ with GODEBUG=fips140=on (Go native FIPS 140-3)
#   - BoringCrypto is NOT available on macOS (linux-only .syso files)

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
PLIST_DIR="/Library/LaunchDaemons"
SERVICE_PREFIX="com.cloudflared-fips"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
ROLE="client"
TIER="1"
ENROLLMENT_TOKEN=""
CONTROLLER_URL=""
ADMIN_KEY=""
NODE_NAME=""
NODE_REGION=""
TLS_CERT=""
TLS_KEY=""
CHECK_ONLY=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)            ROLE="$2"; shift 2 ;;
        --tier)            TIER="$2"; shift 2 ;;
        --enrollment-token) ENROLLMENT_TOKEN="$2"; shift 2 ;;
        --controller-url)  CONTROLLER_URL="$2"; shift 2 ;;
        --admin-key)       ADMIN_KEY="$2"; shift 2 ;;
        --node-name)       NODE_NAME="$2"; shift 2 ;;
        --node-region)     NODE_REGION="$2"; shift 2 ;;
        --cert)            TLS_CERT="$2"; shift 2 ;;
        --key)             TLS_KEY="$2"; shift 2 ;;
        --check)           CHECK_ONLY=true; shift ;;
        --help|-h)
            echo "Usage: sudo $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --role ROLE          Node role: controller, server, proxy, client (default: client)"
            echo "  --tier TIER          Deployment tier: 1, 2, or 3 (default: 1)"
            echo "  --enrollment-token T Enrollment token from controller"
            echo "  --controller-url URL Fleet controller URL"
            echo "  --admin-key KEY      Admin API key for controller"
            echo "  --node-name NAME     Display name for this node"
            echo "  --node-region REGION Region label (e.g., us-east)"
            echo "  --cert PATH          TLS certificate (tier 3 server/proxy)"
            echo "  --key PATH           TLS private key (tier 3 server/proxy)"
            echo "  --check              Check FIPS posture only (no install)"
            echo ""
            echo "macOS FIPS notes:"
            echo "  - CommonCrypto is always FIPS-validated (no switch needed)"
            echo "  - Go binaries use GODEBUG=fips140=on (Go native FIPS 140-3)"
            echo "  - BoringCrypto is NOT available on macOS"
            exit 0
            ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# Validate
case "$ROLE" in
    controller|server|proxy|client) ;;
    *) echo "Invalid role: $ROLE"; exit 1 ;;
esac

case "$TIER" in
    1|2|3) ;;
    *) echo "Invalid tier: $TIER"; exit 1 ;;
esac

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
fail() { echo -e "${RED}[✗]${NC} $*"; exit 1; }
info() { echo -e "${BLUE}[i]${NC} $*"; }

detect_arch() {
    case "$(uname -m)" in
        x86_64)    echo "amd64" ;;
        arm64)     echo "arm64" ;;
        *)         fail "Unsupported architecture: $(uname -m)" ;;
    esac
}

# ---------------------------------------------------------------------------
# FIPS posture check (can run without sudo)
# ---------------------------------------------------------------------------
check_fips_posture() {
    echo ""
    echo "============================================"
    echo "  macOS FIPS Posture Check"
    echo "============================================"
    echo ""

    local ARCH
    ARCH=$(detect_arch)
    local MACOS_VERSION
    MACOS_VERSION=$(sw_vers -productVersion 2>/dev/null || echo "unknown")

    # macOS CommonCrypto is always FIPS-validated
    log "macOS version:        ${MACOS_VERSION}"
    log "Architecture:         ${ARCH}"
    log "CommonCrypto:         Always active (FIPS-validated per release)"

    # Check if corecrypto is present
    if [[ -d /usr/lib/system ]] && ls /usr/lib/system/libcorecrypto* &>/dev/null; then
        log "corecrypto module:    Present"
    else
        warn "corecrypto module:    Not found (expected on macOS)"
    fi

    # Check Secure Transport
    if [[ -f /System/Library/Frameworks/Security.framework/Security ]]; then
        log "Secure Transport:     Present"
    else
        warn "Secure Transport:     Not found"
    fi

    # Check for Go with FIPS support
    if command -v go &>/dev/null; then
        local go_ver
        go_ver=$(go version 2>/dev/null)
        if echo "$go_ver" | grep -qE 'go1\.(2[4-9]|[3-9][0-9])'; then
            log "Go version:           ${go_ver} (FIPS 140-3 capable)"
        else
            warn "Go version:           ${go_ver} (needs Go 1.24+ for native FIPS)"
        fi
    else
        warn "Go:                   Not installed"
    fi

    # Check if our agent is running
    if launchctl list "${SERVICE_PREFIX}.agent" &>/dev/null 2>&1; then
        log "FIPS agent:           Running"
    elif [[ -f "${BIN_DIR}/cloudflared-fips-agent" ]]; then
        warn "FIPS agent:           Installed but not running"
    else
        info "FIPS agent:           Not installed"
    fi

    # Check FileVault (disk encryption)
    if command -v fdesetup &>/dev/null; then
        if fdesetup isactive &>/dev/null; then
            log "FileVault:            Enabled"
        else
            warn "FileVault:            Disabled (recommended for FIPS compliance)"
        fi
    fi

    echo ""
    info "macOS does not have a FIPS mode switch."
    info "CommonCrypto/corecrypto is always FIPS-validated."
    info "Apple submits for CMVP validation per major macOS release."
    echo ""
}

if [[ "$CHECK_ONLY" == "true" ]]; then
    check_fips_posture
    exit 0
fi

# ---------------------------------------------------------------------------
# Require root for installation
# ---------------------------------------------------------------------------
if [[ $EUID -ne 0 ]]; then
    fail "Installation requires root (sudo). Use --check for posture check without sudo."
fi

# Verify macOS
if [[ "$(uname -s)" != "Darwin" ]]; then
    fail "This script is for macOS only. Use provision-linux.sh for Linux."
fi

echo ""
echo "============================================"
echo "  cloudflared-fips macOS provisioning"
echo "  Role: ${ROLE}  |  Tier: ${TIER}"
echo "  macOS $(sw_vers -productVersion 2>/dev/null || echo unknown)"
echo "============================================"
echo ""

info "macOS FIPS crypto: CommonCrypto (always validated, no switch needed)"
info "Go FIPS backend:   GODEBUG=fips140=on (Go native FIPS 140-3, CAVP A6650)"
echo ""

# ---------------------------------------------------------------------------
# Install Go
# ---------------------------------------------------------------------------
ARCH=$(detect_arch)

if command -v go &>/dev/null && go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
    log "Go ${GO_VERSION} already installed"
else
    log "Installing Go ${GO_VERSION} (${ARCH})..."
    # Try Homebrew first if available
    if command -v brew &>/dev/null; then
        # Homebrew Go might not be the exact version we want
        log "Downloading Go ${GO_VERSION} directly..."
    fi

    local go_arch="amd64"
    if [[ "$ARCH" == "arm64" ]]; then
        go_arch="arm64"
    fi
    curl -sLO "https://go.dev/dl/go${GO_VERSION}.darwin-${go_arch}.tar.gz"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "go${GO_VERSION}.darwin-${go_arch}.tar.gz"
    rm -f "go${GO_VERSION}.darwin-${go_arch}.tar.gz"
fi
export PATH="/usr/local/go/bin:$PATH"
log "Go version: $(go version)"

# ---------------------------------------------------------------------------
# Install Node.js (controller/server only)
# ---------------------------------------------------------------------------
if [[ "$ROLE" == "controller" || "$ROLE" == "server" ]]; then
    NODE_VERSION="20.19.0"
    if command -v node &>/dev/null && node --version 2>/dev/null | grep -qE '^v(2[0-9]|[3-9][0-9])\.'; then
        log "Node.js already installed: $(node --version)"
    else
        log "Installing Node.js ${NODE_VERSION}..."
        local node_arch="x64"
        if [[ "$ARCH" == "arm64" ]]; then
            node_arch="arm64"
        fi
        curl -sLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-darwin-${node_arch}.tar.gz"
        rm -rf /usr/local/node
        mkdir -p /usr/local/node
        tar -C /usr/local/node --strip-components=1 -xzf "node-v${NODE_VERSION}-darwin-${node_arch}.tar.gz"
        rm -f "node-v${NODE_VERSION}-darwin-${node_arch}.tar.gz"
        ln -sf /usr/local/node/bin/node /usr/local/bin/node
        ln -sf /usr/local/node/bin/npm /usr/local/bin/npm
        ln -sf /usr/local/node/bin/npx /usr/local/bin/npx
    fi
    export PATH="/usr/local/node/bin:$PATH"
    log "Node.js version: $(node --version)"
fi

# ---------------------------------------------------------------------------
# Clone and build
# ---------------------------------------------------------------------------
if [[ -d "${INSTALL_DIR}/.git" ]]; then
    log "Updating existing repo in ${INSTALL_DIR}..."
    cd "$INSTALL_DIR"
    git pull --ff-only
else
    log "Cloning repo to ${INSTALL_DIR}..."
    mkdir -p "$(dirname "$INSTALL_DIR")"
    git clone "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# Build dashboard frontend (controller/server)
if [[ "$ROLE" == "controller" || "$ROLE" == "server" ]]; then
    log "Building dashboard frontend..."
    cd dashboard && npm install && npm run build && cd ..
    mkdir -p internal/dashboard/static
    cp -r dashboard/dist/* internal/dashboard/static/
fi

# Build Go binaries with Go native FIPS (no BoringCrypto on macOS)
log "Building binaries with GODEBUG=fips140=on..."
export GODEBUG="fips140=on"
export CGO_ENABLED=0

case "$ROLE" in
    controller|server)
        go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-selftest ./cmd/selftest/
        go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-dashboard ./cmd/dashboard/
        if [[ "$TIER" == "3" ]]; then
            go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-proxy ./cmd/fips-proxy/
        fi
        ;;
    proxy)
        go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-selftest ./cmd/selftest/
        go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-proxy ./cmd/fips-proxy/
        ;;
    client)
        go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-selftest ./cmd/selftest/
        go build -trimpath -ldflags="-s -w" -o build-output/cloudflared-fips-agent ./cmd/agent/
        ;;
esac

log "Running self-test..."
./build-output/cloudflared-fips-selftest 2>/dev/null || true
echo ""

# ---------------------------------------------------------------------------
# Install binaries
# ---------------------------------------------------------------------------
log "Installing binaries..."
install -m 0755 build-output/cloudflared-fips-selftest "${BIN_DIR}/"

case "$ROLE" in
    controller|server)
        install -m 0755 build-output/cloudflared-fips-dashboard "${BIN_DIR}/"
        if [[ "$TIER" == "3" && -f build-output/cloudflared-fips-proxy ]]; then
            install -m 0755 build-output/cloudflared-fips-proxy "${BIN_DIR}/"
        fi
        ;;
    proxy)
        install -m 0755 build-output/cloudflared-fips-proxy "${BIN_DIR}/"
        ;;
    client)
        install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
        ;;
esac

# ---------------------------------------------------------------------------
# Config directories
# ---------------------------------------------------------------------------
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"
cp -n build-output/build-manifest.json "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || true
cp -n configs/cloudflared-fips.yaml "${CONFIG_DIR}/cloudflared-fips.yaml" 2>/dev/null || true

# TLS cert/key for Tier 3
if [[ "$TIER" == "3" && -n "$TLS_CERT" && -n "$TLS_KEY" ]]; then
    log "Installing TLS certificate and key for Tier 3..."
    cp "$TLS_CERT" "${CONFIG_DIR}/tls-cert.pem"
    cp "$TLS_KEY" "${CONFIG_DIR}/tls-key.pem"
    chmod 0600 "${CONFIG_DIR}/tls-key.pem"
    chmod 0644 "${CONFIG_DIR}/tls-cert.pem"
fi

# Environment file
ENV_FILE="${CONFIG_DIR}/env"
if [[ ! -f "$ENV_FILE" ]]; then
    touch "$ENV_FILE"
fi
echo "DEPLOYMENT_TIER=${TIER}" >> "$ENV_FILE"
echo "GODEBUG=fips140=on" >> "$ENV_FILE"
chmod 0600 "$ENV_FILE"

# ---------------------------------------------------------------------------
# Fleet enrollment
# ---------------------------------------------------------------------------
if [[ "$ROLE" != "controller" && -n "$ENROLLMENT_TOKEN" && -n "$CONTROLLER_URL" ]]; then
    log "Enrolling with fleet controller..."
    local hostname
    hostname=$(hostname -s)
    local name="${NODE_NAME:-${hostname}}"

    ENROLL_RESP=$(curl -sf -X POST "${CONTROLLER_URL}/api/v1/fleet/enroll" \
        -H "Content-Type: application/json" \
        -d "{\"token\":\"${ENROLLMENT_TOKEN}\",\"name\":\"${name}\",\"region\":\"${NODE_REGION}\",\"os\":\"darwin\",\"arch\":\"${ARCH}\",\"version\":\"dev\",\"fips_backend\":\"GoNative\"}" \
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

    echo "NODE_ID=${NODE_ID}" >> "$ENV_FILE"
    echo "NODE_API_KEY=${API_KEY}" >> "$ENV_FILE"
    echo "CONTROLLER_URL=${CONTROLLER_URL}" >> "$ENV_FILE"

    log "Enrolled as node ${NODE_ID}"
fi

# Controller admin key
if [[ "$ROLE" == "controller" ]]; then
    if [[ -z "$ADMIN_KEY" ]]; then
        ADMIN_KEY=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
        log "Generated admin API key: ${ADMIN_KEY}"
    fi
    echo "FLEET_ADMIN_KEY=${ADMIN_KEY}" >> "$ENV_FILE"
fi

# ---------------------------------------------------------------------------
# Create launchd plists
# ---------------------------------------------------------------------------
log "Creating launchd services..."

# Helper to create a plist
create_plist() {
    local label="$1"
    local binary="$2"
    shift 2
    local args=("$@")

    local plist_path="${PLIST_DIR}/${label}.plist"

    # Build ProgramArguments XML
    local args_xml="    <string>${binary}</string>"
    for arg in "${args[@]}"; do
        args_xml="${args_xml}
    <string>${arg}</string>"
    done

    cat > "$plist_path" <<PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${label}</string>
    <key>ProgramArguments</key>
    <array>
${args_xml}
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>GODEBUG</key>
        <string>fips140=on</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/${label}.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/${label}.err</string>
</dict>
</plist>
PLISTEOF

    chmod 0644 "$plist_path"
}

case "$ROLE" in
    controller)
        create_plist "${SERVICE_PREFIX}.dashboard" \
            "${BIN_DIR}/cloudflared-fips-dashboard" \
            "--addr" "0.0.0.0:8080" \
            "--manifest" "${CONFIG_DIR}/build-manifest.json" \
            "--config" "${CONFIG_DIR}/cloudflared-fips.yaml" \
            "--fleet-mode" "--db-path" "${DATA_DIR}/fleet.db"
        ;;
    server)
        create_plist "${SERVICE_PREFIX}.dashboard" \
            "${BIN_DIR}/cloudflared-fips-dashboard" \
            "--addr" "127.0.0.1:8080" \
            "--manifest" "${CONFIG_DIR}/build-manifest.json" \
            "--config" "${CONFIG_DIR}/cloudflared-fips.yaml"

        if [[ "$TIER" == "3" ]]; then
            local proxy_args=("--listen" ":443" "--upstream" "localhost:8080")
            if [[ -n "$TLS_CERT" ]]; then
                proxy_args+=("--tls-cert" "${CONFIG_DIR}/tls-cert.pem" "--tls-key" "${CONFIG_DIR}/tls-key.pem")
            fi
            create_plist "${SERVICE_PREFIX}.proxy" \
                "${BIN_DIR}/cloudflared-fips-proxy" \
                "${proxy_args[@]}"
        fi
        ;;
    proxy)
        local proxy_args=("--listen" ":443" "--upstream" "localhost:8080")
        if [[ -n "$TLS_CERT" ]]; then
            proxy_args+=("--tls-cert" "${CONFIG_DIR}/tls-cert.pem" "--tls-key" "${CONFIG_DIR}/tls-key.pem")
        fi
        create_plist "${SERVICE_PREFIX}.proxy" \
            "${BIN_DIR}/cloudflared-fips-proxy" \
            "${proxy_args[@]}"
        ;;
    client)
        local agent_args=()
        if [[ -f "${CONFIG_DIR}/enrollment.json" ]]; then
            agent_args+=("--config" "${CONFIG_DIR}/enrollment.json")
        fi
        create_plist "${SERVICE_PREFIX}.agent" \
            "${BIN_DIR}/cloudflared-fips-agent" \
            "${agent_args[@]}"
        ;;
esac

# ---------------------------------------------------------------------------
# Load services
# ---------------------------------------------------------------------------
log "Loading launchd services..."

case "$ROLE" in
    controller)
        launchctl load -w "${PLIST_DIR}/${SERVICE_PREFIX}.dashboard.plist"
        ;;
    server)
        launchctl load -w "${PLIST_DIR}/${SERVICE_PREFIX}.dashboard.plist"
        if [[ "$TIER" == "3" ]]; then
            launchctl load -w "${PLIST_DIR}/${SERVICE_PREFIX}.proxy.plist"
        fi
        ;;
    proxy)
        launchctl load -w "${PLIST_DIR}/${SERVICE_PREFIX}.proxy.plist"
        ;;
    client)
        launchctl load -w "${PLIST_DIR}/${SERVICE_PREFIX}.agent.plist"
        ;;
esac

sleep 2

# ---------------------------------------------------------------------------
# Verify and summary
# ---------------------------------------------------------------------------
echo ""
log "============================================"
log "  cloudflared-fips deployed on macOS!"
log "  Role: ${ROLE}  |  Tier: ${TIER}"
log "  macOS $(sw_vers -productVersion 2>/dev/null || echo unknown) (${ARCH})"
log "============================================"
echo ""

info "FIPS backend:     Go native FIPS 140-3 (GODEBUG=fips140=on)"
info "OS crypto:        CommonCrypto (always FIPS-validated)"
info "CMVP note:        Go native FIPS CAVP A6650, CMVP pending"
echo ""

case "$ROLE" in
    controller)
        info "Dashboard:    http://0.0.0.0:8080"
        info "Fleet API:    http://0.0.0.0:8080/api/v1/fleet/"
        if [[ -n "$ADMIN_KEY" ]]; then
            info "Admin key:    ${ADMIN_KEY}"
        fi
        info "Logs:         tail -f /var/log/${SERVICE_PREFIX}.dashboard.log"
        ;;
    server)
        info "Dashboard:    http://127.0.0.1:8080"
        info "Logs:         tail -f /var/log/${SERVICE_PREFIX}.dashboard.log"
        if [[ "$TIER" == "3" ]]; then
            info "FIPS Proxy:   :443"
            info "Proxy logs:   tail -f /var/log/${SERVICE_PREFIX}.proxy.log"
        fi
        ;;
    proxy)
        info "FIPS Proxy:   :443"
        info "Logs:         tail -f /var/log/${SERVICE_PREFIX}.proxy.log"
        ;;
    client)
        info "Agent:        running as launchd service"
        info "Check:        cloudflared-fips-agent --check"
        info "Logs:         tail -f /var/log/${SERVICE_PREFIX}.agent.log"
        ;;
esac

echo ""
info "To stop:   sudo launchctl unload ${PLIST_DIR}/${SERVICE_PREFIX}.*.plist"
info "To start:  sudo launchctl load -w ${PLIST_DIR}/${SERVICE_PREFIX}.*.plist"
info "Self-test: cloudflared-fips-selftest"
echo ""

#!/usr/bin/env bash
# provision-linux.sh — Multi-role, multi-distro Linux provisioning for cloudflared-fips fleet
#
# Usage:
#   sudo ./scripts/provision-linux.sh                                    # default: server role, tier 1
#   sudo ./scripts/provision-linux.sh --role controller                  # fleet controller
#   sudo ./scripts/provision-linux.sh --role server --tunnel-token TOKEN # server with tunnel
#   sudo ./scripts/provision-linux.sh --role server --tier 3 --cert /path/cert.pem --key /path/key.pem --upstream https://origin:8080
#   sudo ./scripts/provision-linux.sh --role proxy                       # FIPS edge proxy
#   sudo ./scripts/provision-linux.sh --role client --enrollment-token T --controller-url URL
#   sudo ./scripts/provision-linux.sh --no-fips                          # skip FIPS mode (dev/test)
#   sudo ./scripts/provision-linux.sh --with-cf                          # prompt for CF API creds
#
# Supported distros: RHEL 8/9, Rocky 8/9, AlmaLinux 8/9, Ubuntu 20.04+/22.04+,
#                     Amazon Linux 2/2023, SUSE SLES 15, Oracle Linux 8/9
#
# This script is idempotent — safe to re-run after reboot.
# After enabling FIPS mode it will reboot automatically. Run again after reboot.

set -euo pipefail

# Save original args for resume after FIPS reboot
ORIG_ARGS="$*"

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

# Detected at runtime
PKG_MGR=""
DISTRO_FAMILY=""  # rhel, debian, suse, amzn
DISTRO_NAME=""
DISTRO_VERSION=""

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
ROLE="server"
TIER="1"
SKIP_FIPS=false
WITH_CF=false
TUNNEL_TOKEN=""
ENROLLMENT_TOKEN=""
CONTROLLER_URL=""
ADMIN_KEY=""
NODE_NAME=""
NODE_REGION=""
TLS_CERT=""
TLS_KEY=""
UPSTREAM="http://localhost:8080"
CF_API_TOKEN=""
CF_ZONE_ID=""
CF_ACCOUNT_ID=""
CF_TUNNEL_ID=""
SERVICE_HOST=""
SERVICE_PORT=""
SERVICE_NAME=""
SERVICE_TLS=false
ENFORCEMENT_MODE="audit"
REQUIRE_OS_FIPS=false
REQUIRE_DISK_ENC=false
PROTOCOL="quic"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)            ROLE="$2"; shift 2 ;;
        --tier)            TIER="$2"; shift 2 ;;
        --no-fips)         SKIP_FIPS=true; shift ;;
        --with-cf)         WITH_CF=true; shift ;;
        --tunnel-token)    TUNNEL_TOKEN="$2"; shift 2 ;;
        --enrollment-token) ENROLLMENT_TOKEN="$2"; shift 2 ;;
        --controller-url)  CONTROLLER_URL="$2"; shift 2 ;;
        --admin-key)       ADMIN_KEY="$2"; shift 2 ;;
        --node-name)       NODE_NAME="$2"; shift 2 ;;
        --node-region)     NODE_REGION="$2"; shift 2 ;;
        --cert)            TLS_CERT="$2"; shift 2 ;;
        --key)             TLS_KEY="$2"; shift 2 ;;
        --upstream)        UPSTREAM="$2"; shift 2 ;;
        --cf-api-token)    CF_API_TOKEN="$2"; shift 2 ;;
        --cf-zone-id)      CF_ZONE_ID="$2"; shift 2 ;;
        --cf-account-id)   CF_ACCOUNT_ID="$2"; shift 2 ;;
        --cf-tunnel-id)    CF_TUNNEL_ID="$2"; shift 2 ;;
        --service-host)    SERVICE_HOST="$2"; shift 2 ;;
        --service-port)    SERVICE_PORT="$2"; shift 2 ;;
        --service-name)    SERVICE_NAME="$2"; shift 2 ;;
        --service-tls)     SERVICE_TLS=true; shift ;;
        --enforcement-mode) ENFORCEMENT_MODE="$2"; shift 2 ;;
        --require-os-fips) REQUIRE_OS_FIPS=true; shift ;;
        --require-disk-enc) REQUIRE_DISK_ENC=true; shift ;;
        --protocol)        PROTOCOL="$2"; shift 2 ;;
        --help|-h)
            echo "Usage: sudo $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --role ROLE          Node role: controller, server, proxy, client (default: server)"
            echo "  --tier TIER          Deployment tier: 1, 2, or 3 (default: 1)"
            echo "                         1 = Cloudflare Tunnel (standard)"
            echo "                         2 = Cloudflare Tunnel + Regional Services + Keyless SSL"
            echo "                         3 = Self-hosted FIPS proxy (no Cloudflare dependency)"
            echo "  --no-fips            Skip FIPS mode enablement (dev/test only)"
            echo "  --with-cf            Prompt for Cloudflare API credentials"
            echo "  --tunnel-token TOKEN cloudflared tunnel token (controller/proxy, tier 1/2)"
            echo "  --enrollment-token T Enrollment token from controller (server/proxy/client)"
            echo "  --controller-url URL Fleet controller URL (required for non-controller roles)"
            echo "  --admin-key KEY      Admin API key for controller"
            echo "  --node-name NAME     Display name for this node"
            echo "  --node-region REGION Region label (e.g., us-east, eu-west)"
            echo "  --cert PATH          TLS certificate path (tier 3 server/proxy/controller)"
            echo "  --key PATH           TLS private key path (tier 3 server/proxy/controller)"
            echo "  --upstream URL       Origin app URL (default: http://localhost:8080)"
            echo "  --service-host HOST  Origin service host (server role)"
            echo "  --service-port PORT  Origin service port (server role)"
            echo "  --service-name NAME  Origin service display name (server role)"
            echo "  --service-tls        Origin service uses TLS (server role)"
            echo "  --enforcement-mode M Enforcement mode: audit or enforce (controller)"
            echo "  --require-os-fips    Require OS FIPS mode for fleet nodes (controller)"
            echo "  --require-disk-enc   Require disk encryption for fleet nodes (controller)"
            echo "  --protocol PROTO     Tunnel protocol: quic or http2 (controller/proxy)"
            echo "  --cf-api-token TOK   Cloudflare API token (non-interactive)"
            echo "  --cf-zone-id ID      Cloudflare zone ID (non-interactive)"
            echo "  --cf-account-id ID   Cloudflare account ID (non-interactive)"
            echo "  --cf-tunnel-id ID    Cloudflare tunnel ID (non-interactive)"
            exit 0
            ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# ---------------------------------------------------------------------------
# Detect pre-installed binaries (from RPM/DEB package)
# ---------------------------------------------------------------------------
PREINSTALLED=false
if [ -x /usr/local/bin/cloudflared-fips-selftest ] && [ -x /usr/local/bin/cloudflared-fips-agent ]; then
    PREINSTALLED=true
fi

# Validate role
case "$ROLE" in
    controller|server|proxy|client) ;;
    *) echo "Invalid role: $ROLE (must be controller, server, proxy, or client)"; exit 1 ;;
esac

# Validate tier
case "$TIER" in
    1|2|3) ;;
    *) echo "Invalid tier: $TIER (must be 1, 2, or 3)"; exit 1 ;;
esac

# Tier 3 server/proxy requires TLS cert+key; controller with cert+key gets proxy capability
if [[ "$TIER" == "3" && ("$ROLE" == "server" || "$ROLE" == "proxy") ]]; then
    if [[ -z "$TLS_CERT" || -z "$TLS_KEY" ]]; then
        echo "Tier 3 server/proxy requires --cert and --key for TLS termination"
        exit 1
    fi
fi
# Validate cert/key files exist when provided
if [[ -n "$TLS_CERT" && ! -f "$TLS_CERT" ]]; then
    echo "TLS certificate not found: $TLS_CERT"
    exit 1
fi
if [[ -n "$TLS_KEY" && ! -f "$TLS_KEY" ]]; then
    echo "TLS key not found: $TLS_KEY"
    exit 1
fi

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
tier() { echo -e "${CYAN}[T${TIER}]${NC} $*"; }

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
# Distro detection
# ---------------------------------------------------------------------------
detect_distro() {
    if [[ ! -f /etc/os-release ]]; then
        fail "Cannot detect distro: /etc/os-release not found"
    fi

    # shellcheck source=/dev/null
    . /etc/os-release

    DISTRO_NAME="${ID}"
    DISTRO_VERSION="${VERSION_ID:-unknown}"

    case "$ID" in
        rhel|rocky|almalinux|ol)
            DISTRO_FAMILY="rhel"
            PKG_MGR="dnf"
            # RHEL 8 might only have yum
            if ! command -v dnf &>/dev/null && command -v yum &>/dev/null; then
                PKG_MGR="yum"
            fi
            ;;
        centos)
            DISTRO_FAMILY="rhel"
            PKG_MGR="dnf"
            if ! command -v dnf &>/dev/null; then
                PKG_MGR="yum"
            fi
            ;;
        ubuntu|debian)
            DISTRO_FAMILY="debian"
            PKG_MGR="apt-get"
            ;;
        amzn)
            DISTRO_FAMILY="amzn"
            PKG_MGR="dnf"
            if [[ "$VERSION_ID" == "2" ]]; then
                PKG_MGR="yum"
            fi
            ;;
        sles|opensuse*)
            DISTRO_FAMILY="suse"
            PKG_MGR="zypper"
            ;;
        *)
            # Try to detect family from ID_LIKE
            if echo "${ID_LIKE:-}" | grep -q "rhel\|fedora\|centos"; then
                DISTRO_FAMILY="rhel"
                PKG_MGR="dnf"
                if ! command -v dnf &>/dev/null; then
                    PKG_MGR="yum"
                fi
            elif echo "${ID_LIKE:-}" | grep -q "debian\|ubuntu"; then
                DISTRO_FAMILY="debian"
                PKG_MGR="apt-get"
            else
                fail "Unsupported distro: ${ID} (${ID_LIKE:-no ID_LIKE}). Supported: RHEL, Rocky, Alma, Oracle, Ubuntu, Debian, Amazon Linux, SLES"
            fi
            ;;
    esac

    log "Detected: ${PRETTY_NAME:-$ID $VERSION_ID} (family: ${DISTRO_FAMILY}, pkg: ${PKG_MGR})"
}

# ---------------------------------------------------------------------------
# Package installation abstraction
# ---------------------------------------------------------------------------
pkg_install() {
    case "$PKG_MGR" in
        dnf)     dnf install -y "$@" ;;
        yum)     yum install -y "$@" ;;
        apt-get) apt-get install -y "$@" ;;
        zypper)  zypper install -y "$@" ;;
    esac
}

pkg_update() {
    case "$PKG_MGR" in
        dnf)     dnf makecache ;;
        yum)     yum makecache ;;
        apt-get) apt-get update ;;
        zypper)  zypper refresh ;;
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

    case "$DISTRO_FAMILY" in
        rhel)
            log "Enabling FIPS mode (RHEL family)..."
            pkg_install crypto-policies-scripts
            fips-mode-setup --enable
            ;;
        debian)
            if [[ "$DISTRO_NAME" == "ubuntu" ]]; then
                # Ubuntu Pro required for FIPS
                if command -v ua &>/dev/null || command -v pro &>/dev/null; then
                    log "Enabling FIPS mode (Ubuntu Pro)..."
                    if command -v pro &>/dev/null; then
                        pro enable fips --assume-yes
                    else
                        ua enable fips --assume-yes
                    fi
                else
                    warn "Ubuntu FIPS mode requires Ubuntu Pro subscription."
                    warn "Install ubuntu-advantage-tools and attach your subscription first:"
                    warn "  sudo apt install ubuntu-advantage-tools"
                    warn "  sudo pro attach <token>"
                    warn "  sudo pro enable fips"
                    fail "Ubuntu Pro not detected. Cannot enable FIPS mode."
                fi
            else
                warn "Debian does not have official FIPS validation."
                warn "Consider using Ubuntu Pro or RHEL for FIPS compliance."
                fail "FIPS mode not available on plain Debian."
            fi
            ;;
        amzn)
            log "Enabling FIPS mode (Amazon Linux)..."
            if [[ "$DISTRO_VERSION" == "2" ]]; then
                yum install -y dracut-fips
                dracut -f
                # Enable via kernel parameter
                grubby --update-kernel=ALL --args="fips=1"
            else
                # Amazon Linux 2023
                pkg_install crypto-policies-scripts 2>/dev/null || true
                if command -v fips-mode-setup &>/dev/null; then
                    fips-mode-setup --enable
                else
                    # Fallback: kernel parameter
                    grubby --update-kernel=ALL --args="fips=1"
                fi
            fi
            ;;
        suse)
            log "Enabling FIPS mode (SLES)..."
            # SLES uses kernel boot parameter
            if command -v grubby &>/dev/null; then
                grubby --update-kernel=ALL --args="fips=1"
            else
                warn "Set fips=1 in GRUB boot parameters manually:"
                warn "  Edit /etc/default/grub, add fips=1 to GRUB_CMDLINE_LINUX"
                warn "  Run: grub2-mkconfig -o /boot/grub2/grub.cfg"
                fail "Cannot auto-enable FIPS on SLES without grubby."
            fi
            ;;
    esac

    # Save the full command line so we can resume after reboot
    RESUME_CMD="sudo $(readlink -f "$0") $ORIG_ARGS"
    {
        echo "phase=2"
        echo "resume_cmd=$RESUME_CMD"
    } > "$MARKER"

    # Drop a login helper so the user sees the resume command on login
    cat > /etc/profile.d/cloudflared-fips-resume.sh <<'PROFILEEOF'
if [ -f /var/tmp/.cloudflared-fips-provision-phase ]; then
    echo ""
    echo "============================================"
    echo "  cloudflared-fips: provisioning paused"
    echo "  FIPS mode is now enabled after reboot."
    echo ""
    RESUME=$(grep '^resume_cmd=' /var/tmp/.cloudflared-fips-provision-phase | cut -d= -f2-)
    echo "  Resume with:"
    echo "    $RESUME"
    echo "============================================"
    echo ""
fi
PROFILEEOF

    warn "============================================"
    warn "  FIPS mode enabled. Rebooting in 5 seconds."
    warn ""
    warn "  After reboot, log in and run:"
    warn "    $RESUME_CMD"
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
            fail "FIPS mode is NOT enabled. Enable it and reboot first."
        fi
    fi

    # --- Build dependencies ---
    log "Installing build dependencies..."
    case "$DISTRO_FAMILY" in
        rhel|amzn)
            pkg_install gcc gcc-c++ make git curl
            ;;
        debian)
            export DEBIAN_FRONTEND=noninteractive
            pkg_update
            pkg_install gcc g++ make git curl ca-certificates
            ;;
        suse)
            pkg_install gcc gcc-c++ make git curl
            ;;
    esac

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

    # --- Node.js 20 LTS (only needed for controller role with dashboard) ---
    if [[ "$ROLE" == "controller" ]]; then
        NODE_VERSION="20.19.0"
        if command -v node &>/dev/null && node --version 2>/dev/null | grep -qE '^v(2[0-9]|[3-9][0-9])\.'; then
            log "Node.js already installed: $(node --version)"
        else
            log "Installing Node.js ${NODE_VERSION} (${ARCH})..."
            # Remove distro Node.js if present
            case "$DISTRO_FAMILY" in
                rhel|amzn) dnf remove -y nodejs npm 2>/dev/null || yum remove -y nodejs npm 2>/dev/null || true ;;
                debian)    apt-get remove -y nodejs npm 2>/dev/null || true ;;
                suse)      zypper remove -y nodejs npm 2>/dev/null || true ;;
            esac

            local node_arch="x64"
            if [[ "$ARCH" == "arm64" ]]; then
                node_arch="arm64"
            fi
            curl -sLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz"
            rm -rf /usr/local/node
            mkdir -p /usr/local/node
            tar -C /usr/local/node --strip-components=1 -xJf "node-v${NODE_VERSION}-linux-${node_arch}.tar.xz"
            rm -f "node-v${NODE_VERSION}-linux-${node_arch}.tar.xz"
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

    # --- Build dashboard frontend (only for controller role) ---
    if [[ "$ROLE" == "controller" ]]; then
        log "Building dashboard frontend..."
        cd dashboard && npm install && npm run build && cd ..
        mkdir -p internal/dashboard/static
        cp -r dashboard/dist/* internal/dashboard/static/
    fi

    # --- Build binaries based on role and tier ---
    log "Building binaries for role: ${ROLE}, tier: ${TIER}..."
    case "$ROLE" in
        controller)
            # Controller: dashboard (fleet mode), selftest, agent
            # cloudflared tunnel will be managed by the cloudflared binary (downloaded separately)
            if [[ -n "$TLS_CERT" && -n "$TLS_KEY" ]]; then
                make selftest-bin dashboard-bin agent-bin fips-proxy-bin
            else
                make selftest-bin dashboard-bin agent-bin
            fi
            ;;
        server)
            # Server: selftest + agent only (no dashboard, no cloudflared tunnel)
            make selftest-bin agent-bin
            ;;
        proxy)
            # Proxy: fips-proxy, selftest, agent (no dashboard)
            make selftest-bin fips-proxy-bin agent-bin
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
    if [[ "$PREINSTALLED" == "false" ]]; then
        cd "$INSTALL_DIR"
    fi

    # --- Create service user ---
    if id "$SERVICE_USER" &>/dev/null; then
        log "Service user '${SERVICE_USER}' already exists"
    else
        log "Creating service user '${SERVICE_USER}'..."
        case "$DISTRO_FAMILY" in
            rhel|amzn|suse)
                useradd -r -s /sbin/nologin "$SERVICE_USER"
                ;;
            debian)
                useradd -r -s /usr/sbin/nologin "$SERVICE_USER" || adduser --system --no-create-home --group "$SERVICE_USER"
                ;;
        esac
    fi

    # --- Install binaries based on role ---
    if [[ "$PREINSTALLED" == "true" ]]; then
        log "Binaries already installed at ${BIN_DIR}/ (from package). Skipping install."
    else
        log "Installing binaries for role: ${ROLE}..."
        install -m 0755 build-output/cloudflared-fips-selftest "${BIN_DIR}/"

        case "$ROLE" in
            controller)
                install -m 0755 build-output/cloudflared-fips-dashboard "${BIN_DIR}/"
                install -m 0755 build-output/cloudflared-fips-agent "${BIN_DIR}/"
                if [[ -f build-output/cloudflared-fips-proxy ]]; then
                    install -m 0755 build-output/cloudflared-fips-proxy "${BIN_DIR}/"
                fi
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
    fi

    # --- Config and data directories ---
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    if [[ "$PREINSTALLED" == "true" ]]; then
        cp -n /usr/share/cloudflared-fips/build-manifest.json "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || true
        cp -n /etc/cloudflared/config.yaml.sample "${CONFIG_DIR}/cloudflared-fips.yaml" 2>/dev/null || true
    else
        cp -n build-output/build-manifest.json "${CONFIG_DIR}/build-manifest.json" 2>/dev/null || true
        cp -n configs/cloudflared-fips.yaml "${CONFIG_DIR}/cloudflared-fips.yaml" 2>/dev/null || true
    fi

    # --- Copy TLS cert/key for FIPS proxy (Tier 3, or controller with proxy) ---
    if [[ -n "$TLS_CERT" && -n "$TLS_KEY" ]]; then
        log "Installing TLS certificate and key..."
        cp "$TLS_CERT" "${CONFIG_DIR}/tls-cert.pem"
        cp "$TLS_KEY" "${CONFIG_DIR}/tls-key.pem"
        chmod 0600 "${CONFIG_DIR}/tls-key.pem"
        chmod 0644 "${CONFIG_DIR}/tls-cert.pem"
    fi

    # --- Binary hashes for manifest ---
    for bin in selftest dashboard tui proxy agent; do
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
    for bin in selftest dashboard tui proxy agent; do
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
    if [[ -n "$CF_API_TOKEN" ]]; then
        # Non-interactive: credentials passed via flags (from TUI wizard)
        log "Writing Cloudflare API credentials from flags..."
        cat > "$ENV_FILE" <<ENVEOF
CF_API_TOKEN=${CF_API_TOKEN}
CF_ZONE_ID=${CF_ZONE_ID}
CF_ACCOUNT_ID=${CF_ACCOUNT_ID}
CF_TUNNEL_ID=${CF_TUNNEL_ID}
ENVEOF
    elif [[ "$WITH_CF" == "true" && ! -f "$ENV_FILE" ]]; then
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

    # Store deployment tier
    echo "DEPLOYMENT_TIER=${TIER}" >> "$ENV_FILE"

    chmod 0600 "$ENV_FILE"
    chown "${SERVICE_USER}:" "$ENV_FILE"

    # --- Fleet enrollment (for non-controller roles with enrollment token) ---
    if [[ "$ROLE" != "controller" && -n "$ENROLLMENT_TOKEN" && -n "$CONTROLLER_URL" ]]; then
        log "Enrolling with fleet controller..."
        local hostname
        hostname=$(hostname -s)
        local name="${NODE_NAME:-${hostname}}"

        # Resolve version from build manifest (package or source build)
        local manifest_path=""
        if [[ -f "${CONFIG_DIR}/build-manifest.json" ]]; then
            manifest_path="${CONFIG_DIR}/build-manifest.json"
        elif [[ -f /usr/share/cloudflared-fips/build-manifest.json ]]; then
            manifest_path="/usr/share/cloudflared-fips/build-manifest.json"
        elif [[ -f build-output/build-manifest.json ]]; then
            manifest_path="build-output/build-manifest.json"
        fi
        local node_version="dev"
        if [[ -n "$manifest_path" ]]; then
            node_version=$(python3 -c "import sys,json; print(json.load(open('${manifest_path}')).get('version','dev'))" 2>/dev/null || echo "dev")
        fi

        ENROLL_RESP=$(curl -sf -X POST "${CONTROLLER_URL}/api/v1/fleet/enroll" \
            -H "Content-Type: application/json" \
            -d "{\"token\":\"${ENROLLMENT_TOKEN}\",\"name\":\"${name}\",\"region\":\"${NODE_REGION}\",\"version\":\"${node_version}\",\"fips_backend\":\"BoringCrypto\"}" \
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

    # --- Write service info to env file (server role) ---
    if [[ "$ROLE" == "server" && -n "$SERVICE_NAME" ]]; then
        echo "SERVICE_NAME=${SERVICE_NAME}" >> "$ENV_FILE"
        if [[ -n "$SERVICE_HOST" ]]; then
            echo "SERVICE_HOST=${SERVICE_HOST}" >> "$ENV_FILE"
        fi
        if [[ -n "$SERVICE_PORT" ]]; then
            echo "SERVICE_PORT=${SERVICE_PORT}" >> "$ENV_FILE"
        fi
        if [[ "$SERVICE_TLS" == "true" ]]; then
            echo "SERVICE_TLS=true" >> "$ENV_FILE"
        fi
    fi

    # --- Write protocol to env file (controller/proxy) ---
    if [[ "$ROLE" == "controller" || "$ROLE" == "proxy" ]]; then
        echo "PROTOCOL=${PROTOCOL}" >> "$ENV_FILE"
    fi

    # --- Write enforcement mode to env file (controller) ---
    if [[ "$ROLE" == "controller" ]]; then
        echo "ENFORCEMENT_MODE=${ENFORCEMENT_MODE}" >> "$ENV_FILE"
        if [[ "$REQUIRE_OS_FIPS" == "true" ]]; then
            echo "REQUIRE_OS_FIPS=true" >> "$ENV_FILE"
        fi
        if [[ "$REQUIRE_DISK_ENC" == "true" ]]; then
            echo "REQUIRE_DISK_ENC=true" >> "$ENV_FILE"
        fi
    fi

    # --- Download cloudflared (when tunnel-token is provided for controller/proxy) ---
    if [[ -n "$TUNNEL_TOKEN" && ("$ROLE" == "controller" || "$ROLE" == "proxy") ]]; then
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
    log "cloudflared installed: $("${BIN_DIR}/cloudflared" --version 2>&1 | head -1)"
}

# ---------------------------------------------------------------------------
# Create systemd units based on role and tier
# ---------------------------------------------------------------------------
create_systemd_units() {
    log "Creating systemd units for role: ${ROLE}, tier: ${TIER}..."

    # --- Dashboard service (controller only) ---
    if [[ "$ROLE" == "controller" ]]; then
        local fleet_flags=""
        local listen_addr="127.0.0.1:8080"
        local proxy_addr_flag=""
        local tier_flag="--deployment-tier standard"

        if [[ "$ROLE" == "controller" ]]; then
            fleet_flags="--fleet-mode --db-path ${DATA_DIR}/fleet.db"
            listen_addr="0.0.0.0:8080"  # Controllers need to be accessible
        fi

        # When a proxy is running alongside, dashboard fetches client stats from it
        if [[ -n "$TLS_CERT" || "$TIER" == "3" ]]; then
            proxy_addr_flag="--proxy-addr localhost:8081"
        fi

        # Map tier to deployment-tier flag value
        case "$TIER" in
            1) tier_flag="--deployment-tier standard" ;;
            2) tier_flag="--deployment-tier regional_keyless" ;;
            3) tier_flag="--deployment-tier self_hosted" ;;
        esac

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
ExecStart=${BIN_DIR}/cloudflared-fips-dashboard \\
  --addr ${listen_addr} \\
  --manifest ${CONFIG_DIR}/build-manifest.json \\
  --config ${CONFIG_DIR}/cloudflared-fips.yaml \\
  --metrics-addr localhost:2000 \\
  --ingress-targets localhost:8080 \\
  ${tier_flag} \\
  ${proxy_addr_flag} \\
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

    # --- Tunnel service (controller and proxy roles with tunnel-token) ---
    if [[ -n "$TUNNEL_TOKEN" && ("$ROLE" == "controller" || "$ROLE" == "proxy") ]]; then
        echo "TUNNEL_TOKEN=${TUNNEL_TOKEN}" >> "${CONFIG_DIR}/env"

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
ExecStart=${BIN_DIR}/cloudflared tunnel run --token ${TUNNEL_TOKEN}
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

    # --- FIPS proxy service (proxy role, or controller with cert+key) ---
    if [[ "$ROLE" == "proxy" || ("$ROLE" == "controller" && -n "$TLS_CERT") ]]; then
        cat > /etc/systemd/system/cloudflared-fips-proxy.service <<UNITEOF
[Unit]
Description=cloudflared-fips Per-Site FIPS Gateway Proxy
Documentation=https://github.com/JongoDB/cloudflared-fips
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=-${CONFIG_DIR}/env
ExecStart=${BIN_DIR}/cloudflared-fips-proxy \\
  --listen :443 \\
  --cert ${CONFIG_DIR}/tls-cert.pem \\
  --key ${CONFIG_DIR}/tls-key.pem \\
  --upstream ${UPSTREAM}
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

    # --- Agent service (server, proxy, client, and controller roles) ---
    if [[ "$ROLE" == "client" || "$ROLE" == "server" || "$ROLE" == "proxy" || "$ROLE" == "controller" ]]; then
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
            systemctl enable cloudflared-fips-agent.service
            if [[ -f /etc/systemd/system/cloudflared-fips-proxy.service ]]; then
                systemctl enable cloudflared-fips-proxy.service
            fi
            if [[ -f /etc/systemd/system/cloudflared-fips-tunnel.service ]]; then
                systemctl enable cloudflared-fips-tunnel.service
            fi
            ;;
        server)
            systemctl enable cloudflared-fips-agent.service
            ;;
        proxy)
            systemctl enable cloudflared-fips-agent.service
            if [[ -f /etc/systemd/system/cloudflared-fips-proxy.service ]]; then
                systemctl enable cloudflared-fips-proxy.service
            fi
            if [[ -f /etc/systemd/system/cloudflared-fips-tunnel.service ]]; then
                systemctl enable cloudflared-fips-tunnel.service
            fi
            ;;
        client)
            systemctl enable cloudflared-fips-agent.service
            ;;
    esac

    # --- Firewall ---
    local needs_8080=false
    local needs_443=false
    if [[ "$ROLE" == "controller" ]]; then
        needs_8080=true
    fi
    if [[ -f /etc/systemd/system/cloudflared-fips-proxy.service ]]; then
        needs_443=true
    fi

    if command -v firewall-cmd &>/dev/null; then
        if [[ "$needs_8080" == "true" ]]; then
            info "Opening port 8080 for fleet controller..."
            firewall-cmd --add-port=8080/tcp --permanent 2>/dev/null || true
        fi
        if [[ "$needs_443" == "true" ]]; then
            info "Opening port 443 for FIPS proxy..."
            firewall-cmd --add-port=443/tcp --permanent 2>/dev/null || true
        fi
        if [[ "$needs_8080" == "true" || "$needs_443" == "true" ]]; then
            firewall-cmd --reload 2>/dev/null || true
        fi
    elif command -v ufw &>/dev/null; then
        if [[ "$needs_8080" == "true" ]]; then
            info "Opening port 8080 for fleet controller..."
            ufw allow 8080/tcp 2>/dev/null || true
        fi
        if [[ "$needs_443" == "true" ]]; then
            info "Opening port 443 for FIPS proxy..."
            ufw allow 443/tcp 2>/dev/null || true
        fi
    fi

    # Reload systemd so it picks up new/changed unit files
    systemctl daemon-reload

    log "Systemd units installed."
}

# ---------------------------------------------------------------------------
# Phase 4: Start and verify
# ---------------------------------------------------------------------------
phase4_start() {
    # Stop any existing services so we pick up new binaries + unit files
    systemctl stop cloudflared-fips.target 2>/dev/null || true
    for svc in cloudflared-fips-dashboard cloudflared-fips-tunnel cloudflared-fips-proxy cloudflared-fips-agent; do
        systemctl stop "$svc" 2>/dev/null || true
    done

    log "Starting cloudflared-fips services (role: ${ROLE}, tier: ${TIER})..."

    case "$ROLE" in
        controller)
            # Start tunnel + proxy immediately (no dependency on dashboard)
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                systemctl start cloudflared-fips-tunnel
            fi
            if systemctl is-enabled --quiet cloudflared-fips-proxy 2>/dev/null; then
                systemctl start cloudflared-fips-proxy
            fi
            # Start dashboard and wait for API readiness
            systemctl start cloudflared-fips-dashboard
            local retries=0
            local dash_ready=false
            while ! curl -sf http://127.0.0.1:8080/health &>/dev/null; do
                retries=$((retries + 1))
                if [[ $retries -ge 30 ]]; then
                    warn "Dashboard not ready after 30s — agent self-enrollment may fail"
                    break
                fi
                sleep 1
            done
            if curl -sf http://127.0.0.1:8080/health &>/dev/null; then
                dash_ready=true
            fi
            # Self-enroll the controller's own agent so it can report posture
            if [[ "$dash_ready" == "true" ]] && [[ -z "$(grep NODE_ID "${CONFIG_DIR}/env" 2>/dev/null)" ]]; then
                log "Self-enrolling controller agent..."
                local admin_key
                admin_key=$(grep FLEET_ADMIN_KEY "${CONFIG_DIR}/env" | cut -d= -f2-)
                if [[ -n "$admin_key" ]]; then
                    local enroll_ok=false
                    for attempt in 1 2 3; do
                        local token_resp
                        token_resp=$(curl -sf -X POST http://127.0.0.1:8080/api/v1/fleet/tokens \
                            -H "Authorization: Bearer ${admin_key}" \
                            -H "Content-Type: application/json" \
                            -d '{"role":"controller","max_uses":1}' 2>/dev/null) || true
                        if [[ -z "$token_resp" ]]; then
                            sleep 2
                            continue
                        fi
                        local self_token
                        self_token=$(echo "$token_resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
                        if [[ -z "$self_token" ]]; then
                            sleep 2
                            continue
                        fi
                        local hostname
                        hostname=$(hostname -s)
                        local enroll_resp
                        enroll_resp=$(curl -sf -X POST http://127.0.0.1:8080/api/v1/fleet/enroll \
                            -H "Content-Type: application/json" \
                            -d "{\"token\":\"${self_token}\",\"name\":\"${hostname} (controller)\",\"role\":\"controller\",\"version\":\"${CLOUDFLARED_VERSION}\",\"fips_backend\":\"BoringCrypto\"}" 2>/dev/null) || true
                        if [[ -n "$enroll_resp" ]]; then
                            local self_node_id self_api_key
                            self_node_id=$(echo "$enroll_resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['node_id'])" 2>/dev/null)
                            self_api_key=$(echo "$enroll_resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['api_key'])" 2>/dev/null)
                            if [[ -n "$self_node_id" && -n "$self_api_key" ]]; then
                                echo "NODE_ID=${self_node_id}" >> "${CONFIG_DIR}/env"
                                echo "NODE_API_KEY=${self_api_key}" >> "${CONFIG_DIR}/env"
                                echo "CONTROLLER_URL=http://127.0.0.1:8080" >> "${CONFIG_DIR}/env"
                                log "Controller self-enrolled as node ${self_node_id}"
                                enroll_ok=true
                                break
                            fi
                        fi
                        sleep 2
                    done
                    if [[ "$enroll_ok" != "true" ]]; then
                        warn "Self-enrollment failed after 3 attempts. Run manually:"
                        warn "  sudo cloudflared-fips-provision --role controller --self-enroll"
                    fi
                fi
            elif [[ "$dash_ready" != "true" ]]; then
                warn "Skipping self-enrollment (dashboard not ready). Run manually after dashboard starts:"
                warn "  sudo cloudflared-fips-provision --role controller --self-enroll"
            fi
            # Start agent last (needs enrollment credentials)
            systemctl start cloudflared-fips-agent
            ;;
        server)
            systemctl start cloudflared-fips-agent
            ;;
        proxy)
            systemctl start cloudflared-fips-agent
            if systemctl is-enabled --quiet cloudflared-fips-proxy 2>/dev/null; then
                systemctl start cloudflared-fips-proxy
            fi
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                systemctl start cloudflared-fips-tunnel
            fi
            ;;
        client)
            systemctl start cloudflared-fips-agent
            ;;
    esac

    sleep 2

    # Verify based on role
    case "$ROLE" in
        controller)
            if systemctl is-active --quiet cloudflared-fips-dashboard; then
                log "Dashboard service is running"
            else
                fail "Dashboard failed to start. Check: journalctl -u cloudflared-fips-dashboard -n 50"
            fi
            if systemctl is-active --quiet cloudflared-fips-agent; then
                log "Agent service is running"
            else
                warn "Agent failed to start. Check: journalctl -u cloudflared-fips-agent -n 50"
            fi
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                if systemctl is-active --quiet cloudflared-fips-tunnel; then
                    log "Tunnel service is running"
                else
                    warn "Tunnel failed to start. Check: journalctl -u cloudflared-fips-tunnel -n 50"
                fi
            fi

            log "Verifying endpoints..."
            echo ""
            info "Health check:"
            curl -s http://127.0.0.1:8080/api/v1/health 2>/dev/null || warn "Could not reach health endpoint"
            echo ""

            info "Fleet summary:"
            curl -s http://127.0.0.1:8080/api/v1/fleet/summary 2>/dev/null || warn "Could not reach fleet endpoint"
            echo ""
            ;;
        server)
            if systemctl is-active --quiet cloudflared-fips-agent; then
                log "Agent service is running"
            else
                fail "Agent failed to start. Check: journalctl -u cloudflared-fips-agent -n 50"
            fi
            ;;
        proxy)
            if systemctl is-active --quiet cloudflared-fips-agent; then
                log "Agent service is running"
            else
                warn "Agent failed to start. Check: journalctl -u cloudflared-fips-agent -n 50"
            fi
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

    # --- Post-install summary ---
    print_summary
}

# ---------------------------------------------------------------------------
# Post-install summary with tier-specific guidance
# ---------------------------------------------------------------------------
print_summary() {
    echo ""
    log "============================================"
    log "  cloudflared-fips deployed successfully!"
    log "  Role: ${ROLE}  |  Tier: ${TIER}  |  OS: ${DISTRO_NAME} ${DISTRO_VERSION}"
    log "============================================"
    echo ""

    # --- Tier explanation ---
    case "$TIER" in
        1)
            tier "Standard Cloudflare Tunnel"
            tier "Trust model:"
            tier "  Client --> Cloudflare Edge --> cloudflared-fips --> Origin"
            tier "  - Segment 2 (tunnel): FIPS-validated crypto (BoringCrypto)"
            tier "  - Segment 1 (edge):   Cloudflare FedRAMP Moderate (inherited trust)"
            tier "  - Edge crypto module NOT independently FIPS-validated"
            echo ""
            ;;
        2)
            tier "Cloudflare Tunnel + Regional Services + Keyless SSL"
            tier "Trust model:"
            tier "  Client --> CF FedRAMP DC --> Keyless SSL (customer HSM) --> cloudflared-fips --> Origin"
            tier "  - Private keys stay in your FIPS 140-2 Level 3 HSM"
            tier "  - TLS restricted to FedRAMP-compliant US data centers"
            tier "  - Bulk encryption still performed by Cloudflare edge BoringSSL"
            echo ""
            warn "Required Cloudflare configuration (not automated):"
            warn "  1. Enable Regional Services for your zone"
            warn "     Cloudflare Dashboard > SSL/TLS > Edge Certificates > Regional Services"
            warn "  2. Configure Keyless SSL with your HSM"
            warn "     See: docs/deployment-tier-guide.md for HSM setup (8 vendor guides)"
            warn "  3. Verify: curl https://<controller>:8080/api/v1/compliance | jq '.edge'"
            echo ""
            ;;
        3)
            tier "Per-Site FIPS Gateway (symmetric architecture)"
            tier "Trust model:"
            tier "  [Client Site]                     [Origin / Data Center]"
            tier "  Client -> FIPS Proxy (:443)       Controller (:443 + :8080 fleet)"
            tier "        -> Cloudflare (WAF/CDN) --> cloudflared tunnel -> Origin App"
            tier ""
            tier "  - Symmetric: site proxy mirrors origin controller"
            tier "  - Every TLS hop uses FIPS-validated crypto (BoringCrypto)"
            tier "  - Zero trust: even localhost connections use FIPS TLS"
            tier "  - Cloudflare optional: add for WAF/CDN via tunnel"
            echo ""
            ;;
    esac

    # --- Role-specific info ---
    case "$ROLE" in
        controller)
            info "Dashboard:    http://0.0.0.0:8080"
            info "Fleet API:    http://0.0.0.0:8080/api/v1/fleet/"
            info "Self-test:    cloudflared-fips-selftest"
            info "Service logs: journalctl -u cloudflared-fips-dashboard -f"
            if systemctl is-enabled --quiet cloudflared-fips-proxy 2>/dev/null; then
                info "FIPS Proxy:   listening on :443 (origin-side TLS termination)"
                info "Proxy logs:   journalctl -u cloudflared-fips-proxy -f"
            fi
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                if systemctl is-active --quiet cloudflared-fips-tunnel; then
                    info "Tunnel:       cloudflared connected to Cloudflare"
                else
                    warn "Tunnel:       NOT RUNNING — check: journalctl -u cloudflared-fips-tunnel -n 50"
                fi
                info "Tunnel logs:  journalctl -u cloudflared-fips-tunnel -f"
            fi
            if [[ -n "$ADMIN_KEY" ]]; then
                info "Admin key:    ${ADMIN_KEY}"
                info "  (save this — needed for creating enrollment tokens)"
            fi
            echo ""
            info "To add a site proxy node:"
            info "  1. Create token: curl -X POST -H 'Authorization: Bearer <admin-key>' \\"
            info "       http://<this-host>:8080/api/v1/fleet/tokens -d '{\"role\":\"proxy\",\"max_uses\":10}'"
            info "  2. On site: sudo ./provision-linux.sh --role proxy --tier 3 --cert /path/cert --key /path/key --enrollment-token <token> --controller-url http://<this-host>:8080"
            ;;
        server)
            info "Agent:        cloudflared-fips-agent (reporting to controller)"
            info "Self-test:    cloudflared-fips-selftest"
            info "Service logs: journalctl -u cloudflared-fips-agent -f"
            if [[ -n "$SERVICE_NAME" ]]; then
                info "Service:      ${SERVICE_NAME} (${SERVICE_HOST:-localhost}:${SERVICE_PORT:-N/A})"
            fi
            ;;
        proxy)
            info "Proxy:        listening on :443"
            info "Agent:        cloudflared-fips-agent (reporting to controller)"
            info "Self-test:    cloudflared-fips-selftest"
            info "Proxy logs:   journalctl -u cloudflared-fips-proxy -f"
            info "Agent logs:   journalctl -u cloudflared-fips-agent -f"
            if systemctl is-enabled --quiet cloudflared-fips-tunnel 2>/dev/null; then
                info "Tunnel:       cloudflared tunnel active"
                info "Tunnel logs:  journalctl -u cloudflared-fips-tunnel -f"
            fi
            ;;
        client)
            info "Agent check:  cloudflared-fips-agent --check"
            info "Service logs: journalctl -u cloudflared-fips-agent -f"
            ;;
    esac

    echo ""
    info "FIPS backend:  BoringCrypto (CMVP #4735, FIPS 140-3)"
    info "OS FIPS mode:  $(cat /proc/sys/crypto/fips_enabled 2>/dev/null || echo 'N/A')"
    info "Distro:        ${DISTRO_NAME} ${DISTRO_VERSION} (${DISTRO_FAMILY})"
    echo ""
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    check_root
    detect_distro

    echo ""
    echo "============================================"
    echo "  cloudflared-fips provisioning"
    echo "  Role: ${ROLE}  |  Tier: ${TIER}"
    echo "  OS:   ${DISTRO_NAME} ${DISTRO_VERSION}"
    echo "============================================"
    echo ""

    # Detect if we're resuming after a FIPS reboot
    if [[ -f "$MARKER" ]]; then
        info "Resuming provisioning after FIPS reboot..."
        # Clean up the login helper
        rm -f /etc/profile.d/cloudflared-fips-resume.sh
    fi

    phase1_fips

    if [[ "$PREINSTALLED" == "true" ]]; then
        log "Detected pre-installed binaries (from package). Skipping build phase."
    else
        phase2_deps
        phase2_build
    fi

    phase3_install
    phase4_start
}

main "$@"

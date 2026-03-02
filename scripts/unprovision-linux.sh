#!/usr/bin/env bash
# unprovision-linux.sh — Cleanly remove cloudflared-fips from a Linux system
#
# Usage:
#   sudo ./scripts/unprovision-linux.sh                    # interactive: show what will be removed, ask confirmation
#   sudo ./scripts/unprovision-linux.sh --yes              # non-interactive: remove everything without prompting
#   sudo ./scripts/unprovision-linux.sh --dry-run          # show what would be removed without doing it
#   sudo ./scripts/unprovision-linux.sh --keep-config      # preserve /etc/cloudflared-fips/ for reprovision
#   sudo ./scripts/unprovision-linux.sh --keep-data        # preserve /var/lib/cloudflared-fips/ (fleet DB)
#   sudo ./scripts/unprovision-linux.sh --purge            # also remove Go, Node.js, /opt/cloudflared-fips repo
#   sudo ./scripts/unprovision-linux.sh --teardown-cf \
#       --cf-api-token TOK --cf-account-id ID --cf-tunnel-id TID  # also delete CF tunnel + DNS
#
# This script is the inverse of provision-linux.sh. It stops services,
# removes systemd units, binaries, config, data, firewall rules, and
# optionally tears down Cloudflare resources.

set -euo pipefail

# ---------------------------------------------------------------------------
# Config — must match provision-linux.sh
# ---------------------------------------------------------------------------
CONFIG_DIR="/etc/cloudflared-fips"
DATA_DIR="/var/lib/cloudflared-fips"
BIN_DIR="/usr/local/bin"
INSTALL_DIR="/opt/cloudflared-fips"
SERVICE_USER="cloudflared"
MARKER="/var/tmp/.cloudflared-fips-provision-phase"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
ROLE=""
KEEP_CONFIG=false
KEEP_DATA=false
KEEP_BINARIES=false
KEEP_USER=false
PURGE=false
TEARDOWN_CF=false
CF_API_TOKEN=""
CF_ACCOUNT_ID=""
CF_TUNNEL_ID=""
CF_ZONE_ID=""
YES=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)            ROLE="$2"; shift 2 ;;
        --keep-config)     KEEP_CONFIG=true; shift ;;
        --keep-data)       KEEP_DATA=true; shift ;;
        --keep-binaries)   KEEP_BINARIES=true; shift ;;
        --keep-user)       KEEP_USER=true; shift ;;
        --purge)           PURGE=true; shift ;;
        --teardown-cf)     TEARDOWN_CF=true; shift ;;
        --cf-api-token)    CF_API_TOKEN="$2"; shift 2 ;;
        --cf-account-id)   CF_ACCOUNT_ID="$2"; shift 2 ;;
        --cf-tunnel-id)    CF_TUNNEL_ID="$2"; shift 2 ;;
        --cf-zone-id)      CF_ZONE_ID="$2"; shift 2 ;;
        --yes)             YES=true; shift ;;
        --dry-run)         DRY_RUN=true; shift ;;
        --help|-h)
            echo "Usage: sudo $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --role ROLE          Node role (auto-detected from env file if not set)"
            echo "  --keep-config        Don't delete /etc/cloudflared-fips/"
            echo "  --keep-data          Don't delete /var/lib/cloudflared-fips/"
            echo "  --keep-binaries      Don't delete binaries from /usr/local/bin/"
            echo "  --keep-user          Don't delete the cloudflared system user"
            echo "  --purge              Also remove Go, Node.js, and /opt/cloudflared-fips repo"
            echo "  --teardown-cf        Delete Cloudflare tunnel + DNS records via API"
            echo "  --cf-api-token TOK   CF API token (required for --teardown-cf)"
            echo "  --cf-account-id ID   CF account ID (required for --teardown-cf)"
            echo "  --cf-tunnel-id ID    CF tunnel ID (required for --teardown-cf)"
            echo "  --cf-zone-id ID      CF zone ID (for DNS cleanup with --teardown-cf)"
            echo "  --yes                Skip confirmation prompt"
            echo "  --dry-run            Show what would be removed without doing it"
            echo "  --help, -h           Show this help"
            exit 0
            ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# ---------------------------------------------------------------------------
# Helpers — same colors and style as provision-linux.sh
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

# Run a command, or print what would happen in dry-run mode
run() {
    if $DRY_RUN; then
        info "Would run: $*"
    else
        "$@"
    fi
}

# ---------------------------------------------------------------------------
# Phase 1: Discovery — auto-detect what's installed
# ---------------------------------------------------------------------------
FOUND_SERVICES=()
FOUND_BINARIES=()
FOUND_CONFIG=false
FOUND_DATA=false
FOUND_USER=false
FOUND_FIREWALL_CMD=false
FOUND_UFW=false
FOUND_PROFILE=false
FOUND_MARKER=false
FOUND_PLIST=false
FOUND_INSTALL_DIR=false
FOUND_GO=false
FOUND_NODE=false

discover() {
    info "Discovering installed components..."

    # Detect role from env file if not specified
    if [[ -z "$ROLE" && -f "${CONFIG_DIR}/env" ]]; then
        # shellcheck source=/dev/null
        ROLE=$(grep -oP '(?<=^ROLE=).*' "${CONFIG_DIR}/env" 2>/dev/null || true)
        if [[ -n "$ROLE" ]]; then
            info "Auto-detected role: $ROLE"
        fi
    fi

    # Check systemd units
    for svc in dashboard tunnel proxy agent; do
        if [[ -f "/etc/systemd/system/cloudflared-fips-${svc}.service" ]]; then
            FOUND_SERVICES+=("$svc")
        fi
    done
    if [[ -f "/etc/systemd/system/cloudflared-fips.target" ]]; then
        FOUND_SERVICES+=("target")
    fi

    # Check binaries (unified CLI + individual binaries)
    if [[ -f "${BIN_DIR}/cloudflared-fips" ]]; then
        FOUND_BINARIES+=("unified")
    fi
    for bin in selftest dashboard agent proxy tui provision unprovision; do
        if [[ -f "${BIN_DIR}/cloudflared-fips-${bin}" ]]; then
            FOUND_BINARIES+=("$bin")
        fi
    done

    # Check directories and user
    # Note: use `|| true` to prevent set -e from killing the script when
    # a test returns false (the && expression returns non-zero).
    [[ -d "$CONFIG_DIR" ]] && FOUND_CONFIG=true || true
    [[ -d "$DATA_DIR" ]] && FOUND_DATA=true || true
    id "$SERVICE_USER" &>/dev/null 2>&1 && FOUND_USER=true || true
    command -v firewall-cmd &>/dev/null && FOUND_FIREWALL_CMD=true || true
    command -v ufw &>/dev/null && FOUND_UFW=true || true
    [[ -f "/etc/profile.d/cloudflared-fips-resume.sh" ]] && FOUND_PROFILE=true || true
    [[ -f "$MARKER" ]] && FOUND_MARKER=true || true
    [[ -d "$INSTALL_DIR" ]] && FOUND_INSTALL_DIR=true || true
    [[ -d "/usr/local/go" ]] && FOUND_GO=true || true
    [[ -d "/usr/local/node" || -f "/usr/local/bin/node" ]] && FOUND_NODE=true || true
}

# ---------------------------------------------------------------------------
# Phase 2: Show summary and confirm
# ---------------------------------------------------------------------------
show_summary() {
    echo ""
    echo "============================================"
    echo "  cloudflared-fips — Unprovision Summary"
    echo "============================================"
    echo ""

    if [[ ${#FOUND_SERVICES[@]} -gt 0 ]]; then
        log "Systemd services: ${FOUND_SERVICES[*]}"
    else
        info "No systemd services found"
    fi

    if [[ ${#FOUND_BINARIES[@]} -gt 0 ]]; then
        if $KEEP_BINARIES; then
            info "Binaries (KEPT): ${FOUND_BINARIES[*]}"
        else
            log "Binaries to remove: ${FOUND_BINARIES[*]}"
        fi
    else
        info "No binaries found"
    fi

    if $FOUND_CONFIG; then
        if $KEEP_CONFIG; then
            info "Config dir (KEPT): $CONFIG_DIR"
        else
            log "Config dir: $CONFIG_DIR"
        fi
    fi

    if $FOUND_DATA; then
        if $KEEP_DATA; then
            info "Data dir (KEPT): $DATA_DIR"
        else
            log "Data dir: $DATA_DIR"
        fi
    fi

    if $FOUND_USER; then
        if $KEEP_USER; then
            info "System user (KEPT): $SERVICE_USER"
        else
            log "System user: $SERVICE_USER"
        fi
    fi

    if $FOUND_FIREWALL_CMD || $FOUND_UFW; then
        log "Firewall rules: ports 8080/tcp, 443/tcp"
    fi

    if $FOUND_PROFILE; then
        log "Profile script: /etc/profile.d/cloudflared-fips-resume.sh"
    fi

    if $FOUND_MARKER; then
        log "Provision marker: $MARKER"
    fi

    if $PURGE; then
        echo ""
        warn "PURGE mode: will also remove:"
        $FOUND_INSTALL_DIR && warn "  - $INSTALL_DIR (git repo + build output)"
        $FOUND_GO && warn "  - /usr/local/go/ (Go installation)"
        $FOUND_NODE && warn "  - Node.js (/usr/local/node, /usr/local/bin/node)"
        [[ -f "/etc/profile.d/go.sh" ]] && warn "  - /etc/profile.d/go.sh"
    fi

    if $TEARDOWN_CF; then
        echo ""
        warn "Cloudflare teardown: will delete tunnel and DNS records"
        [[ -n "$CF_TUNNEL_ID" ]] && warn "  Tunnel ID: $CF_TUNNEL_ID"
        [[ -n "$CF_ZONE_ID" ]] && warn "  Zone ID: $CF_ZONE_ID (DNS cleanup)"
    fi

    echo ""
    info "Upstream cloudflared binary (/usr/local/bin/cloudflared) will NOT be removed"
    echo ""
}

confirm() {
    if $YES || $DRY_RUN; then
        return 0
    fi

    read -rp "Continue with unprovision? [y/N] " answer
    case "$answer" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) echo "Aborted."; exit 0 ;;
    esac
}

# ---------------------------------------------------------------------------
# Phase 3: Stop & Disable Services
# ---------------------------------------------------------------------------
stop_services() {
    if [[ ${#FOUND_SERVICES[@]} -eq 0 ]]; then
        info "No services to stop"
        return
    fi

    log "Stopping and disabling services..."

    run systemctl stop cloudflared-fips.target 2>/dev/null || true
    run systemctl disable cloudflared-fips.target 2>/dev/null || true

    for svc in dashboard tunnel proxy agent; do
        run systemctl stop "cloudflared-fips-${svc}.service" 2>/dev/null || true
        run systemctl disable "cloudflared-fips-${svc}.service" 2>/dev/null || true
    done
}

# ---------------------------------------------------------------------------
# Phase 4: Remove systemd units
# ---------------------------------------------------------------------------
remove_units() {
    if [[ ${#FOUND_SERVICES[@]} -eq 0 ]]; then
        return
    fi

    log "Removing systemd units..."

    for svc in dashboard tunnel proxy agent; do
        if [[ -f "/etc/systemd/system/cloudflared-fips-${svc}.service" ]]; then
            run rm -f "/etc/systemd/system/cloudflared-fips-${svc}.service"
        fi
    done

    if [[ -f "/etc/systemd/system/cloudflared-fips.target" ]]; then
        run rm -f "/etc/systemd/system/cloudflared-fips.target"
    fi

    run systemctl daemon-reload 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Phase 5: Remove binaries
# ---------------------------------------------------------------------------
remove_binaries() {
    if $KEEP_BINARIES; then
        info "Keeping binaries (--keep-binaries)"
        return
    fi

    if [[ ${#FOUND_BINARIES[@]} -eq 0 ]]; then
        info "No binaries to remove"
        return
    fi

    log "Removing binaries..."

    # Unified CLI binary
    if [[ -f "${BIN_DIR}/cloudflared-fips" ]]; then
        run rm -f "${BIN_DIR}/cloudflared-fips"
    fi

    # Individual binaries
    for bin in selftest dashboard agent proxy tui provision unprovision; do
        local path="${BIN_DIR}/cloudflared-fips-${bin}"
        if [[ -f "$path" ]]; then
            run rm -f "$path"
        fi
    done

    # Note: does NOT remove /usr/local/bin/cloudflared (upstream binary)
}

# ---------------------------------------------------------------------------
# Phase 6: Remove configuration
# ---------------------------------------------------------------------------
remove_config() {
    if $KEEP_CONFIG; then
        info "Keeping config (--keep-config)"
        return
    fi

    if ! $FOUND_CONFIG; then
        return
    fi

    log "Removing configuration: $CONFIG_DIR"
    run rm -rf "$CONFIG_DIR"
}

# ---------------------------------------------------------------------------
# Phase 7: Remove data
# ---------------------------------------------------------------------------
remove_data() {
    if $KEEP_DATA; then
        info "Keeping data (--keep-data)"
        return
    fi

    if ! $FOUND_DATA; then
        return
    fi

    log "Removing data: $DATA_DIR"
    run rm -rf "$DATA_DIR"
}

# ---------------------------------------------------------------------------
# Phase 8: Remove system user
# ---------------------------------------------------------------------------
remove_user() {
    if $KEEP_USER; then
        info "Keeping user (--keep-user)"
        return
    fi

    if ! $FOUND_USER; then
        return
    fi

    log "Removing system user: $SERVICE_USER"
    run userdel "$SERVICE_USER" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Phase 9: Remove firewall rules
# ---------------------------------------------------------------------------
remove_firewall() {
    if $FOUND_FIREWALL_CMD; then
        log "Removing firewall-cmd rules..."
        run firewall-cmd --remove-port=8080/tcp --permanent 2>/dev/null || true
        run firewall-cmd --remove-port=443/tcp --permanent 2>/dev/null || true
        run firewall-cmd --reload 2>/dev/null || true
    fi

    if $FOUND_UFW; then
        log "Removing ufw rules..."
        run ufw delete allow 8080/tcp 2>/dev/null || true
        run ufw delete allow 443/tcp 2>/dev/null || true
    fi
}

# ---------------------------------------------------------------------------
# Phase 10: Remove profile scripts
# ---------------------------------------------------------------------------
remove_profile() {
    if $FOUND_PROFILE; then
        log "Removing profile script: /etc/profile.d/cloudflared-fips-resume.sh"
        run rm -f /etc/profile.d/cloudflared-fips-resume.sh
    fi
}

# ---------------------------------------------------------------------------
# Phase 11: Remove provision markers & temp files
# ---------------------------------------------------------------------------
remove_markers() {
    if $FOUND_MARKER; then
        log "Removing provision marker: $MARKER"
        run rm -f "$MARKER"
    fi

    # Clean up any stale Unix sockets
    local sockets
    sockets=$(find /tmp -maxdepth 1 -name 'cloudflared-fips-*.sock' 2>/dev/null || true)
    if [[ -n "$sockets" ]]; then
        log "Removing temp sockets..."
        if $DRY_RUN; then
            echo "$sockets" | while read -r s; do info "Would remove: $s"; done
        else
            echo "$sockets" | while read -r s; do rm -f "$s"; done
        fi
    fi
}

# ---------------------------------------------------------------------------
# Phase 12: Purge (only if --purge)
# ---------------------------------------------------------------------------
purge() {
    if ! $PURGE; then
        return
    fi

    warn "Purging development dependencies..."

    if $FOUND_INSTALL_DIR; then
        log "Removing repo: $INSTALL_DIR"
        run rm -rf "$INSTALL_DIR"
    fi

    if $FOUND_GO; then
        log "Removing Go: /usr/local/go/"
        run rm -rf /usr/local/go/
        if [[ -f /etc/profile.d/go.sh ]]; then
            run rm -f /etc/profile.d/go.sh
        fi
    fi

    if $FOUND_NODE; then
        log "Removing Node.js..."
        run rm -rf /usr/local/node/
        run rm -f /usr/local/bin/node /usr/local/bin/npm /usr/local/bin/npx
    fi
}

# ---------------------------------------------------------------------------
# Phase 13: Cloudflare teardown (only if --teardown-cf)
# ---------------------------------------------------------------------------
teardown_cloudflare() {
    if ! $TEARDOWN_CF; then
        return
    fi

    if [[ -z "$CF_API_TOKEN" ]]; then
        warn "Cloudflare teardown requires --cf-api-token"
        return
    fi

    if [[ -z "$CF_ACCOUNT_ID" || -z "$CF_TUNNEL_ID" ]]; then
        warn "Cloudflare teardown requires --cf-account-id and --cf-tunnel-id"
        return
    fi

    log "Tearing down Cloudflare resources..."

    # Try dashboard binary first (has proper API client)
    if [[ -x "${BIN_DIR}/cloudflared-fips-dashboard" ]] && ! $DRY_RUN; then
        log "Using dashboard binary for teardown..."
        "${BIN_DIR}/cloudflared-fips-dashboard" --teardown-tunnel \
            --cf-api-token "$CF_API_TOKEN" \
            --cf-account-id "$CF_ACCOUNT_ID" \
            --cf-tunnel-id "$CF_TUNNEL_ID" \
            ${CF_ZONE_ID:+--cf-zone-id "$CF_ZONE_ID"} 2>&1 || {
            warn "Dashboard teardown failed, falling back to curl..."
            teardown_cloudflare_curl
        }
        return
    fi

    if $DRY_RUN; then
        info "Would delete Cloudflare tunnel: $CF_TUNNEL_ID"
        [[ -n "$CF_ZONE_ID" ]] && info "Would clean up DNS records in zone: $CF_ZONE_ID"
        return
    fi

    teardown_cloudflare_curl
}

# Fallback: use curl directly for CF API calls
teardown_cloudflare_curl() {
    local api="https://api.cloudflare.com/client/v4"
    local auth="Authorization: Bearer ${CF_API_TOKEN}"

    # Delete DNS records (if zone ID provided)
    if [[ -n "$CF_ZONE_ID" ]]; then
        log "Looking up DNS records pointing to tunnel..."
        local dns_response
        dns_response=$(curl -sf -H "$auth" \
            "${api}/zones/${CF_ZONE_ID}/dns_records?type=CNAME&content=${CF_TUNNEL_ID}.cfargotunnel.com" 2>/dev/null || echo '{"result":[]}')

        local record_ids
        record_ids=$(echo "$dns_response" | grep -oP '"id"\s*:\s*"\K[^"]+' || true)

        if [[ -n "$record_ids" ]]; then
            while read -r rid; do
                log "Deleting DNS record: $rid"
                curl -sf -X DELETE -H "$auth" \
                    "${api}/zones/${CF_ZONE_ID}/dns_records/${rid}" >/dev/null 2>&1 || {
                    warn "Failed to delete DNS record: $rid"
                }
            done <<< "$record_ids"
        else
            info "No DNS records found for tunnel"
        fi
    fi

    # Delete tunnel
    log "Deleting Cloudflare tunnel: $CF_TUNNEL_ID"
    local tun_response
    tun_response=$(curl -sf -X DELETE -H "$auth" \
        "${api}/accounts/${CF_ACCOUNT_ID}/cfd_tunnel/${CF_TUNNEL_ID}" 2>/dev/null || echo "")

    if echo "$tun_response" | grep -q '"success":true'; then
        log "Tunnel deleted successfully"
    else
        warn "Tunnel deletion may have failed — check Cloudflare dashboard"
        warn "Response: $tun_response"
    fi
}

# ---------------------------------------------------------------------------
# Phase 14: Summary
# ---------------------------------------------------------------------------
show_result() {
    local svc_count=${#FOUND_SERVICES[@]}
    local bin_count=${#FOUND_BINARIES[@]}

    echo ""
    echo "============================================"
    if $DRY_RUN; then
        echo "  cloudflared-fips — Dry Run Complete"
    else
        echo "  cloudflared-fips unprovisioned"
    fi
    echo "============================================"

    local removed=()
    local kept=()

    [[ $svc_count -gt 0 ]] && removed+=("${svc_count} services")
    if ! $KEEP_BINARIES && [[ $bin_count -gt 0 ]]; then
        removed+=("${bin_count} binaries")
    elif $KEEP_BINARIES && [[ $bin_count -gt 0 ]]; then
        kept+=("binaries")
    fi

    if ! $KEEP_CONFIG && $FOUND_CONFIG; then
        removed+=("config")
    elif $KEEP_CONFIG && $FOUND_CONFIG; then
        kept+=("config")
    fi

    if ! $KEEP_DATA && $FOUND_DATA; then
        removed+=("data")
    elif $KEEP_DATA && $FOUND_DATA; then
        kept+=("data")
    fi

    if ! $KEEP_USER && $FOUND_USER; then
        removed+=("user")
    elif $KEEP_USER && $FOUND_USER; then
        kept+=("user")
    fi

    $TEARDOWN_CF && removed+=("CF tunnel")
    $PURGE && removed+=("dev deps (Go/Node/repo)")

    if [[ ${#removed[@]} -gt 0 ]]; then
        log "Removed: $(printf '%s' "${removed[0]}"; printf ', %s' "${removed[@]:1}")"
    else
        info "Nothing to remove"
    fi

    if [[ ${#kept[@]} -gt 0 ]]; then
        info "Kept: $(printf '%s' "${kept[0]}"; printf ', %s' "${kept[@]:1}")"
    fi

    info "Kept: /usr/local/bin/cloudflared (upstream)"
    echo ""
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    check_root
    discover
    show_summary
    confirm

    if $DRY_RUN; then
        info "=== DRY RUN — no changes will be made ==="
        echo ""
    fi

    stop_services
    remove_units
    remove_binaries
    remove_config
    remove_data
    remove_user
    remove_firewall
    remove_profile
    remove_markers
    purge
    teardown_cloudflare
    show_result
}

main

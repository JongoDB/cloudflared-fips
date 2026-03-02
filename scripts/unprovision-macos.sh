#!/usr/bin/env bash
# unprovision-macos.sh — Cleanly remove cloudflared-fips from a macOS system
#
# Usage:
#   sudo ./scripts/unprovision-macos.sh                    # interactive: show what will be removed, ask confirmation
#   sudo ./scripts/unprovision-macos.sh --yes              # non-interactive: remove everything without prompting
#   sudo ./scripts/unprovision-macos.sh --dry-run          # show what would be removed without doing it
#   sudo ./scripts/unprovision-macos.sh --keep-config      # preserve /etc/cloudflared-fips/
#   sudo ./scripts/unprovision-macos.sh --keep-data        # preserve /var/lib/cloudflared-fips/
#   sudo ./scripts/unprovision-macos.sh --purge            # also remove Go, /opt/cloudflared-fips repo
#   sudo ./scripts/unprovision-macos.sh --teardown-cf \
#       --cf-api-token TOK --cf-account-id ID --cf-tunnel-id TID
#
# This script is the inverse of provision-macos.sh. It unloads launchd plists,
# removes binaries, config, data, and optionally tears down Cloudflare resources.
#
# macOS differences from Linux:
#   - launchd (launchctl) instead of systemd
#   - No firewall rules modified by provision
#   - No system user created (macOS doesn't create one)
#   - Plists in /Library/LaunchDaemons/com.cloudflared-fips.*.plist
#   - Log files in /var/log/com.cloudflared-fips.*.{log,err}

set -euo pipefail

# ---------------------------------------------------------------------------
# Config — must match provision-macos.sh
# ---------------------------------------------------------------------------
CONFIG_DIR="/etc/cloudflared-fips"
DATA_DIR="/var/lib/cloudflared-fips"
BIN_DIR="/usr/local/bin"
INSTALL_DIR="/opt/cloudflared-fips"
PLIST_DIR="/Library/LaunchDaemons"
SERVICE_PREFIX="com.cloudflared-fips"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
ROLE=""
KEEP_CONFIG=false
KEEP_DATA=false
KEEP_BINARIES=false
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
            echo "  --purge              Also remove Go and /opt/cloudflared-fips repo"
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
# Helpers — same colors and style as provision-macos.sh
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

check_macos() {
    if [[ "$(uname -s)" != "Darwin" ]]; then
        fail "This script is for macOS only. Use unprovision-linux.sh on Linux."
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
# Phase 1: Discovery
# ---------------------------------------------------------------------------
FOUND_PLISTS=()
FOUND_BINARIES=()
FOUND_LOGS=()
FOUND_CONFIG=false
FOUND_DATA=false
FOUND_INSTALL_DIR=false
FOUND_GO=false

discover() {
    info "Discovering installed components..."

    # Detect role from env file if not specified
    if [[ -z "$ROLE" && -f "${CONFIG_DIR}/env" ]]; then
        ROLE=$(grep -oP '(?<=^ROLE=).*' "${CONFIG_DIR}/env" 2>/dev/null || true)
        if [[ -n "$ROLE" ]]; then
            info "Auto-detected role: $ROLE"
        fi
    fi

    # Check launchd plists
    for svc in dashboard tunnel proxy agent; do
        if [[ -f "${PLIST_DIR}/${SERVICE_PREFIX}.${svc}.plist" ]]; then
            FOUND_PLISTS+=("$svc")
        fi
    done

    # Check binaries
    for bin in selftest dashboard agent proxy tui provision unprovision; do
        if [[ -f "${BIN_DIR}/cloudflared-fips-${bin}" ]]; then
            FOUND_BINARIES+=("$bin")
        fi
    done

    # Check log files
    for logfile in "${SERVICE_PREFIX}".*.log "${SERVICE_PREFIX}".*.err; do
        if [[ -f "/var/log/${logfile}" ]]; then
            FOUND_LOGS+=("/var/log/${logfile}")
        fi
    done

    # Check directories
    [[ -d "$CONFIG_DIR" ]] && FOUND_CONFIG=true
    [[ -d "$DATA_DIR" ]] && FOUND_DATA=true
    [[ -d "$INSTALL_DIR" ]] && FOUND_INSTALL_DIR=true
    [[ -d "/usr/local/go" ]] && FOUND_GO=true
}

# ---------------------------------------------------------------------------
# Phase 2: Show summary and confirm
# ---------------------------------------------------------------------------
show_summary() {
    echo ""
    echo "============================================"
    echo "  cloudflared-fips — Unprovision Summary"
    echo "  (macOS)"
    echo "============================================"
    echo ""

    if [[ ${#FOUND_PLISTS[@]} -gt 0 ]]; then
        log "LaunchDaemons: ${FOUND_PLISTS[*]}"
    else
        info "No LaunchDaemon plists found"
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

    if [[ ${#FOUND_LOGS[@]} -gt 0 ]]; then
        log "Log files: ${#FOUND_LOGS[@]} files"
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

    if $PURGE; then
        echo ""
        warn "PURGE mode: will also remove:"
        $FOUND_INSTALL_DIR && warn "  - $INSTALL_DIR (git repo + build output)"
        $FOUND_GO && warn "  - /usr/local/go/ (Go installation)"
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
# Phase 3: Unload & remove launchd plists
# ---------------------------------------------------------------------------
stop_services() {
    if [[ ${#FOUND_PLISTS[@]} -eq 0 ]]; then
        info "No services to stop"
        return
    fi

    log "Unloading launchd services..."

    for svc in dashboard tunnel proxy agent; do
        local plist="${PLIST_DIR}/${SERVICE_PREFIX}.${svc}.plist"
        if [[ -f "$plist" ]]; then
            run launchctl unload -w "$plist" 2>/dev/null || true
            run rm -f "$plist"
        fi
    done
}

# ---------------------------------------------------------------------------
# Phase 4: Remove binaries
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

    for bin in selftest dashboard agent proxy tui provision unprovision; do
        local path="${BIN_DIR}/cloudflared-fips-${bin}"
        if [[ -f "$path" ]]; then
            run rm -f "$path"
        fi
    done
}

# ---------------------------------------------------------------------------
# Phase 5: Remove log files
# ---------------------------------------------------------------------------
remove_logs() {
    if [[ ${#FOUND_LOGS[@]} -eq 0 ]]; then
        return
    fi

    log "Removing log files..."
    for logfile in "${FOUND_LOGS[@]}"; do
        run rm -f "$logfile"
    done
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
# Phase 8: Remove temp files
# ---------------------------------------------------------------------------
remove_temp() {
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
# Phase 9: Purge (only if --purge)
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
    fi
}

# ---------------------------------------------------------------------------
# Phase 10: Cloudflare teardown (only if --teardown-cf)
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

    # Try dashboard binary first
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
        record_ids=$(echo "$dns_response" | python3 -c "import sys,json; [print(r['id']) for r in json.load(sys.stdin).get('result',[])]" 2>/dev/null || true)

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
    fi
}

# ---------------------------------------------------------------------------
# Phase 11: Summary
# ---------------------------------------------------------------------------
show_result() {
    local plist_count=${#FOUND_PLISTS[@]}
    local bin_count=${#FOUND_BINARIES[@]}

    echo ""
    echo "============================================"
    if $DRY_RUN; then
        echo "  cloudflared-fips — Dry Run Complete"
    else
        echo "  cloudflared-fips unprovisioned (macOS)"
    fi
    echo "============================================"

    local removed=()
    local kept=()

    [[ $plist_count -gt 0 ]] && removed+=("${plist_count} services")
    if ! $KEEP_BINARIES && [[ $bin_count -gt 0 ]]; then
        removed+=("${bin_count} binaries")
    elif $KEEP_BINARIES && [[ $bin_count -gt 0 ]]; then
        kept+=("binaries")
    fi

    [[ ${#FOUND_LOGS[@]} -gt 0 ]] && removed+=("log files")

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

    $TEARDOWN_CF && removed+=("CF tunnel")
    $PURGE && removed+=("dev deps (Go/repo)")

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
    check_macos
    check_root
    discover
    show_summary
    confirm

    if $DRY_RUN; then
        info "=== DRY RUN — no changes will be made ==="
        echo ""
    fi

    stop_services
    remove_binaries
    remove_logs
    remove_config
    remove_data
    remove_temp
    purge
    teardown_cloudflare
    show_result
}

main

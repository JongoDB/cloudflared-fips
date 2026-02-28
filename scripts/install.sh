#!/usr/bin/env bash
# install.sh — Universal installer for cloudflared-fips
#
# Detects the operating system and runs the appropriate provisioning script.
# All flags are passed through to the platform-specific script.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/JongoDB/cloudflared-fips/main/scripts/install.sh | sudo bash
#   curl -fsSL .../install.sh | sudo bash -s -- --role client --enrollment-token TOKEN --controller-url URL
#   curl -fsSL .../install.sh | sudo bash -s -- --role server --tier 3 --cert /path/cert.pem --key /path/key.pem
#
# Windows users: Use PowerShell instead:
#   irm https://raw.githubusercontent.com/JongoDB/cloudflared-fips/main/scripts/provision-windows.ps1 -OutFile provision-windows.ps1
#   .\provision-windows.ps1 -Role client -EnrollmentToken TOKEN -ControllerUrl URL

set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
REPO_URL="https://github.com/JongoDB/cloudflared-fips.git"
RAW_BASE="https://raw.githubusercontent.com/JongoDB/cloudflared-fips/main/scripts"
INSTALL_DIR="/opt/cloudflared-fips"

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

# ---------------------------------------------------------------------------
# Detect OS
# ---------------------------------------------------------------------------
detect_os() {
    case "$(uname -s)" in
        Linux)   echo "linux" ;;
        Darwin)  echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            echo "windows"
            ;;
        *)
            fail "Unsupported OS: $(uname -s)"
            ;;
    esac
}

# ---------------------------------------------------------------------------
# Detect Linux distro
# ---------------------------------------------------------------------------
detect_linux_distro() {
    if [[ ! -f /etc/os-release ]]; then
        fail "Cannot detect distro: /etc/os-release not found"
    fi

    # shellcheck source=/dev/null
    . /etc/os-release
    echo "${ID}"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    local OS
    OS=$(detect_os)

    echo ""
    echo "============================================"
    echo "  cloudflared-fips universal installer"
    echo "  OS: ${OS}"
    echo "============================================"
    echo ""

    case "$OS" in
        linux)
            local DISTRO
            DISTRO=$(detect_linux_distro)
            log "Detected Linux distro: ${DISTRO}"
            log "Running Linux provisioning script..."
            echo ""

            # If we're running from a pipe (curl | bash), download the script
            if [[ -f "$(dirname "$0")/provision-linux.sh" ]]; then
                exec bash "$(dirname "$0")/provision-linux.sh" "$@"
            elif [[ -d "$INSTALL_DIR" && -f "${INSTALL_DIR}/scripts/provision-linux.sh" ]]; then
                exec bash "${INSTALL_DIR}/scripts/provision-linux.sh" "$@"
            else
                log "Downloading provision script..."
                local tmp_script
                tmp_script=$(mktemp /tmp/provision-linux-XXXXXX.sh)
                curl -fsSL "${RAW_BASE}/provision-linux.sh" -o "$tmp_script"
                chmod +x "$tmp_script"
                exec bash "$tmp_script" "$@"
            fi
            ;;

        darwin)
            log "Detected macOS"
            log "Running macOS provisioning script..."
            echo ""

            if [[ -f "$(dirname "$0")/provision-macos.sh" ]]; then
                exec bash "$(dirname "$0")/provision-macos.sh" "$@"
            elif [[ -d "$INSTALL_DIR" && -f "${INSTALL_DIR}/scripts/provision-macos.sh" ]]; then
                exec bash "${INSTALL_DIR}/scripts/provision-macos.sh" "$@"
            else
                log "Downloading macOS provision script..."
                local tmp_script
                tmp_script=$(mktemp /tmp/provision-macos-XXXXXX.sh)
                curl -fsSL "${RAW_BASE}/provision-macos.sh" -o "$tmp_script"
                chmod +x "$tmp_script"
                exec bash "$tmp_script" "$@"
            fi
            ;;

        windows)
            echo ""
            warn "Windows detected, but this script runs in bash."
            warn "Please use PowerShell instead:"
            echo ""
            info "  # Download and run:"
            info "  irm ${RAW_BASE}/provision-windows.ps1 -OutFile provision-windows.ps1"
            info "  .\\provision-windows.ps1 -Role client -EnrollmentToken TOKEN -ControllerUrl URL"
            echo ""
            info "  # Or for server role:"
            info "  .\\provision-windows.ps1 -Role server -Tier 3 -TlsCert C:\\certs\\cert.pem -TlsKey C:\\certs\\key.pem"
            echo ""
            exit 1
            ;;
    esac
}

main "$@"

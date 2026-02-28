#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Windows provisioning for cloudflared-fips fleet.

.DESCRIPTION
    Installs and configures cloudflared-fips components on Windows.
    Uses Go 1.24+ with GODEBUG=fips140=on (Go native FIPS 140-3) which
    delegates to Windows CNG (Cryptography API: Next Generation).

    Windows FIPS mode: Registry key at
    HKLM\SYSTEM\CurrentControlSet\Control\Lsa\FIPSAlgorithmPolicy\Enabled

.PARAMETER Role
    Node role: controller, server, proxy, client (default: client)

.PARAMETER Tier
    Deployment tier: 1, 2, or 3 (default: 1)

.PARAMETER EnrollmentToken
    Enrollment token from fleet controller

.PARAMETER ControllerUrl
    Fleet controller URL (required for non-controller roles)

.PARAMETER AdminKey
    Admin API key for controller role

.PARAMETER NodeName
    Display name for this node

.PARAMETER NodeRegion
    Region label (e.g., us-east)

.PARAMETER TlsCert
    TLS certificate path (tier 3 server/proxy)

.PARAMETER TlsKey
    TLS private key path (tier 3 server/proxy)

.PARAMETER EnableFips
    Enable Windows FIPS mode (sets registry key, requires reboot)

.PARAMETER SkipFips
    Skip FIPS mode check (dev/test only)

.PARAMETER CheckOnly
    Only check FIPS posture, don't install anything

.EXAMPLE
    .\provision-windows.ps1 -Role client -EnrollmentToken TOKEN -ControllerUrl http://controller:8080

.EXAMPLE
    .\provision-windows.ps1 -Role server -Tier 3 -TlsCert C:\certs\cert.pem -TlsKey C:\certs\key.pem

.EXAMPLE
    .\provision-windows.ps1 -CheckOnly
#>

param(
    [ValidateSet("controller", "server", "proxy", "client")]
    [string]$Role = "client",

    [ValidateSet("1", "2", "3")]
    [string]$Tier = "1",

    [string]$EnrollmentToken = "",
    [string]$ControllerUrl = "",
    [string]$AdminKey = "",
    [string]$NodeName = "",
    [string]$NodeRegion = "",
    [string]$TlsCert = "",
    [string]$TlsKey = "",
    [switch]$EnableFips,
    [switch]$SkipFips,
    [switch]$CheckOnly
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
$GoVersion = "1.24.0"
$RepoUrl = "https://github.com/JongoDB/cloudflared-fips.git"
$InstallDir = "C:\Program Files\cloudflared-fips"
$ConfigDir = "C:\ProgramData\cloudflared-fips"
$DataDir = "C:\ProgramData\cloudflared-fips\data"
$BinDir = "C:\Program Files\cloudflared-fips\bin"
$ServicePrefix = "cloudflared-fips"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function Write-Log { param([string]$Message) Write-Host "[+] $Message" -ForegroundColor Green }
function Write-Warn { param([string]$Message) Write-Host "[!] $Message" -ForegroundColor Yellow }
function Write-Fail { param([string]$Message) Write-Host "[x] $Message" -ForegroundColor Red; exit 1 }
function Write-Info { param([string]$Message) Write-Host "[i] $Message" -ForegroundColor Cyan }

function Get-WindowsFipsEnabled {
    try {
        $val = Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa\FIPSAlgorithmPolicy" -Name "Enabled" -ErrorAction SilentlyContinue
        return ($val.Enabled -eq 1)
    } catch {
        return $false
    }
}

function Get-Architecture {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { return "amd64" }
        "Arm64" { return "arm64" }
        default { Write-Fail "Unsupported architecture: $arch" }
    }
}

# ---------------------------------------------------------------------------
# FIPS posture check
# ---------------------------------------------------------------------------
function Check-FipsPosture {
    Write-Host ""
    Write-Host "============================================"
    Write-Host "  Windows FIPS Posture Check"
    Write-Host "============================================"
    Write-Host ""

    $arch = Get-Architecture
    $osVersion = [System.Environment]::OSVersion.VersionString
    $fipsEnabled = Get-WindowsFipsEnabled

    Write-Log "Windows version:      $osVersion"
    Write-Log "Architecture:         $arch"

    if ($fipsEnabled) {
        Write-Log "FIPS mode:            Enabled (CNG FIPS Algorithm Policy)"
    } else {
        Write-Warn "FIPS mode:            Disabled"
        Write-Info "  Enable via: Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Lsa\FIPSAlgorithmPolicy' -Name 'Enabled' -Value 1"
        Write-Info "  Or via GPO: Computer Configuration > Windows Settings > Security Settings >"
        Write-Info "              Local Policies > Security Options > 'System cryptography: Use FIPS compliant algorithms'"
    fi

    # Check CNG providers
    try {
        $cngProviders = Get-ChildItem "HKLM:\SOFTWARE\Microsoft\Cryptography\Defaults\Provider" -ErrorAction SilentlyContinue
        if ($cngProviders) {
            Write-Log "CNG providers:        $($cngProviders.Count) registered"
        }
    } catch {
        Write-Info "CNG providers:        Could not enumerate"
    }

    # Check for Go
    $goPath = Get-Command go -ErrorAction SilentlyContinue
    if ($goPath) {
        $goVer = & go version 2>$null
        if ($goVer -match "go1\.(2[4-9]|[3-9][0-9])") {
            Write-Log "Go version:           $goVer (FIPS 140-3 capable)"
        } else {
            Write-Warn "Go version:           $goVer (needs Go 1.24+ for native FIPS)"
        }
    } else {
        Write-Warn "Go:                   Not installed"
    }

    # Check if our service is running
    $agentService = Get-Service -Name "${ServicePrefix}-agent" -ErrorAction SilentlyContinue
    if ($agentService) {
        Write-Log "FIPS agent:           $($agentService.Status)"
    } else {
        Write-Info "FIPS agent:           Not installed"
    }

    # Check BitLocker
    try {
        $bitlocker = Get-BitLockerVolume -MountPoint "C:" -ErrorAction SilentlyContinue
        if ($bitlocker -and $bitlocker.ProtectionStatus -eq "On") {
            Write-Log "BitLocker (C:):       Enabled"
        } else {
            Write-Warn "BitLocker (C:):       Not enabled (recommended for FIPS compliance)"
        }
    } catch {
        Write-Info "BitLocker:            Could not check (requires elevated privileges)"
    }

    Write-Host ""
    Write-Info "Windows FIPS mode activates CNG (CMVP validated per Windows release)."
    Write-Info "Go native FIPS 140-3 delegates to CNG on Windows."
    Write-Host ""
}

if ($CheckOnly) {
    Check-FipsPosture
    exit 0
}

# ---------------------------------------------------------------------------
# Verify admin privileges
# ---------------------------------------------------------------------------
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Fail "This script requires Administrator privileges. Run from an elevated PowerShell."
}

Write-Host ""
Write-Host "============================================"
Write-Host "  cloudflared-fips Windows provisioning"
Write-Host "  Role: $Role  |  Tier: $Tier"
Write-Host "  $([System.Environment]::OSVersion.VersionString)"
Write-Host "============================================"
Write-Host ""

# ---------------------------------------------------------------------------
# Enable Windows FIPS mode (optional)
# ---------------------------------------------------------------------------
if ($EnableFips) {
    $fipsEnabled = Get-WindowsFipsEnabled
    if ($fipsEnabled) {
        Write-Log "Windows FIPS mode already enabled"
    } else {
        Write-Log "Enabling Windows FIPS Algorithm Policy..."
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa\FIPSAlgorithmPolicy" -Name "Enabled" -Value 1

        Write-Warn "============================================"
        Write-Warn "  FIPS mode enabled. A reboot is required."
        Write-Warn "  Re-run this script after reboot."
        Write-Warn "============================================"

        $restart = Read-Host "Restart now? (y/N)"
        if ($restart -eq "y" -or $restart -eq "Y") {
            Restart-Computer -Force
        }
        exit 0
    }
}

# Check FIPS mode if not skipped
if (-not $SkipFips) {
    $fipsEnabled = Get-WindowsFipsEnabled
    if ($fipsEnabled) {
        Write-Log "Windows FIPS mode: Enabled"
    } else {
        Write-Warn "Windows FIPS mode is NOT enabled."
        Write-Warn "Run with -EnableFips to enable, or -SkipFips for dev/test."
        Write-Warn "The Go binary will still use FIPS-capable crypto via GODEBUG=fips140=on."
    }
}

# ---------------------------------------------------------------------------
# Install Go
# ---------------------------------------------------------------------------
$arch = Get-Architecture

$goInstalled = Get-Command go -ErrorAction SilentlyContinue
$goNeedsInstall = $true
if ($goInstalled) {
    $goVer = & go version 2>$null
    if ($goVer -match "go$GoVersion") {
        Write-Log "Go $GoVersion already installed"
        $goNeedsInstall = $false
    }
}

if ($goNeedsInstall) {
    Write-Log "Installing Go $GoVersion ($arch)..."

    $goArch = if ($arch -eq "arm64") { "arm64" } else { "amd64" }
    $goMsi = "go${GoVersion}.windows-${goArch}.msi"
    $goUrl = "https://go.dev/dl/$goMsi"

    $tempMsi = Join-Path $env:TEMP $goMsi
    Invoke-WebRequest -Uri $goUrl -OutFile $tempMsi -UseBasicParsing

    Start-Process msiexec.exe -ArgumentList "/i", "`"$tempMsi`"", "/quiet", "/norestart" -Wait -NoNewWindow
    Remove-Item $tempMsi -Force -ErrorAction SilentlyContinue

    # Refresh PATH
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
}

Write-Log "Go version: $(& go version 2>$null)"

# ---------------------------------------------------------------------------
# Install Git (if needed)
# ---------------------------------------------------------------------------
$gitInstalled = Get-Command git -ErrorAction SilentlyContinue
if (-not $gitInstalled) {
    Write-Log "Installing Git..."

    # Try winget first
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if ($winget) {
        & winget install --id Git.Git -e --source winget --accept-package-agreements --accept-source-agreements
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    } else {
        Write-Fail "Git is not installed and winget is not available. Please install Git manually: https://git-scm.com/download/win"
    }
}

# ---------------------------------------------------------------------------
# Clone and build
# ---------------------------------------------------------------------------
if (Test-Path (Join-Path $InstallDir ".git")) {
    Write-Log "Updating existing repo in $InstallDir..."
    Push-Location $InstallDir
    & git pull --ff-only
} else {
    Write-Log "Cloning repo to $InstallDir..."
    New-Item -ItemType Directory -Path (Split-Path $InstallDir) -Force | Out-Null
    & git clone $RepoUrl $InstallDir
    Push-Location $InstallDir
}

# Build Go binaries with Go native FIPS
Write-Log "Building binaries with GODEBUG=fips140=on..."
$env:GODEBUG = "fips140=on"
$env:CGO_ENABLED = "0"

New-Item -ItemType Directory -Path "build-output" -Force | Out-Null

switch ($Role) {
    "controller" {
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-selftest.exe" .\cmd\selftest\
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-dashboard.exe" .\cmd\dashboard\
    }
    "server" {
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-selftest.exe" .\cmd\selftest\
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-dashboard.exe" .\cmd\dashboard\
        if ($Tier -eq "3") {
            & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-proxy.exe" .\cmd\fips-proxy\
        }
    }
    "proxy" {
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-selftest.exe" .\cmd\selftest\
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-proxy.exe" .\cmd\fips-proxy\
    }
    "client" {
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-selftest.exe" .\cmd\selftest\
        & go build -trimpath -ldflags="-s -w" -o "build-output\cloudflared-fips-agent.exe" .\cmd\agent\
    }
}

Write-Log "Running self-test..."
& ".\build-output\cloudflared-fips-selftest.exe" 2>$null
Write-Host ""

# ---------------------------------------------------------------------------
# Install binaries
# ---------------------------------------------------------------------------
Write-Log "Installing binaries..."
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
New-Item -ItemType Directory -Path $DataDir -Force | Out-Null

Copy-Item "build-output\cloudflared-fips-selftest.exe" $BinDir -Force

switch ($Role) {
    { $_ -in "controller", "server" } {
        Copy-Item "build-output\cloudflared-fips-dashboard.exe" $BinDir -Force
        if ($Tier -eq "3" -and (Test-Path "build-output\cloudflared-fips-proxy.exe")) {
            Copy-Item "build-output\cloudflared-fips-proxy.exe" $BinDir -Force
        }
    }
    "proxy" {
        Copy-Item "build-output\cloudflared-fips-proxy.exe" $BinDir -Force
    }
    "client" {
        Copy-Item "build-output\cloudflared-fips-agent.exe" $BinDir -Force
    }
}

# Config files
if (Test-Path "build-output\build-manifest.json") {
    Copy-Item "build-output\build-manifest.json" $ConfigDir -Force -ErrorAction SilentlyContinue
}
if (Test-Path "configs\cloudflared-fips.yaml") {
    Copy-Item "configs\cloudflared-fips.yaml" $ConfigDir -Force -ErrorAction SilentlyContinue
}

# TLS cert/key for Tier 3
if ($Tier -eq "3" -and $TlsCert -and $TlsKey) {
    Write-Log "Installing TLS certificate and key for Tier 3..."
    Copy-Item $TlsCert (Join-Path $ConfigDir "tls-cert.pem") -Force
    Copy-Item $TlsKey (Join-Path $ConfigDir "tls-key.pem") -Force
}

# Environment config
$envFile = Join-Path $ConfigDir "env.json"
$envConfig = @{
    DEPLOYMENT_TIER = $Tier
    GODEBUG = "fips140=on"
}

# ---------------------------------------------------------------------------
# Fleet enrollment
# ---------------------------------------------------------------------------
if ($Role -ne "controller" -and $EnrollmentToken -and $ControllerUrl) {
    Write-Log "Enrolling with fleet controller..."
    $hostname = $env:COMPUTERNAME
    $name = if ($NodeName) { $NodeName } else { $hostname }

    $enrollBody = @{
        token = $EnrollmentToken
        name = $name
        region = $NodeRegion
        os = "windows"
        arch = $arch
        version = "dev"
        fips_backend = "GoNative"
    } | ConvertTo-Json

    try {
        $enrollResp = Invoke-RestMethod -Uri "$ControllerUrl/api/v1/fleet/enroll" `
            -Method Post -ContentType "application/json" -Body $enrollBody
    } catch {
        Write-Fail "Fleet enrollment failed: $_. Check -ControllerUrl and -EnrollmentToken."
    }

    $enrollment = @{
        node_id = $enrollResp.node_id
        api_key = $enrollResp.api_key
        controller_url = $ControllerUrl
        role = $Role
    }
    $enrollment | ConvertTo-Json | Set-Content (Join-Path $ConfigDir "enrollment.json")

    $envConfig["NODE_ID"] = $enrollResp.node_id
    $envConfig["NODE_API_KEY"] = $enrollResp.api_key
    $envConfig["CONTROLLER_URL"] = $ControllerUrl

    Write-Log "Enrolled as node $($enrollResp.node_id)"
}

# Controller admin key
if ($Role -eq "controller") {
    if (-not $AdminKey) {
        $AdminKey = -join ((48..57) + (97..102) | Get-Random -Count 64 | ForEach-Object { [char]$_ })
        Write-Log "Generated admin API key: $AdminKey"
    }
    $envConfig["FLEET_ADMIN_KEY"] = $AdminKey
}

$envConfig | ConvertTo-Json | Set-Content $envFile

# ---------------------------------------------------------------------------
# Create Windows Services
# ---------------------------------------------------------------------------
Write-Log "Creating Windows services..."

function Install-FipsService {
    param(
        [string]$Name,
        [string]$DisplayName,
        [string]$BinaryPath
    )

    # Remove existing service if present
    $existing = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($existing) {
        if ($existing.Status -eq "Running") {
            Stop-Service $Name -Force
        }
        & sc.exe delete $Name 2>$null
        Start-Sleep -Seconds 2
    }

    & sc.exe create $Name binPath= "`"$BinaryPath`"" start= delayed-auto DisplayName= "`"$DisplayName`""
    & sc.exe description $Name "cloudflared-fips FIPS 140-3 compliant service"
    & sc.exe failure $Name reset= 86400 actions= restart/5000/restart/10000/restart/30000
}

switch ($Role) {
    "controller" {
        $dashArgs = "--addr 0.0.0.0:8080 --manifest `"$ConfigDir\build-manifest.json`" --config `"$ConfigDir\cloudflared-fips.yaml`" --fleet-mode --db-path `"$DataDir\fleet.db`""
        Install-FipsService -Name "${ServicePrefix}-dashboard" `
            -DisplayName "cloudflared-fips Dashboard (Controller)" `
            -BinaryPath "$BinDir\cloudflared-fips-dashboard.exe $dashArgs"
    }
    "server" {
        $dashArgs = "--addr 127.0.0.1:8080 --manifest `"$ConfigDir\build-manifest.json`" --config `"$ConfigDir\cloudflared-fips.yaml`""
        Install-FipsService -Name "${ServicePrefix}-dashboard" `
            -DisplayName "cloudflared-fips Dashboard" `
            -BinaryPath "$BinDir\cloudflared-fips-dashboard.exe $dashArgs"

        if ($Tier -eq "3") {
            $proxyArgs = "--listen :443 --upstream localhost:8080"
            if ($TlsCert) {
                $proxyArgs += " --tls-cert `"$ConfigDir\tls-cert.pem`" --tls-key `"$ConfigDir\tls-key.pem`""
            }
            Install-FipsService -Name "${ServicePrefix}-proxy" `
                -DisplayName "cloudflared-fips FIPS Proxy (Tier 3)" `
                -BinaryPath "$BinDir\cloudflared-fips-proxy.exe $proxyArgs"
        }
    }
    "proxy" {
        $proxyArgs = "--listen :443 --upstream localhost:8080"
        if ($TlsCert) {
            $proxyArgs += " --tls-cert `"$ConfigDir\tls-cert.pem`" --tls-key `"$ConfigDir\tls-key.pem`""
        }
        Install-FipsService -Name "${ServicePrefix}-proxy" `
            -DisplayName "cloudflared-fips FIPS Proxy" `
            -BinaryPath "$BinDir\cloudflared-fips-proxy.exe $proxyArgs"
    }
    "client" {
        $agentArgs = ""
        if (Test-Path (Join-Path $ConfigDir "enrollment.json")) {
            $agentArgs = "--config `"$ConfigDir\enrollment.json`""
        }
        Install-FipsService -Name "${ServicePrefix}-agent" `
            -DisplayName "cloudflared-fips FIPS Posture Agent" `
            -BinaryPath "$BinDir\cloudflared-fips-agent.exe $agentArgs"
    }
}

# ---------------------------------------------------------------------------
# Start services
# ---------------------------------------------------------------------------
Write-Log "Starting services..."

switch ($Role) {
    "controller" {
        Start-Service "${ServicePrefix}-dashboard" -ErrorAction SilentlyContinue
    }
    "server" {
        Start-Service "${ServicePrefix}-dashboard" -ErrorAction SilentlyContinue
        if ($Tier -eq "3") {
            Start-Service "${ServicePrefix}-proxy" -ErrorAction SilentlyContinue
        }
    }
    "proxy" {
        Start-Service "${ServicePrefix}-proxy" -ErrorAction SilentlyContinue
    }
    "client" {
        Start-Service "${ServicePrefix}-agent" -ErrorAction SilentlyContinue
    }
}

Start-Sleep -Seconds 2

# Add bin dir to system PATH
$machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
if ($machinePath -notlike "*$BinDir*") {
    [System.Environment]::SetEnvironmentVariable("Path", "$machinePath;$BinDir", "Machine")
    $env:Path += ";$BinDir"
    Write-Log "Added $BinDir to system PATH"
}

Pop-Location

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
Write-Host ""
Write-Log "============================================"
Write-Log "  cloudflared-fips deployed on Windows!"
Write-Log "  Role: $Role  |  Tier: $Tier"
Write-Log "============================================"
Write-Host ""

Write-Info "FIPS backend:     Go native FIPS 140-3 (GODEBUG=fips140=on)"
Write-Info "Windows CNG:      $(if (Get-WindowsFipsEnabled) { 'FIPS mode enabled' } else { 'FIPS mode NOT enabled' })"
Write-Info "CMVP note:        Go native FIPS CAVP A6650, CMVP pending"
Write-Host ""

switch ($Role) {
    "controller" {
        Write-Info "Dashboard:    http://0.0.0.0:8080"
        Write-Info "Fleet API:    http://0.0.0.0:8080/api/v1/fleet/"
        if ($AdminKey) {
            Write-Info "Admin key:    $AdminKey"
        }
        Write-Info "Logs:         Get-EventLog -LogName Application -Source '${ServicePrefix}-dashboard'"
    }
    "server" {
        Write-Info "Dashboard:    http://127.0.0.1:8080"
        if ($Tier -eq "3") {
            Write-Info "FIPS Proxy:   :443"
        }
        Write-Info "Logs:         Get-EventLog -LogName Application -Source '${ServicePrefix}-dashboard'"
    }
    "proxy" {
        Write-Info "FIPS Proxy:   :443"
        Write-Info "Logs:         Get-EventLog -LogName Application -Source '${ServicePrefix}-proxy'"
    }
    "client" {
        Write-Info "Agent:        running as Windows service"
        Write-Info "Check:        cloudflared-fips-agent.exe --check"
        Write-Info "Logs:         Get-EventLog -LogName Application -Source '${ServicePrefix}-agent'"
    }
}

Write-Host ""
Write-Info "Service management:"
Write-Info "  Stop:    Stop-Service ${ServicePrefix}-*"
Write-Info "  Start:   Start-Service ${ServicePrefix}-*"
Write-Info "  Status:  Get-Service ${ServicePrefix}-*"
Write-Info "  Remove:  sc.exe delete ${ServicePrefix}-agent"
Write-Host ""
Write-Info "Self-test: cloudflared-fips-selftest.exe"
Write-Host ""

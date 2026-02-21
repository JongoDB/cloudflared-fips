# cloudflared-fips

FIPS 140-2/3 compliant build of Cloudflare Tunnel (`cloudflared`) with end-to-end cryptographic observability, a compliance dashboard, and an AO authorization toolkit.

## Overview

Cloudflare Tunnel (`cloudflared`) uses standard Go crypto by default — it is **not** FIPS-validated. This project rebuilds it with validated cryptographic modules and wraps it in tooling that makes every link in the TLS chain visible and auditable.

Three core deliverables:

1. **FIPS-compliant cloudflared binary** using validated cryptographic modules (BoringCrypto on Linux, Go native FIPS 140-3 on macOS/Windows)
2. **Real-time compliance dashboard** — a 41-item checklist making every security property of the full connection chain transparently visible, with honest verification-method indicators (web GUI + terminal TUI)
3. **AO authorization documentation toolkit** — templates and auto-generated artifacts supporting an Authorizing Official authorization path

## Three-Segment Architecture

A Cloudflare Tunnel connection has three segments, each with different crypto properties and different levels of verifiability:

```
┌──────────┐       ┌─────────────────┐       ┌──────────────────┐       ┌──────────────┐
│  Client  │──TLS──│ Cloudflare Edge │──TLS──│ cloudflared-fips │──TLS──│ Local Service│
│ (Seg. 1) │       │    (Seg. 2a)    │       │    (Seg. 2b)     │       │   (Seg. 3)   │
└──────────┘       └─────────────────┘       └──────────────────┘       └──────────────┘
  Reported /          Inherited from            Direct — we                Probe — TLS
  Probe only          FedRAMP authz             control this               handshake
```

| Segment | What we control | What we verify | Verification method |
|---------|----------------|----------------|---------------------|
| **Client → Edge** | Nothing (client OS) | Cipher suites via ClientHello, device posture via WARP/MDM | Probe, Reported |
| **Cloudflare Edge** | Cipher config, TLS min version, Access policies | API settings, TLS probe, certificate chain | API, Inherited |
| **Edge → cloudflared** | Crypto module, cipher enforcement, binary integrity | BoringCrypto symbols, self-test KATs, OS FIPS mode | Direct |
| **cloudflared → Origin** | TLS to local service | Negotiated cipher, cert validity, reachability | Probe |

### The Honesty Problem

Cloudflare does **not** hold a CMVP certificate. Their edge uses BoringSSL (which contains a FIPS-validated sub-module), but the CMVP certificates belong to Google, not Cloudflare. There is no API to verify FIPS mode is active on the edge server handling your request.

This project is transparent about this gap. Every dashboard item shows a **verification method badge**:

| Badge | Meaning |
|-------|---------|
| **Direct** | Measured locally (self-test, binary hash, OS FIPS mode) |
| **API** | Confirmed via Cloudflare API (cipher config, Access policy) |
| **Probe** | Confirmed via TLS handshake (negotiated cipher, cert chain) |
| **Inherited** | Relies on Cloudflare's FedRAMP authorization — not independently verifiable |
| **Reported** | Client-reported via WARP/device posture — trust depends on endpoint management |

## Deployment Tiers

Three tiers of increasing FIPS assurance:

### Tier 1: Standard Cloudflare Tunnel

```
Client → Cloudflare Edge → cloudflared-fips → Origin
```

Default. Edge crypto inherited from Cloudflare's FedRAMP Moderate authorization. Gap: edge crypto module not independently FIPS-validated.

### Tier 2: Cloudflare's FIPS 140 Level 3 Architecture (Keyless SSL + HSM)

This is [Cloudflare's official reference architecture for FIPS 140 Level 3 compliance](https://developers.cloudflare.com/reference-architecture/diagrams/security/fips-140-3/).

```
Client → Cloudflare Edge (Regional Services, US FedRAMP DCs)
              ↓ key operation request
         cloudflared-fips tunnel (BoringCrypto — carries app traffic + key ops)
              ↓
         Keyless SSL Module (software proxy)
              ↓ PKCS#11
         Customer HSM (FIPS 140-2 Level 3)
```

**The tunnel is doubly critical in Tier 2:** it carries both application data AND Keyless SSL cryptographic key operations. Every TLS handshake at the Cloudflare edge triggers a key operation that flows through this tunnel to the customer's HSM. The private key never leaves the HSM — only the signed result returns.

Traffic restricted to US FedRAMP-compliant data centers via Regional Services. Supported HSMs: AWS CloudHSM, Azure Dedicated/Managed HSM, Entrust nShield Connect, Fortanix, Google Cloud HSM, IBM Cloud HSM. Remaining gap: bulk encryption (AES-GCM) still via Cloudflare's edge BoringSSL.

### Tier 3: Self-Hosted FIPS Proxy

```
Client → FIPS Proxy (GovCloud) → [optional: Cloudflare for WAF/DDoS] → cloudflared-fips → Origin
```

Full control. `cmd/fips-proxy` is a Go reverse proxy built with BoringCrypto that terminates client TLS in your GovCloud environment. Every TLS termination point uses a FIPS-validated module you control.

## Crypto Module Matrix

The crypto backend is modular (`pkg/fipsbackend/`). Users select their FIPS module based on deployment platform:

| | BoringCrypto (Google) | Go Native FIPS 140-3 | Microsoft SystemCrypto |
|---|---|---|---|
| **FIPS 140-2 certs** | #3678, #4407 | None | Windows CNG #4825 |
| **FIPS 140-3 certs** | **#4735** | CAVP A6650 (CMVP pending) | Varies by platform |
| **Linux** | Yes (`GOEXPERIMENT=boringcrypto`) | Yes (`GODEBUG=fips140=on`) | Yes (OpenSSL) |
| **macOS** | No | Yes | Yes (CommonCrypto) |
| **Windows** | No | Yes | Yes (CNG) |
| **Requires CGO** | Yes | No | Platform-dependent |

### FIPS 140-2 Sunset: September 21, 2026

All FIPS 140-2 certificates move to the CMVP Historical List on this date. The dashboard displays a countdown banner with migration urgency. Migration path: BoringCrypto 140-3 (#4735) or Go native FIPS 140-3 (once CMVP validates).

### Post-Quantum Cryptography (PQC) Readiness

Cloudflare's edge already uses post-quantum key exchange (ML-KEM/Kyber) for connections to origin servers. Our crypto stack has PQC support at multiple levels:

| Component | PQC Status | Details |
|-----------|-----------|---------|
| **BoringSSL** | ML-KEM (Kyber) supported | BoringCrypto includes post-quantum key exchange; used in Cloudflare's edge |
| **Go 1.24+** | `crypto/mlkem` package | Native ML-KEM support via `crypto/mlkem`; available with `GODEBUG=fips140=on` |
| **Cloudflare edge → origin** | Active | Cloudflare uses PQC for edge-to-origin connections when supported |
| **Client → edge** | Browser-dependent | Chrome/Firefox support ML-KEM hybrid key exchange (X25519Kyber768) |

PQC is not yet part of FIPS 140-3 validation (NIST is developing FIPS 203/204/205 for ML-KEM/ML-DSA/SLH-DSA). When FIPS PQC standards are finalized, the modular backend can add PQC-specific validation checks.

## Client-Side FIPS Detection

The product detects client FIPS capability through:

- **TLS ClientHello inspection** — absence of ChaCha20-Poly1305 indicates FIPS mode (FIPS clients only offer AES-GCM)
- **JA4 fingerprinting** — simplified TLS client fingerprint for FIPS-mode pattern matching
- **Device posture API** — agents report OS FIPS mode, MDM enrollment, disk encryption via `POST /api/v1/posture`
- **Cloudflare Access integration** — device posture checks (OS version, MDM, disk encryption)

## Directory Structure

```
├── build/
│   ├── Dockerfile.fips              # RHEL UBI 9 FIPS build (BoringCrypto)
│   ├── Dockerfile.fips-proxy        # Tier 3 FIPS proxy container
│   ├── build.sh                     # Build orchestrator
│   └── packaging/                   # RPM, DEB, macOS .pkg, Windows MSI (WiX)
├── cmd/
│   ├── selftest/                    # Standalone self-test CLI
│   ├── dashboard/                   # Compliance dashboard server (localhost-only)
│   ├── fips-proxy/                  # Tier 3 FIPS reverse proxy
│   └── tui/                         # TUI: setup wizard + status monitor
├── internal/
│   ├── selftest/                    # KATs, cipher validation, BoringCrypto detection
│   ├── compliance/                  # Live compliance checker (system + config + binary)
│   ├── dashboard/                   # HTTP API handlers + SSE + WebSocket
│   └── tui/                         # Terminal UI (Bubbletea)
│       ├── config/                  # Config struct, YAML write/read, validators
│       ├── common/                  # Shared components: textinput, toggle, selector, ingress
│       ├── wizard/                  # 5-page setup wizard
│       └── status/                  # Live compliance status monitor
├── pkg/
│   ├── buildinfo/                   # Linker-injected build metadata
│   ├── manifest/                    # Build manifest types + read/write
│   ├── fipsbackend/                 # Modular crypto backend + 140-3 migration tracking
│   ├── cfapi/                       # Cloudflare API client (zone, Access, tunnel health)
│   ├── clientdetect/                # TLS ClientHello inspector, JA4, device posture
│   ├── deployment/                  # Deployment tier config (standard/regional/self-hosted)
│   └── signing/                     # Artifact signing (GPG + cosign) and verification
├── dashboard/                       # React + TypeScript + Tailwind (Vite)
├── configs/
│   ├── cloudflared-fips.yaml        # Sample tunnel + FIPS config with deployment_tier
│   └── build-manifest.json          # Sample manifest with FIPS certificate references
├── scripts/
│   ├── check-fips.sh                # Post-build FIPS validation
│   ├── verify-boring.sh             # Verify BoringCrypto symbols in binary
│   ├── generate-manifest.sh         # Produce build-manifest.json
│   ├── generate-sbom.sh             # CycloneDX + SPDX SBOM generation
│   ├── generate-docs.sh             # AO doc package (PDF/HTML via pandoc)
│   ├── audit-crypto-deps.sh         # Full dependency tree crypto audit
│   └── sign-artifacts.sh            # GPG signing + signature manifest
├── deploy/
│   ├── terraform/                   # AWS GovCloud Terraform (ECS Fargate)
│   └── cloudformation/              # AWS CloudFormation equivalent
├── docs/                            # AO package: SSP, crypto usage, control mapping,
│                                    # hardening guide, monitoring plan, IR addendum
└── .github/workflows/
    ├── fips-build.yml               # Cross-platform build + package + sign
    └── compliance-check.yml         # PR validation (lint, test, dashboard build)
```

## Prerequisites

- **Go 1.24+** with CGO support (Linux builds)
- **Docker** (for FIPS container builds)
- **Node.js 22+** (for dashboard development)
- **RHEL UBI 9** base image (pulled automatically by Docker)

## Quick Start

### Configure with the setup wizard

```bash
go build -o cloudflared-fips-tui ./cmd/tui
./cloudflared-fips-tui setup
```

Walks through 5 pages: tunnel config, dashboard wiring, deployment tier, FIPS options, review — then writes `configs/cloudflared-fips.yaml`.

### Run the compliance dashboard (web)

```bash
cd dashboard
npm install
npm run dev
```

### Monitor compliance from the terminal

```bash
./cloudflared-fips-tui status --api localhost:8080 --interval 5s
```

Polls the dashboard API and renders all 41 checklist items with pass/warn/fail in a scrollable terminal view. Useful for headless/SSH environments.

### Build the FIPS binary

```bash
make build-fips
```

### Build the TUI

```bash
make tui
```

### Run self-tests

```bash
make selftest
```

### Generate build manifest

```bash
make manifest
```

### Build Docker image

```bash
make docker-build
```

### Generate SBOMs

```bash
make sbom
```

### Run crypto dependency audit

```bash
make crypto-audit
```

### Generate AO documentation package

```bash
make docs
```

## Dashboard

The compliance dashboard displays 41 checklist items across five sections:

- **Client Posture** (8 items) — OS FIPS mode, TLS capabilities, device posture, MDM
- **Cloudflare Edge** (11 items) — Access policy, cipher restriction, TLS version, HSTS, certificates, Keyless SSL, Regional Services
- **Tunnel** (12 items) — BoringCrypto active, self-test KATs, cipher suites, binary integrity, tunnel health
- **Local Service** (4 items) — TLS enabled, cipher negotiation, cert validity, reachability
- **Build & Supply Chain** (6 items) — SBOM, manifest, reproducibility, signatures, FIPS certs

### Screenshots

#### Dashboard Overview
FIPS 140-2 sunset countdown banner, Tier 1 deployment badge, FIPS Cryptographic Module card (showing active backend, CMVP cert, 140-2/3 badge), 41 compliance checks with pass/warn/fail summary, verification method badges, and live SSE toggle.

![Dashboard Overview](docs/screenshots/dashboard-overview.png)

#### Full Dashboard
All five sections with status, severity, and verification method per item. FIPS backend card and build manifest expandable above the checklist.

![Full Dashboard](docs/screenshots/dashboard-full.png)

#### Expanded Checklist Item
Each item expands to show What, Why, Remediation steps, and NIST SP 800-53 control references.

![Expanded Checklist Item](docs/screenshots/checklist-item-expanded.png)

#### Build Manifest Panel
Build metadata, upstream cloudflared version, FIPS certificate details, and integrity hashes.

![Build Manifest](docs/screenshots/build-manifest-expanded.png)

#### Summary Bar
Sunset banner with progress bar, deployment tier badge, compliance summary (80%), and export controls.

![Summary Bar](docs/screenshots/summary-bar.png)

### Dashboard API

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/compliance` | Full compliance state (all sections) |
| `GET /api/v1/manifest` | Build manifest |
| `GET /api/v1/selftest` | On-demand FIPS self-test |
| `GET /api/v1/backend` | Active FIPS crypto backend info |
| `GET /api/v1/events` | SSE stream (real-time updates) |
| `GET /api/v1/ws` | WebSocket (real-time updates with SSE fallback) |
| `GET /api/v1/clients` | TLS ClientHello inspection results |
| `GET /api/v1/posture` | Device posture reports |
| `POST /api/v1/posture` | Submit device posture report |
| `GET /api/v1/deployment` | Deployment tier info |
| `GET /api/v1/migration` | FIPS 140-2 → 140-3 migration status |
| `GET /api/v1/migration/backends` | All backend migration details |
| `GET /api/v1/signatures` | Artifact signature manifest |
| `GET /api/v1/compliance/export` | JSON export of full compliance state |
| `GET /api/v1/mdm/devices` | MDM-enrolled device compliance list |
| `GET /api/v1/mdm/summary` | MDM fleet compliance summary |
| `GET /health` | Health check |

## Terminal UI (TUI)

A lightweight alternative to the web dashboard for headless and SSH environments, built with [Bubbletea](https://github.com/charmbracelet/bubbletea).

### Setup Wizard

```bash
./cloudflared-fips-tui setup
```

Interactive 5-page wizard:

| Page | Fields |
|------|--------|
| **1. Tunnel** | UUID, credentials file, protocol (QUIC/HTTP2), ingress rules (add/remove) |
| **2. Dashboard Wiring** | CF API token, zone/account/tunnel IDs, metrics address, MDM provider (None/Intune/Jamf) |
| **3. Deployment Tier** | Tier 1 (standard), Tier 2 (Keyless SSL + HSM), or Tier 3 (self-hosted proxy) with conditional fields |
| **4. FIPS Options** | Self-test on start, fail-on-failure, binary signature verification, output path |
| **5. Review & Write** | Scrollable summary (secrets masked), Enter writes `configs/cloudflared-fips.yaml` |

Navigation: `Tab`/`Enter` = next, `Shift+Tab` = back, `Ctrl+C` = quit.

### Status Monitor

```bash
./cloudflared-fips-tui status [--api localhost:8080] [--interval 5s]
```

Polls `GET /api/v1/compliance` and renders all 41 items grouped by section:

```
 cloudflared-fips v0.1.0 | Compliance Status | Updated 12:34:56

 ┌──────────────────────────────────────────────────────────┐
 │  38/41 PASS   2 WARN   1 FAIL              93% ██████░  │
 └──────────────────────────────────────────────────────────┘

 CLIENT POSTURE                                    7/8 pass
   ● Client OS FIPS Mode                              PASS
   ○ Browser TLS Capabilities                         WARN
   ...

 TUNNEL — CLOUDFLARED                            11/12 pass
   ● BoringCrypto Active                              PASS
   ✖ Binary Integrity                                 FAIL
   ...

 [q] Quit  [r] Refresh | Polling every 5s | Connected to localhost:8080
```

Keys: `q` = quit, `r` = force refresh, arrows/PgUp/PgDn = scroll.

## FIPS Build Pipeline

### Linux (primary)

Uses `GOEXPERIMENT=boringcrypto` on RHEL UBI 9. BoringCrypto's FIPS-validated `.syso` object files are statically linked into the binary — the validated crypto travels with the binary and does **not** depend on host OS OpenSSL.

1. Build on RHEL UBI 9 with `GOEXPERIMENT=boringcrypto` + `CGO_ENABLED=1`
2. Verify `_goboringcrypto_` symbols are present
3. Run Known Answer Tests against NIST CAVP vectors (AES-GCM, SHA-256/384, HMAC, ECDSA, RSA)
4. Validate only FIPS-approved cipher suites are available
5. Generate build manifest with cryptographic provenance
6. Package as RPM, DEB, and OCI container
7. Sign artifacts with GPG; sign containers with cosign (Sigstore)

### macOS / Windows

Uses `GODEBUG=fips140=on` (Go 1.24+ native FIPS 140-3 module, CAVP cert A6650, CMVP pending). No CGO required. Packaged as macOS `.pkg` and Windows `.msi`.

### Supported Host OSes

The binary uses FIPS-validated crypto on any Linux (amd64/arm64). For full-stack FIPS, the host OS should also run in FIPS mode:

| Distro | FIPS 140-3 | FIPS Mode |
|--------|-----------|-----------|
| RHEL 9 | Validated | `fips-mode-setup --enable` |
| Ubuntu Pro 22.04 | Validated | `ua enable fips` |
| Amazon Linux 2023 | Validated | `fips-mode-setup --enable` |
| SUSE SLES 15 SP6+ | Validated | `fips=1` boot param |
| AlmaLinux 9.2+ | Validated | `fips-mode-setup --enable` |

## AO Documentation Package

Templates for supporting an Authorizing Official authorization path:

- **System Security Plan (SSP)** — module boundaries, validated modules with CMVP certs, crypto operations mapped to modules
- **Cryptographic Module Usage Document** — operation-to-algorithm-to-module-to-certificate mapping
- **FIPS Compliance Justification Letter** — structured AO argument for leveraged validated module approach
- **Client Endpoint Hardening Guide** — Windows GPO, RHEL, macOS, MDM policy templates
- **Continuous Monitoring Plan** — dashboard as monitoring tool, re-verification on updates
- **Incident Response Addendum** — crypto failure procedures
- **NIST 800-53 Control Mapping** — SC-8, SC-13, SC-12, IA-7, SA-11, CM-14

## Artifact Signing

- **Binaries and packages:** GPG detached signatures (`.sig` files)
- **Container images:** cosign (Sigstore) keyless signing in CI
- **Signature manifest:** `signatures.json` with artifact hashes and signer identity
- CI `sign-artifacts` job runs automatically on tagged releases

## License

See LICENSE file for details.

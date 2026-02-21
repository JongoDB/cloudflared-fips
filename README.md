# cloudflared-fips

FIPS 140-2 compliant build of Cloudflare Tunnel (`cloudflared`) with integrated compliance tooling, self-test validation, and a React-based compliance dashboard.

## Overview

This project provides:

- **FIPS-validated build pipeline** using `GOEXPERIMENT=boringcrypto` (BoringCrypto cert #4407) on RHEL UBI 9, with Go native FIPS (`GODEBUG=fips140=on`) for macOS/Windows
- **Modular crypto backend** supporting BoringCrypto, Go native FIPS 140-3, and Microsoft SystemCrypto with auto-detection
- **Runtime self-test suite** verifying BoringCrypto linkage, OS FIPS mode, cipher suites, and Known Answer Tests (NIST CAVP vectors)
- **Compliance dashboard** (React + TypeScript + Tailwind) with 39 checks across all three network segments, FIPS 140-2 sunset countdown, deployment tier display, verification method badges, and real-time SSE updates
- **Live compliance checks** querying local system state, Cloudflare API (edge settings, tunnel health), and TLS ClientHello inspection for client FIPS detection
- **Three deployment tiers** from standard Cloudflare Tunnel (Tier 1) to self-hosted FIPS proxy in GovCloud (Tier 3)
- **Build manifest and SBOM generation** with full cryptographic provenance, CycloneDX/SPDX output, and artifact signing (GPG + cosign)
- **CI/CD pipelines** for cross-platform builds (Linux/macOS/Windows, amd64/arm64) with real packaging (RPM, DEB, MSI, .pkg, OCI)
- **AO documentation toolkit** with SSP template, crypto module usage doc, NIST 800-53 control mapping, and client hardening guides

## Directory Structure

```
├── build/           # Dockerfile, build scripts, patches
├── cmd/             # Go entry points (selftest CLI, dashboard server)
├── internal/        # Private Go packages (selftest, compliance, dashboard)
├── pkg/             # Public Go packages (buildinfo, manifest)
├── dashboard/       # React + TypeScript + Tailwind compliance dashboard
├── configs/         # Sample configuration and manifest files
├── scripts/         # Helper scripts (FIPS checks, manifest generation)
├── docs/            # Authorization narrative, control mapping, architecture
└── .github/         # CI/CD workflow definitions
```

## Prerequisites

- **Go 1.24+** with CGO support
- **Docker** (for FIPS container builds)
- **Node.js 22+** (for dashboard development)
- **RHEL UBI 9** base image (pulled automatically by Docker)

## Quick Start

### Run the compliance dashboard (development)

```bash
cd dashboard
npm install
npm run dev
```

### Build the FIPS binary

```bash
make build-fips
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

## Dashboard Screenshots

### Dashboard Overview
FIPS 140-2 sunset countdown banner (212 days), deployment tier badge (Tier 1 — Standard Tunnel), 39 compliance checks with pass/warn/fail summary, verification method badges (Direct/API/Probe/Inherited/Reported), and live SSE toggle.

![Dashboard Overview](docs/screenshots/dashboard-overview.png)

### Full Dashboard
All five compliance checklist sections: Client Posture, Cloudflare Edge, Tunnel, Local Service, and Build & Supply Chain. Each item shows status, severity, and verification method.

![Full Dashboard](docs/screenshots/dashboard-full.png)

### Expanded Checklist Item
Each checklist item expands to show What, Why, Remediation steps, and NIST SP 800-53 control references.

![Expanded Checklist Item](docs/screenshots/checklist-item-expanded.png)

### Build Manifest Panel
Build metadata, upstream cloudflared version, FIPS certificate details (BoringSSL #3678, RHEL OpenSSL #4349), and integrity hashes.

![Build Manifest](docs/screenshots/build-manifest-expanded.png)

### Summary Bar Close-up
Sunset banner with progress bar, Tier 1 deployment badge, compliance summary (85%), and export controls.

![Summary Bar](docs/screenshots/summary-bar.png)

## FIPS Compliance

This project uses `GOEXPERIMENT=boringcrypto` to link Go's BoringCrypto module, which has completed FIPS 140-2 validation (certificate #4407). The build pipeline:

1. Builds on RHEL UBI 9 with FIPS-validated system OpenSSL
2. Verifies `_goboringcrypto_` symbols are present in the binary
3. Runs Known Answer Tests against NIST CAVP vectors
4. Validates only FIPS-approved cipher suites are available
5. Generates a build manifest with full cryptographic provenance

## Three-Segment Architecture

| Segment | Description | Crypto Module |
|---------|-------------|---------------|
| **Client → Cloudflare Edge** | Device posture, WARP client, TLS 1.2/1.3 | Client-side FIPS module |
| **Cloudflare Edge** | Access policies, Gateway inspection, key management | Cloudflare edge FIPS |
| **Tunnel (Edge → Origin)** | `cloudflared` tunnel, cipher enforcement | BoringCrypto (this build) |

## License

See LICENSE file for details.

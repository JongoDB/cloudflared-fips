# Changelog

All notable changes to the cloudflared-fips project are documented here.

## [0.1.0] — 2026-02-21

Initial public release. FIPS-compliant cloudflared build system, real-time compliance dashboard, and AO authorization documentation toolkit.

### Build System

- FIPS build on RHEL UBI 9 with `GOEXPERIMENT=boringcrypto` and BoringCrypto verification + self-test gates (`build/Dockerfile.fips`)
- Build orchestrator script (`build/build.sh`) clones upstream cloudflared source and applies FIPS build flags
- Reproducible builds: `SOURCE_DATE_EPOCH`, `-trimpath`, `-s -w -buildid=` across Dockerfile, build.sh, Makefile, and CI
- Modular crypto backend (`pkg/fipsbackend/`) with `Backend` interface supporting BoringCrypto, Go native FIPS 140-3, and SystemCrypto
- Auto-detection of active FIPS backend via `Detect()` with JSON-serializable `Info` struct
- FIPS edge proxy for Tier 3 self-hosted deployments (`cmd/fips-proxy/main.go`, `build/Dockerfile.fips-proxy`)
- Build manifest schema and generation (`pkg/manifest/`, `scripts/generate-manifest.sh`, `configs/build-manifest.json`)

### Dashboard

- React + TypeScript + Tailwind CSS frontend with 41 compliance checklist items across 5 sections
- Client Posture (8 items), Cloudflare Edge (11 items), Tunnel/cloudflared (12 items), Local Service (4 items), Build & Supply Chain (6 items)
- Verification badges on every item: Direct, API, Probe, Inherited, Reported — with tooltip explanations
- FIPS Cryptographic Module card showing active backend, CMVP certificate, and 140-2/140-3 badge
- Sunset banner with countdown to FIPS 140-2 end-of-life (September 21, 2026) and urgency colors
- Deployment tier badge (Standard / Regional Keyless / Self-Hosted)
- SSE real-time updates with `useComplianceSSE` hook, auto-reconnect, Live toggle, and connection status indicator
- WebSocket alternative (`useComplianceWS`) with automatic SSE fallback
- JSON export via ExportButtons component
- Build Manifest expandable panel with build metadata, FIPS certificates, and integrity hashes
- Air-gap friendly: all assets bundled, no CDN dependencies at runtime
- Dashboard binds to `127.0.0.1:8080` (localhost-only by default)

### Self-Test Suite

- Real NIST CAVP test vectors for AES-GCM, SHA-256, SHA-384, HMAC-SHA-256, ECDSA P-256, RSA-2048
- Backend-specific dispatch: BoringCrypto and Go native FIPS KAT runners
- QUIC cipher safety check (blocks non-FIPS ChaCha20-Poly1305)
- OS FIPS mode detection (`/proc/sys/crypto/fips_enabled` on Linux)
- Binary integrity verification (SHA-256 vs manifest)
- Optional GPG signature verification at startup (`--verify-signature` flag)
- BoringCrypto 140-2 vs 140-3 version detection based on Go version and `.syso` hashes
- Standalone CLI (`cmd/selftest/main.go`) and library (`internal/selftest/`)

### Live Compliance Checks

- Local system checks (`internal/compliance/live.go`): 22 checks across tunnel, local service, and build sections
- Active TLS probing for local services (TLS enabled, cipher suite, certificate validity)
- Cloudflare API integration (`pkg/cfapi/`): 11 checks including Access policy, cipher restriction, TLS minimum, HSTS, edge certificate, tunnel health, Keyless SSL, Regional Services
- Rate-limited, caching API client with bearer token auth and configurable TTL
- Client FIPS detection (`pkg/clientdetect/`): TLS ClientHello inspection, JA4 fingerprinting, device posture API
- Known FIPS fingerprints for Windows, RHEL, macOS, Go, and WARP clients
- MDM integration stubs for Intune (Graph API) and Jamf Pro

### CI/CD

- Compliance pipeline (`compliance-check.yml`): Go lint/test, dashboard lint/build, docs check, manifest validation, shell syntax
- Build matrix (`fips-build.yml`): per-OS native runners
  - Linux (RHEL UBI 9 + BoringCrypto, amd64/arm64)
  - macOS (Go native FIPS via `GODEBUG=fips140=on`, amd64/arm64)
  - Windows (Go native FIPS via `GODEBUG=fips140=on`, amd64/arm64)
- `if-no-files-found: error` on all artifact uploads — builds fail loudly on missing artifacts
- Artifact signing job (`sign-artifacts`) runs on tags with cosign + GPG
- SBOM generation integrated into CI pipeline

### Packaging

- RPM: `.spec` file with systemd unit, rpmbuild in CI (`build/packaging/rpm/`)
- DEB: control file, postinst, prerm scripts, dpkg-deb build (`build/packaging/deb/`)
- macOS: `.pkg` via pkgbuild + productbuild with optional codesigning (`build/packaging/macos/`)
- Windows: MSI via WiX v4 (`build/packaging/windows/cloudflared-fips.wxs`)
- OCI container: `podman build -f build/Dockerfile.fips`
- All packages include: binary, self-test, sample config, build manifest
- All packages run self-test as post-install verification

### SBOM & Supply Chain

- `scripts/generate-sbom.sh` produces CycloneDX 1.5 + SPDX 2.3 from `cyclonedx-gomod` or `go list -m -json all` fallback
- Crypto audit (`crypto-audit.json`) classifying all `crypto/*` and `golang.org/x/crypto` imports with BoringCrypto bypass detection
- `scripts/audit-crypto-deps.sh` maps every dependency to FIPS routing status (routed/partial/bypass)
- Artifact signing via `pkg/signing/` (GPG + cosign) and `scripts/sign-artifacts.sh`
- Signature verification script (`scripts/verify-signatures.sh`) for user-facing verification

### Documentation (AO Package)

- System Security Plan template (`docs/ao-narrative.md`) with FIPS 140-3 references
- Cryptographic module usage document (`docs/crypto-module-usage.md`) with quic-go audit findings
- FIPS compliance justification letter (`docs/fips-justification-letter.md`)
- Client endpoint hardening guide (`docs/client-hardening-guide.md`) for Windows, RHEL, macOS, and MDM
- Continuous monitoring plan (`docs/continuous-monitoring-plan.md`)
- Incident response addendum (`docs/incident-response-addendum.md`)
- NIST 800-53 control mapping (`docs/control-mapping.md`)
- Architecture diagram (`docs/architecture-diagram.md`) with Mermaid diagrams
- Deployment tier guide (`docs/deployment-tier-guide.md`) with HSM setup for 8 vendors
- Key rotation procedure (`docs/key-rotation-procedure.md`)
- quic-go cryptographic audit (`docs/quic-go-crypto-audit.md`)

### Infrastructure

- IPC interface via Unix domain socket (`internal/ipc/ipc.go`) with JSON-RPC protocol for CloudSH integration
- FIPS 140-3 migration tracking (`pkg/fipsbackend/migration.go`) with urgency levels, countdown, and recommended actions
- Deployment tier support (`pkg/deployment/tier.go`): Standard, Regional Keyless (FIPS 140 L3), Self-Hosted
- Terraform template for AWS GovCloud ECS Fargate deployment (`deploy/terraform/main.tf`)
- CloudFormation template (`deploy/cloudformation/cloudflared-fips.yaml`)
- 13 HTTP API endpoints under `/api/v1/` for compliance, manifest, self-test, health, backend, migration, posture, clients, signatures, and MDM
- PDF/HTML document generation via pandoc (`scripts/generate-docs.sh`)
- GPG public key infrastructure (`configs/public-key.asc`, key rotation docs)

[0.1.0]: https://github.com/cloudflared-fips/cloudflared-fips/releases/tag/v0.1.0

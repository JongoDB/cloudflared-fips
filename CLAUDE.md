# cloudflared-fips — Product Spec & Implementation Roadmap

> This file is the canonical reference for the cloudflared-fips product.
> It captures the full original spec and tracks implementation progress.
> Update checkboxes as work is completed.

---

## Product Vision

Three core deliverables:

1. **FIPS-compliant cloudflared build** using validated cryptographic modules (BoringCrypto + RHEL validated OpenSSL) — targeting the server/tunnel side
2. **Real-time compliance dashboard** — an "all green" checklist making every security property of the full connection chain transparently visible
3. **AO authorization documentation toolkit** — templates and auto-generated artifacts supporting an Authorizing Official authorization path without full CMVP validation

**Repo:** `cloudflared-fips` (private GitHub, later incorporated into CloudSH)
**Target market:** U.S. government, DoD, defense contractors needing FIPS-compliant access to globally distributed infrastructure

---

## FIPS Compliance Architecture — Three Segments

### Segment 1: Client OS/Browser → Cloudflare Edge (HTTPS)

NOT controlled by cloudflared. Standard HTTPS between client TLS stack and Cloudflare edge. FIPS enforced at client OS level:

- **Windows:** GPO "System cryptography: Use FIPS compliant algorithms" → Windows CNG (cert #2357+). Firefox requires separate NSS FIPS config.
- **RHEL/Linux:** `fips=1` kernel boot parameter → validated OpenSSL. **Known gap:** Chrome on Linux uses BoringSSL (not OS library).
- **macOS:** CommonCrypto/Secure Transport (cert #3856+) always active — no switch required.

**This product's role on Segment 1:**
- Dashboard detects/reports client FIPS posture
- Cloudflare Access device posture integration with MDM (Intune, Jamf) enforces client FIPS compliance as precondition for tunnel access
- Documents client-side FIPS requirements and MDM policy templates

### Segment 2: Cloudflare Edge → cloudflared (Tunnel)

**This is the segment this product directly controls.** cloudflared uses quic-go (QUIC over UDP 7844, falling back to HTTP/2 over TLS on TCP 443). This product replaces Go's standard crypto with BoringCrypto via `GOEXPERIMENT=boringcrypto`.

### Segment 3: cloudflared → Local Service (Internal)

Within deployment boundary. Loopback may be out of scope for FIPS (AO interpretation). Networked segments must use TLS with FIPS-approved ciphers. Document both scenarios.

---

## Phase 1 — FIPS-Compliant cloudflared Build

### Build Goals

- [x] Clone and build cloudflared from source (https://github.com/cloudflare/cloudflared)
- [x] Replace standard Go crypto with BoringCrypto via `GOEXPERIMENT=boringcrypto`
- [x] Write verification that only FIPS-approved algorithms are in use
- [x] Target production deployment on FIPS-mode RHEL 8/9 (`fips=1` kernel parameter, OpenSSL certs #3842, #4349)
- [ ] Audit full dependency tree for bundled crypto bypassing the validated module
- [ ] Specifically audit quic-go for crypto compliance — verify TLS operations route through BoringCrypto
- [x] Produce reproducible build scripts and Dockerfiles
- [x] Tag every build with metadata: validated modules, algorithm list, build timestamp, git commit hash, target platform

### Startup Self-Test

- [x] Confirms BoringCrypto is the active crypto provider (not standard Go crypto)
- [x] Confirms OS FIPS mode is enabled (`/proc/sys/crypto/fips_enabled == 1` on Linux; document macOS and Windows equivalents)
- [x] Confirms only FIPS-approved cipher suites are configured
- [x] Runs BoringCrypto's built-in known-answer tests (KATs)
- [x] Refuses to start if any check fails with clear, actionable error messages
- [x] Logs all results to structured JSON for compliance audit trail

### SBOM Generation

- [ ] Auto-generate SBOM at build time listing every dependency and version
- [ ] Flag whether each dependency performs cryptographic operations
- [ ] For each crypto dependency: which validated module it delegates to
- [ ] Output as CycloneDX and SPDX format (both accepted by government procurement)

---

## Phase 2 — Compliance Dashboard

### Vision

Web-based GUI showing real-time compliance checklist. Every item is green/yellow/red. Each item expandable with: what is checked, why it matters, what to do if red, applicable NIST/FIPS reference.

### Checklist Sections & Items

#### CLIENT POSTURE (Segment 1) — 8 items

- [x] Client OS FIPS mode: enabled/disabled/unknown
- [x] Client OS type and version
- [x] Browser TLS capabilities: FIPS-approved ciphers available yes/no
- [x] Negotiated cipher suite (client→edge): [show actual] — FIPS approved: yes/no
- [x] TLS version (client→edge): [show]
- [x] Cloudflare Access device posture check: passed/failed/not configured
- [x] MDM enrollment verified: yes/no/unknown
- [x] Client certificate (if mTLS): valid/expired/none

#### CLOUDFLARE EDGE — 9 items

- [x] Cloudflare Access policy: active/inactive
- [x] Identity provider connected: [IdP name]
- [x] Authentication method: [SAML/OIDC/etc.]
- [x] MFA enforced: yes/no
- [x] Cipher suite restriction: FIPS-140-2 profile active yes/no
- [x] Minimum TLS version enforced: [show]
- [x] Edge certificate valid: yes/no, expiry [days]
- [x] HSTS enforced: yes/no
- [x] Last auth event: [timestamp]

#### TUNNEL — CLOUDFLARED (Segment 2) — 12 items

- [x] BoringCrypto active: yes/no (validated module: cert #XXXX)
- [x] OS FIPS mode (server): enabled/disabled
- [x] FIPS self-test: passed/failed
- [x] Tunnel protocol: QUIC/HTTP2
- [x] TLS version (edge→tunnel): [show]
- [x] Cipher suite (edge→tunnel): [show] — FIPS approved: yes/no
- [x] Tunnel authenticated: yes/no
- [x] Tunnel redundancy: [N active connections to Cloudflare PoPs]
- [x] Tunnel uptime: [duration]
- [x] Binary integrity: [hash match yes/no vs known-good]
- [x] cloudflared version: [show]
- [x] Last heartbeat: [timestamp]

#### LOCAL SERVICE (Segment 3) — 4 items

- [x] Service connection type: loopback / networked
- [x] If networked: TLS enabled yes/no
- [x] If TLS: cipher suite FIPS approved yes/no
- [x] Service reachable: yes/no

#### BUILD AND SUPPLY CHAIN — 6 items

- [x] SBOM loaded and verified
- [x] Build reproducibility: [hash of binary vs known-good]
- [x] BoringCrypto version: [show] — FIPS cert: [#]
- [x] RHEL OpenSSL FIPS module: [cert #]
- [x] Configuration drift: none / [changes listed]
- [x] Last compliance scan: [timestamp]

> **Status:** All 39 checklist items scaffolded with mock data. Items marked [x] have mock data in dashboard; real data wiring is Phase 2 implementation work.

### Dashboard Implementation

- [x] React + TypeScript frontend, Tailwind CSS
- [x] Mock data covering all three segments (validates UX before wiring real sources)
- [x] Export compliance report as JSON
- [x] PDF export stub (requires pandoc backend)
- [x] Dashboard localhost-only by default
- [x] Air-gap friendly: all assets bundled, no CDN dependencies at runtime
- [ ] Backend: Go sidecar service querying local system state, cloudflared /metrics, Cloudflare API
- [ ] Active TLS probing (connect to tunnel hostname, inspect negotiated cipher/TLS version)
- [ ] Cloudflare API integration (Access policy status, tunnel health, cipher config)
- [ ] MDM API integration (Intune/Jamf) for client posture data
- [ ] Real-time updates via SSE (endpoint scaffolded, needs real data sources)
- [ ] WebSocket alternative for real-time updates

---

## Phase 3 — AO Authorization Documentation Package

### Documents

1. [x] **System Security Plan (SSP) template** — `docs/ao-narrative.md`
   - [x] Module boundary definition
   - [x] Validated modules with NIST certificate numbers
   - [x] Cryptographic operations mapped to validated modules
   - [x] References to NIST SP 800-52, 800-57, 800-53

2. [x] **Cryptographic Module Usage Document** — `docs/crypto-module-usage.md`
   - [x] Table: Operation → Algorithm → Validated Module → Certificate #
   - [x] Statement: no cryptographic algorithms implemented in application layer
   - [x] Covers all three segments with honest assessment of each

3. [x] **FIPS Compliance Justification Letter template** — `docs/fips-justification-letter.md`
   - [x] Structured AO argument for leveraged validated module approach
   - [x] Precedent: RHEL, Windows, most enterprise software
   - [x] Cites NIST FIPS 140-2 Implementation Guidance
   - [x] Honest about Segment 1 (client side)

4. [x] **Client Endpoint Hardening Guide** — `docs/client-hardening-guide.md`
   - [x] Windows: step-by-step GPO configuration for FIPS mode
   - [x] Windows: Firefox NSS FIPS mode configuration
   - [x] RHEL: `fips=1` boot parameter and verification steps
   - [x] macOS: documentation of always-active validated module, verification steps
   - [x] MDM policy templates for Intune and Jamf

5. [ ] **SBOM** — auto-generated at build time (CycloneDX + SPDX)
   - [ ] CycloneDX output
   - [ ] SPDX output
   - [ ] Crypto dependency flagging

6. [x] **Continuous Monitoring Plan template** — `docs/continuous-monitoring-plan.md`
   - [x] Dashboard as continuous monitoring tool
   - [x] Process for re-verifying validated module status on updates
   - [x] Vulnerability response process

7. [x] **Incident Response Addendum** — `docs/incident-response-addendum.md`
   - [x] Procedure when a compliance check goes red
   - [x] Cryptographic failure reporting

### Additional Reference Docs

- [x] **NIST 800-53 Control Mapping** — `docs/control-mapping.md` (SC-8, SC-13, SC-12, IA-7, SA-11, CM-14)
- [x] **Architecture Diagram** — `docs/architecture-diagram.md` (Mermaid diagrams of three-segment architecture)

---

## Phase 4 — CI/CD Cross-Platform Build Pipeline

### Target Platforms (10 total)

| Platform | Arch | Output | Status |
|----------|------|--------|--------|
| Linux (RPM) | amd64 | .rpm (RHEL 8/9) | [x] CI matrix entry |
| Linux (RPM) | arm64 | .rpm (RHEL 8/9 ARM) | [x] CI matrix entry |
| Linux (DEB) | amd64 | .deb (Ubuntu/Debian) | [x] CI matrix entry |
| Linux (DEB) | arm64 | .deb (Ubuntu/Debian ARM) | [x] CI matrix entry |
| macOS | arm64 (Apple Silicon) | .pkg / binary | [x] CI matrix entry |
| macOS | amd64 (Intel) | .pkg / binary | [x] CI matrix entry |
| Windows | amd64 | .msi / .exe | [x] CI matrix entry |
| Windows | arm64 | .msi / .exe | [x] CI matrix entry |
| Container | amd64 | OCI image (RHEL UBI 9 base) | [x] CI matrix entry |
| Container | arm64 | OCI image (RHEL UBI 9 base) | [x] CI matrix entry |

### Build Strategy

- [x] Primary FIPS build: RHEL UBI 9 container with `GOEXPERIMENT=boringcrypto`
- [x] Cross-compilation: Go native cross-compile (GOOS/GOARCH)
- [x] BoringCrypto on all platforms (document platform-specific caveats)
- [ ] Windows builds: cross-compile + MSI packaging via WiX toolset
- [ ] macOS builds: cross-compile + .pkg creation (document Apple code signing requirement)
- [ ] RPM packaging: rpmbuild in RHEL UBI 9 container
- [ ] DEB packaging: dpkg-deb in Debian container
- [ ] Reproducible builds: SOURCE_DATE_EPOCH, strip debug info, verify byte-identical output

### Build Metadata Artifact (`build-manifest.json`)

Schema (implemented):

```json
{
  "version": "1.0.0",
  "commit": "abc123",
  "build_time": "2026-02-20T00:00:00Z",
  "cloudflared_upstream_version": "2025.x.x",
  "cloudflared_upstream_commit": "def456",
  "crypto_engine": "boringcrypto",
  "boringssl_version": "...",
  "fips_certificates": [
    {"module": "BoringSSL", "certificate": "#3678", "algorithms": [...]},
    {"module": "RHEL OpenSSL", "certificate": "#4349", "algorithms": [...]}
  ],
  "target_platform": "linux/amd64",
  "package_format": "rpm",
  "sbom_sha256": "...",
  "binary_sha256": "..."
}
```

- [x] Schema defined in Go types (`pkg/manifest/types.go`)
- [x] Sample manifest (`configs/build-manifest.json`)
- [x] Generation script (`scripts/generate-manifest.sh`)
- [x] Dashboard displays manifest data
- [x] CI pipeline generates manifest per build

---

## Phase 5 — CloudSH Integration Interface

- [x] Single YAML config file (`configs/cloudflared-fips.yaml`)
- [ ] IPC interface: local Unix socket or gRPC for CloudSH queries
- [x] Structured JSON API for compliance data (not just rendered UI)
- [ ] RPM/DEB/container artifacts self-contained and importable as CloudSH plugin
- [x] All API paths prefixed with `cloudflared-fips/` namespacing (`/api/v1/`)

---

## Technical Stack

| Component | Technology | Status |
|-----------|-----------|--------|
| cloudflared build | Go 1.24 + `GOEXPERIMENT=boringcrypto`, RHEL UBI 9 | [x] Scaffolded |
| Dashboard backend | Go (sidecar binary) | [x] Scaffolded |
| Dashboard frontend | React + TypeScript + Tailwind | [x] Built |
| Compliance checks | syscalls + metrics API + Cloudflare API + TLS probing | [ ] Stub only |
| Doc generation | Go templates → Markdown → PDF via pandoc | [ ] Not started |
| CI | GitHub Actions with RHEL UBI 9 container | [x] Scaffolded |
| Packaging | rpmbuild / dpkg-deb / WiX / OCI buildah | [ ] Stubs only |

---

## Constraints & Principles

- Never implement custom cryptography
- Fail closed — unknown compliance state = red, not green
- Transparency: every check shows its source data, not just pass/fail
- Reproducible builds — byte-identical output from same source
- Dashboard local-only by default
- Air-gap friendly — all runtime dependencies bundled
- Open source (Apache 2.0) — business model is support, deployment services, AO package consulting

---

## File Structure

```
/workspace/
├── upstream-cloudflared/        # Cloned upstream cloudflared source
├── go.mod                       # Go module definition
├── Makefile                     # Build targets
├── README.md                    # Project overview
├── CLAUDE.md                    # This file — spec & roadmap
├── build/
│   ├── Dockerfile.fips          # RHEL UBI 9 FIPS build container
│   ├── build.sh                 # Build orchestrator (clones upstream)
│   └── patches/README.md
├── cmd/
│   ├── selftest/main.go         # Standalone self-test CLI
│   └── dashboard/main.go        # Dashboard server (localhost-only)
├── internal/
│   ├── selftest/                # Self-test orchestrator, ciphers, KATs
│   ├── compliance/              # Compliance state aggregation
│   └── dashboard/               # HTTP API handlers + SSE
├── pkg/
│   ├── buildinfo/               # Linker-injected build metadata
│   └── manifest/                # Build manifest types + read/write
├── dashboard/                   # React + TypeScript + Tailwind (Vite)
│   └── src/
│       ├── types/compliance.ts  # TS types matching spec schema
│       ├── data/mockData.ts     # Mock data: all 39 checklist items
│       ├── components/          # StatusBadge, ChecklistItem, etc.
│       └── pages/DashboardPage.tsx
├── configs/
│   ├── cloudflared-fips.yaml    # Sample tunnel + FIPS config
│   └── build-manifest.json      # Sample manifest (spec schema)
├── scripts/
│   ├── check-fips.sh            # Post-build FIPS validation
│   ├── generate-manifest.sh     # Produce build-manifest.json
│   └── verify-boring.sh         # Verify BoringCrypto symbols
├── docs/
│   ├── ao-narrative.md          # SSP template
│   ├── crypto-module-usage.md   # Operation → Algorithm → Module → Cert
│   ├── fips-justification-letter.md  # AO justification template
│   ├── client-hardening-guide.md     # Windows/RHEL/macOS/MDM guide
│   ├── continuous-monitoring-plan.md # Ongoing compliance verification
│   ├── incident-response-addendum.md # Crypto failure procedures
│   ├── control-mapping.md       # NIST 800-53 control mapping
│   └── architecture-diagram.md  # Mermaid diagrams
└── .github/workflows/
    ├── fips-build.yml           # 6-entry matrix (10 platform targets)
    └── compliance-check.yml     # PR validation
```

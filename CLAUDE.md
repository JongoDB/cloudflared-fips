# cloudflared-fips — Product Spec & Implementation Roadmap

> This file is the canonical reference for the cloudflared-fips product.
> It captures the full original spec and tracks implementation progress.
> Update checkboxes as work is completed.

---

## Progress Summary (last updated: 2026-02-21, post-P3)

| Area | Status | Notes |
|------|--------|-------|
| **FIPS build system** | **Working** | Dockerfile.fips builds on RHEL UBI 9 with BoringCrypto verification + self-test gates. build.sh orchestrates full flow. |
| **Self-test suite** | **Working** | Real NIST CAVP vectors. BoringCrypto detection, OS FIPS mode, cipher suite validation, KATs. |
| **Compliance dashboard** | **Working (mock data)** | All 39 items with verification badges (Direct/API/Probe/Inherited/Reported). SSE hook + Live toggle. JSON export. |
| **AO documentation** | **Complete (templates)** | 8 docs: SSP, crypto usage, justification letter, hardening guide, monitoring plan, IR addendum, control mapping, architecture. |
| **CI — compliance** | **Working** | Go lint/test, dashboard lint/build, docs check, manifest validation, shell syntax. |
| **CI — build matrix** | **Working** | Per-OS jobs: Linux/RHEL with boringcrypto, macOS with Go native FIPS, Windows with Go native FIPS. `if-no-files-found: error`. |
| **Packaging** | **Implemented** | RPM (.spec + rpmbuild), DEB (dpkg-deb + build script), macOS .pkg (pkgbuild + productbuild), Windows MSI (WiX v4). All include self-test post-install. |
| **SBOM** | **Implemented** | `scripts/generate-sbom.sh` produces CycloneDX 1.5 + SPDX 2.3 from `go mod`. Crypto audit JSON. CI wired. |
| **Artifact signing** | **Implemented** | `pkg/signing/` (GPG + cosign), `scripts/sign-artifacts.sh`, CI `sign-artifacts` job runs on tags. |
| **SSE real-time** | **Frontend + backend** | Go SSE handler works. React `useComplianceSSE` hook with auto-reconnect, Live toggle, connection status. |
| **Live compliance checks** | **Implemented** | Local: `LiveChecker` (BoringCrypto, OS FIPS, KATs, ciphers, binary integrity, tunnel metrics, TLS probing). Edge: `cfapi.ComplianceChecker` (9 checks). Client: `clientdetect.ComplianceChecker` (8 checks). |
| **PDF export** | **Not implemented** | Intentional stub returning install instructions for pandoc. |
| **quic-go audit** | **Not started** | Need to verify quic-go TLS routes through BoringCrypto, not its own crypto. |
| **OS support matrix** | **Needs update** | Product supports any FIPS-mode Linux (RHEL, Ubuntu Pro, Amazon Linux, SLES, Oracle, Alma), not just RHEL. Docs/dashboard should reflect this. |
| **FIPS 140-2 sunset** | **Implemented** | `pkg/fipsbackend/migration.go` tracks sunset (Sept 21, 2026). Dashboard sunset banner with countdown + urgency. API: `/api/v1/migration`. |
| **Edge FIPS honesty** | **Implemented** | Cloudflare Edge items show "Inherited" or "API" verification badges. Tooltip explains FedRAMP reliance. |
| **macOS/Windows targets** | **CI implemented** | Per-OS CI jobs use `GODEBUG=fips140=on` (Go native FIPS 140-3, CAVP A6650, CMVP pending). |
| **Modular crypto backend** | **Implemented** | `pkg/fipsbackend/` with Backend interface. BoringCrypto, GoNative, SystemCrypto backends. `Detect()` auto-selects. Live checker uses it. Dashboard display TODO. |
| **Deployment tiers** | **Implemented** | `pkg/deployment/tier.go`, `cmd/fips-proxy/main.go` (Tier 3 proxy), `build/Dockerfile.fips-proxy`, config `deployment_tier` field, dashboard badge. API: `/api/v1/deployment`. |
| **Client FIPS detection** | **Implemented** | `pkg/clientdetect/` with TLS Inspector (ClientHello analysis, JA4 fingerprinting), PostureCollector (device agent API), ComplianceChecker (8-item Client Posture section). |
| **Dashboard honesty indicators** | **Implemented** | VerificationBadge component. All 39 items tagged: direct/api/probe/inherited/reported. Color-coded with tooltips. |

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

## Critical Architecture Findings (researched 2026-02-20)

### Finding 1: BoringCrypto is statically linked — OS OpenSSL is NOT required for our binary

`GOEXPERIMENT=boringcrypto` compiles pre-built BoringSSL `.syso` object files directly into the Go binary. The FIPS-validated crypto (CMVP #3678, #4407) travels with the binary. It does **not** call the host OS's OpenSSL.

**Implication:** Our cloudflared-fips binary uses FIPS-validated crypto on ANY Linux (amd64/arm64), not just RHEL. However, an AO will want the entire system stack in FIPS mode (kernel crypto, SSH, disk encryption), which requires an OS with its own FIPS-validated modules.

### Finding 2: Supported host OSes (broader than RHEL)

| Distro | FIPS Validation | OS FIPS Mode | Notes |
|--------|----------------|--------------|-------|
| RHEL 8/9 | 140-2 + 140-3 | `fips=1` boot param | Industry standard for FedRAMP |
| Ubuntu Pro 20.04/22.04 | 140-2 + 140-3 | `fips=1` boot param | Requires Pro subscription |
| Amazon Linux 2/2023 | 140-2 + 140-3 | `fips-mode-setup --enable` | Native AWS integration |
| SUSE SLES 15 | 140-3 | `fips=1` boot param | Common in enterprise |
| Oracle Linux 8/9 | 140-2 + 140-3 | `fips=1` boot param | |
| AlmaLinux 9.2+ | 140-3 | `fips=1` boot param | |

### Finding 3: `GOEXPERIMENT=boringcrypto` is Linux-only

Build constraint: `//go:build boringcrypto && linux && (amd64 || arm64) && !android && !msan`

**No Windows. No macOS.** The `.syso` files only exist for linux/amd64 and linux/arm64. This means:
- Our CI matrix entries for macOS/Windows with BoringCrypto **will always fail** (currently swallowed by `|| echo`)
- Alternatives for non-Linux: Microsoft Build of Go (`systemcrypto` → CNG/CommonCrypto), or Go 1.24+ native FIPS 140-3 module (`GODEBUG=fips140=on`) — but the native module has **not yet completed CMVP validation**

### Finding 4: FIPS 140-2 sunset — September 21, 2026

All FIPS 140-2 certificates (including BoringCrypto #3678, #4407) move to the CMVP Historical List on **September 21, 2026**. After that date, only FIPS 140-3 validated modules are acceptable for new federal acquisitions. Our product must plan migration to either:
- Go 1.24+ native FIPS 140-3 module (CAVP cert A6650, CMVP validation in progress)
- BoringCrypto FIPS 140-3 (Google CMVP #4735)
- Microsoft Build of Go with system crypto backends

### Finding 5: Cloudflare's edge crypto is NOT independently FIPS-validated

**Cloudflare does NOT hold a CMVP certificate.** They are not listed as a vendor in the NIST CMVP database.

- Cloudflare uses BoringSSL on their edge (migrated from OpenSSL in 2017)
- BoringSSL contains a FIPS-validated sub-module (BoringCrypto), but the CMVP certificates belong to **Google, Inc.**, not Cloudflare
- Using the same code ≠ having your own validation. NIST: "non-validated cryptography is viewed as providing no protection"
- There is no Cloudflare API endpoint to verify FIPS mode is enabled on the edge server handling your request

**What Cloudflare DOES have:**
- FedRAMP Moderate authorization (FedRAMP High in process)
- Configurable FIPS-compliant cipher suites (verifiable via API)
- Keyless SSL for customer-controlled FIPS 140-2 Level 3 HSMs
- Cloudflare for Government (same infrastructure, logical separation via Regional Services)

**What we CAN verify about the edge externally:**
- Negotiated TLS cipher suite (TLS probing: `openssl s_client`, `testssl.sh`)
- TLS version (1.2/1.3)
- Cipher suite configuration (API: `GET /zones/{zone_id}/settings/ciphers`)
- Minimum TLS version setting (API)
- Edge certificate validity and chain

**What we CANNOT verify:**
- Whether the edge crypto module runs in FIPS mode
- Which BoringSSL build/version is running
- Internal Cloudflare backbone encryption
- Key storage protections at rest (unless using Keyless SSL)

### Finding 6: Dashboard honesty implications

Our dashboard's "Cloudflare Edge" section shows 9 items. The honest status:

| Item | Verifiable? | How |
|------|-------------|-----|
| Access policy active | Yes | Cloudflare API |
| IdP connected | Yes | Cloudflare API |
| Auth method | Yes | Cloudflare API |
| MFA enforced | Yes | Cloudflare API |
| Cipher suite restriction | Yes (config) | API + TLS probe — but cannot verify the **module** is validated |
| Min TLS version | Yes (config) | API |
| Edge certificate valid | Yes | TLS probe |
| HSTS enforced | Yes (config) | HTTP header check |
| Last auth event | Yes | Cloudflare API |

**None of these verify that Cloudflare's edge crypto module is FIPS-validated.** The dashboard should clearly indicate: "Cipher configuration verified via API; edge crypto module FIPS validation inherited through Cloudflare's FedRAMP Moderate authorization, not independently verifiable."

### Finding 7: The cloudflared binary from Cloudflare is NOT FIPS-built

The official `cloudflared` binary distributed by Cloudflare uses standard Go crypto. It is not built with `GOEXPERIMENT=boringcrypto` or `GODEBUG=fips140=on`. **This is the entire reason our product exists.**

**However:** Cloudflare's repo does contain `build-packages-fips.sh` and `check-fips.sh` — they have the scripts to produce FIPS builds but do not ship them as default.

---

## Go FIPS Crypto Module Matrix

### Four approaches, compared

| | BoringCrypto (Google) | Go Native FIPS 140-3 | Microsoft systemcrypto | Red Hat golang-fips |
|---|---|---|---|---|
| **Mechanism** | Static `.syso` linked into binary | Pure Go, compiled in | `dlopen` to platform libs | `dlopen` to OpenSSL |
| **Requires CGO** | Yes | **No** | Yes (Linux); No (Win/Mac) | Yes |
| **FIPS 140-2 certs** | #2964, #3318, #3678, #3753, #4156, #4407 | None | Windows CNG #4825 | Via host OS OpenSSL |
| **FIPS 140-3 certs** | **#4735**, #4953 (interim) | **None yet** (CAVP A6650, MIP list) | OpenSSL #4985 (Linux) | Via RHEL OpenSSL #4746, #4857 |
| **Linux amd64** | Yes | Yes | Yes | Yes |
| **Linux arm64** | Yes | Yes | Yes | Yes |
| **Windows amd64** | **No** | Yes | Yes (CNG) | No |
| **Windows arm64** | **No** | Yes | Yes (CNG) | No |
| **macOS amd64** | **No** | Yes | Yes (CommonCrypto) | No |
| **macOS arm64** | **No** | Yes | Yes (CommonCrypto) | No |
| **Build flag** | `GOEXPERIMENT=boringcrypto` | `GODEBUG=fips140=on` | `GOEXPERIMENT=systemcrypto` | Auto-detects OS FIPS mode |
| **Future** | Planned removal once Google migrates | **The future standard** | Default in MS Go 1.25+ | Planned sunset for upstream native |

### Host OS FIPS Validation Matrix

| OS / Distro | FIPS 140-2 | FIPS 140-3 | FIPS Mode Activation | Notes |
|-------------|-----------|-----------|---------------------|-------|
| RHEL 8 | Validated (OpenSSL, GnuTLS, Kernel) | — | `fips-mode-setup --enable` | Industry standard for FedRAMP |
| RHEL 9 | — | Validated (#4746, #4857, #4780, #4796) | `fips-mode-setup --enable` | First OS with full 140-3 suite |
| Ubuntu Pro 20.04 | Validated (#2888, #2962) | — | `ua enable fips` | Requires Pro subscription |
| Ubuntu Pro 22.04 | — | Validated | `ua enable fips` | Requires Pro subscription |
| Amazon Linux 2 | Validated (#3553, #3567, #4593) | — | `sudo fips-mode-setup --enable` | Native AWS integration |
| Amazon Linux 2023 | — | Validated (June 2025) | `sudo fips-mode-setup --enable` | FIPS 140-3 Level 1 |
| SUSE SLES 15 SP6+ | — | Validated (OpenSSL 3) | `fips=1` boot param | |
| Oracle Linux 8 | Validated (#4215, #3893) | — | `fips-mode-setup --enable` | |
| Oracle Linux 9 | — | Validated | `fips-mode-setup --enable` | |
| AlmaLinux 9.2+ | — | Validated | `fips-mode-setup --enable` | Funded by CloudLinux |
| Windows Server 2022+ | Validated (CNG #4825) | In process | GPO: "Use FIPS compliant algorithms" | |
| macOS (all recent) | Validated (CommonCrypto, per-release) | In process | Always active — no switch | Apple submits per major release |

### FIPS 140-2 Sunset: September 21, 2026

All 140-2 certificates (including BoringCrypto #3678/#4407, Windows CNG #4825) move to the CMVP Historical List. After that date, only 140-3 validated modules are acceptable for new federal acquisitions.

**Our migration path (in priority order):**
1. **BoringCrypto 140-3 (#4735)** — already validated, works today with `GOEXPERIMENT=boringcrypto` on Linux. Verify the Go integration uses the 140-3 certified version.
2. **Go native FIPS 140-3** — once CMVP validates (currently MIP). Broadest platform support, no CGO. This is the long-term answer.
3. **Microsoft systemcrypto** — for Windows/macOS targets where BoringCrypto doesn't work. Delegates to platform-validated modules.

**Product design: the crypto backend should be modular.** Users must be able to select their FIPS module based on their deployment platform and compliance requirements. The dashboard should display which module is in use and its exact CMVP certificate number and FIPS standard (140-2 vs 140-3).

---

## End-to-End Observability Architecture

### The Gap: Cloudflare's Edge

Cloudflare Tunnel **requires** traffic to flow through Cloudflare's edge. There is no bypass. The edge uses BoringSSL but does not hold its own CMVP certificate. This creates an observability gap.

### Deployment Tiers (increasing FIPS assurance)

#### Tier 1: Standard Cloudflare Tunnel (current architecture)
```
Client → Cloudflare Edge → cloudflared-fips → Origin
```
- **What we control:** Segment 2 (our binary) and Segment 3 (origin)
- **What we verify:** Edge cipher config (API), TLS version (probe), client cipher suite (ClientHello)
- **What we trust:** Cloudflare's FedRAMP Moderate authorization
- **Gap:** Edge crypto module not independently FIPS-validated

#### Tier 2: Cloudflare Tunnel + Regional Services + Keyless SSL
```
Client → Cloudflare FedRAMP DC (Regional Services) → Keyless SSL (customer HSM) → cloudflared-fips → Origin
```
- **Additional controls:** TLS termination restricted to FedRAMP-compliant US data centers; private keys stay in customer's FIPS 140-2 Level 3 HSMs (AWS CloudHSM, Azure HSM, etc.)
- **Improvement:** Key material never touches Cloudflare's infrastructure
- **Remaining gap:** Bulk encryption (AES-GCM) still performed by Cloudflare's edge BoringSSL

#### Tier 3: Self-Hosted FIPS Edge Proxy (full control)
```
Client → FIPS Proxy (GovCloud) → [optional: Cloudflare for WAF/DDoS] → cloudflared-fips → Origin
```
- **Full control:** Client TLS terminates at a FIPS-validated proxy we control (our Go binary with BoringCrypto, or nginx with FIPS OpenSSL) deployed in AWS GovCloud / Azure Government / Google Assured Workloads
- **Optional Cloudflare:** Can still use Cloudflare on the backend for security services (WAF, DDoS, bot management) with a separate tunnel
- **No gap:** Every TLS termination point uses a FIPS-validated module we control or can verify

### FedRAMP Cloud Environments for Tier 3

| Provider | FedRAMP Level | HSM | FIPS OS Options |
|----------|--------------|-----|-----------------|
| AWS GovCloud | High + IL5 | CloudHSM (Level 3) | RHEL, Amazon Linux, Ubuntu Pro |
| Azure Government | High | Dedicated HSM (Level 3) | Windows Server, RHEL |
| Google Assured Workloads | High (select) | Cloud HSM (Level 3) | RHEL, custom |
| Oracle Cloud Government | High + FedRAMP+ | Software (Level 1) | Oracle Linux |

### Client-Side FIPS Observability

#### What we CAN detect (and should implement)

| Method | What it detects | Reliability | Implementation |
|--------|----------------|-------------|----------------|
| **TLS ClientHello inspection** | Absence of ChaCha20-Poly1305 = FIPS mode | High | Go `tls.Config.GetConfigForClient` → inspect `ClientHelloInfo.CipherSuites` |
| **JA3/JA4 fingerprinting** | FIPS-mode browser produces distinct fingerprint | High | Cloudflare exposes JA3/JA4 in WAF rules; or compute server-side |
| **WARP + custom device posture** | OS FIPS mode enabled | High | Custom service-to-service API checking `/proc/sys/crypto/fips_enabled` (Linux) or registry key (Windows) |
| **Cloudflare Access device posture** | MDM enrollment, disk encryption, OS version | High | Native Cloudflare Access integration |
| **Negotiated cipher suite logging** | Which cipher was actually used | Definitive | Server-side TLS connection metadata |

#### What we CANNOT detect

| Gap | Why | Mitigation |
|-----|-----|------------|
| Client crypto module is FIPS-**validated** (has CMVP cert) | No external API to query a browser's CMVP status | Document in AO package; require managed endpoints with known OS/browser combinations |
| Client browser uses BoringSSL vs OS crypto | Chrome on Linux uses BoringSSL regardless of OS FIPS mode | Document as known gap; recommend Firefox with NSS FIPS mode on Linux |
| Non-managed device compliance | BYOD can't be verified | Require WARP + MDM enrollment via Cloudflare Access |

### Dashboard Honesty Requirements

Each dashboard section should display a **verification method** indicator:

| Indicator | Meaning |
|-----------|---------|
| **Verified (direct)** | We directly measured this value (e.g., self-test result, binary hash) |
| **Verified (API)** | Confirmed via Cloudflare API or system call (e.g., cipher config, OS FIPS mode) |
| **Verified (probe)** | Confirmed via TLS handshake inspection (e.g., negotiated cipher) |
| **Inherited (FedRAMP)** | Relies on Cloudflare's FedRAMP authorization, not independently verified |
| **Reported (client)** | Client-reported via WARP/device posture — trust depends on endpoint management |

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
- [ ] Cross-compile build step must fail (not silently succeed) when compilation errors occur — currently `|| echo` swallows failures in CI

### Startup Self-Test

- [x] Confirms BoringCrypto is the active crypto provider (not standard Go crypto)
- [x] Confirms OS FIPS mode is enabled (`/proc/sys/crypto/fips_enabled == 1` on Linux; document macOS and Windows equivalents)
- [x] Confirms only FIPS-approved cipher suites are configured
- [x] Runs BoringCrypto's built-in known-answer tests (KATs) — real NIST CAVP vectors for AES-GCM, SHA-256, SHA-384, HMAC-SHA-256, ECDSA P-256, RSA-2048
- [x] Refuses to start if any check fails with clear, actionable error messages
- [x] Logs all results to structured JSON for compliance audit trail

### SBOM Generation

- [ ] Auto-generate SBOM at build time listing every dependency and version — **current CI generates skeleton JSON with correct schema headers but no actual dependency enumeration; needs `cyclonedx-gomod` or `spdx-sbom-generator`**
- [ ] Flag whether each dependency performs cryptographic operations
- [ ] For each crypto dependency: which validated module it delegates to
- [ ] Output as CycloneDX and SPDX format (both accepted by government procurement) — **schema stubs exist, need real tooling**

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

- [x] React + TypeScript frontend, Tailwind CSS — built and verified rendering
- [x] Mock data covering all three segments (validates UX before wiring real sources)
- [x] Export compliance report as JSON — working via ExportButtons component (Blob download)
- [x] PDF export stub (requires pandoc backend) — intentional stub with install instructions
- [x] Dashboard localhost-only by default — Go server binds `127.0.0.1:8080`
- [x] Air-gap friendly: all assets bundled, no CDN dependencies at runtime
- [ ] Backend: Go sidecar service querying local system state, cloudflared /metrics, Cloudflare API — **handler scaffolded but serves static checker data, not live system queries**
- [ ] Active TLS probing (connect to tunnel hostname, inspect negotiated cipher/TLS version)
- [ ] Cloudflare API integration (Access policy status, tunnel health, cipher config)
- [ ] MDM API integration (Intune/Jamf) for client posture data
- [ ] Real-time updates via SSE — **Go backend SSE handler implemented (`/api/v1/events`, 30s ticker), but React frontend has NO EventSource consumer wired up**
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

5. [ ] **SBOM** — auto-generated at build time (CycloneDX + SPDX) — **CI produces skeleton JSON with correct schema headers but no actual dependency enumeration**
   - [ ] CycloneDX output — schema stub exists, needs `cyclonedx-gomod` or equivalent
   - [ ] SPDX output — schema stub exists, needs `spdx-sbom-generator` or equivalent
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

| Platform | Arch | Output | CI Matrix | Compile | Package |
|----------|------|--------|-----------|---------|---------|
| Linux (RPM) | amd64 | .rpm (RHEL 8/9) | [x] | [x] native | [ ] rpmbuild stub |
| Linux (RPM) | arm64 | .rpm (RHEL 8/9 ARM) | [x] | [~] cross-compile, needs toolchain | [ ] rpmbuild stub |
| Linux (DEB) | amd64 | .deb (Ubuntu/Debian) | [x] | [x] native | [ ] dpkg-deb stub |
| Linux (DEB) | arm64 | .deb (Ubuntu/Debian ARM) | [x] | [~] cross-compile, needs toolchain | [ ] dpkg-deb stub |
| macOS | arm64 (Apple Silicon) | .pkg / binary | [x] | [ ] CGO cross-compile fails silently | [ ] no .pkg step |
| macOS | amd64 (Intel) | .pkg / binary | [x] | [ ] CGO cross-compile fails silently | [ ] no .pkg step |
| Windows | amd64 | .msi / .exe | [x] | [ ] CGO cross-compile fails silently | [ ] WiX stub |
| Windows | arm64 | .msi / .exe | [x] | [ ] CGO cross-compile fails silently | [ ] no MSI step |
| Container | amd64 | OCI image (RHEL UBI 9 base) | [x] | [x] via Dockerfile.fips | [ ] buildah stub |
| Container | arm64 | OCI image (RHEL UBI 9 base) | [x] | [~] Dockerfile.fips needs multi-arch | [ ] buildah stub |

> **Key issue:** `GOEXPERIMENT=boringcrypto` requires `CGO_ENABLED=1`, which means cross-compiling to macOS/Windows from a Linux runner needs a C cross-compiler for each target OS. The current CI swallows these failures with `|| echo`. Options: (a) use native macOS/Windows runners, (b) use CGO_ENABLED=0 and accept no BoringCrypto for those platforms, or (c) use a cross-compilation toolchain like zig cc.

### Build Strategy

- [x] Primary FIPS build: RHEL UBI 9 container with `GOEXPERIMENT=boringcrypto` — Dockerfile.fips is complete with BoringCrypto verification and self-test gates
- [x] Cross-compilation: Go native cross-compile (GOOS/GOARCH) — CI matrix covers all 6 entries / 10 targets, **but non-linux CGO cross-compile will fail silently (swallowed by `|| echo`); needs native toolchains or separate runners**
- [x] BoringCrypto on all platforms (document platform-specific caveats)
- [ ] Windows builds: cross-compile + MSI packaging via WiX toolset — **CI step is echo stub only**
- [ ] macOS builds: cross-compile + .pkg creation (document Apple code signing requirement) — **CI step outputs binary only, no .pkg packaging**
- [ ] RPM packaging: rpmbuild in RHEL UBI 9 container — **CI step is echo stub; no .spec file exists**
- [ ] DEB packaging: dpkg-deb in Debian container — **CI step is echo stub; no DEBIAN/control file exists**
- [ ] OCI container packaging: buildah/docker build — **CI step is echo stub; Dockerfile.fips exists but is not invoked by the CI packaging step**
- [ ] Reproducible builds: SOURCE_DATE_EPOCH, strip debug info, verify byte-identical output
- [ ] Artifact signing: cosign for containers, GPG for binaries — **CI step is echo stub with documentation only**

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
- [x] Structured JSON API for compliance data (not just rendered UI) — `/api/v1/compliance`, `/api/v1/manifest`, `/api/v1/selftest`, `/api/v1/health`, `/api/v1/compliance/export`
- [ ] RPM/DEB/container artifacts self-contained and importable as CloudSH plugin — **packaging not yet implemented**
- [x] All API paths prefixed with `cloudflared-fips/` namespacing (`/api/v1/`)

---

## Phase 6 — End-to-End FIPS Observability Implementation Roadmap

This phase turns the architecture research (Findings 1-7, Deployment Tiers, Crypto Module Matrix) into working code. Ordered by dependency — later items build on earlier ones.

### 6.1 Modular Crypto Backend

**Goal:** Users select their FIPS module per platform. Dashboard reports active module + CMVP cert.

- [x] Add `pkg/fipsbackend/` package with `Backend` interface (Name, DisplayName, CMVPCertificate, FIPSStandard, Validated, Active, SelfTest)
- [x] Implement `BoringCrypto` backend — detects via TLS cipher suite restriction heuristic
- [x] Implement `GoNative` backend — detects via `GODEBUG=fips140=on` env var
- [x] Implement `SystemCrypto` backend — stub for Microsoft Go fork (`GOEXPERIMENT=systemcrypto`)
- [x] `Detect()` auto-selects active backend; `DetectInfo()` returns JSON-serializable `Info` struct
- [x] Build manifest `crypto_engine` field records backend per platform in CI
- [ ] Dashboard: display active backend, cert number, validation status, and 140-2 vs 140-3 badge
- [ ] Self-test suite: dispatch to backend-specific KAT runners

### 6.2 Fix CI Cross-Platform Builds

**Goal:** Every CI matrix entry either produces a real binary or fails visibly.

- [x] Remove `|| echo` from cross-compile step — builds must fail loudly
- [x] Split CI matrix into platform-specific jobs:
  - **linux/amd64, linux/arm64:** `GOEXPERIMENT=boringcrypto` + `CGO_ENABLED=1` on RHEL UBI 9 (`build-linux` job)
  - **linux/arm64 from amd64:** Install `gcc-aarch64-linux-gnu`, set `CC=aarch64-linux-gnu-gcc`
  - **windows/amd64, windows/arm64:** `runs-on: windows-latest` with `GODEBUG=fips140=on` (`build-windows` job)
  - **darwin/amd64, darwin/arm64:** `runs-on: macos-13/macos-14` with `GODEBUG=fips140=on` (`build-darwin` job)
  - container builds: stub — see 6.3
- [x] Build manifest records which FIPS backend was used (`crypto_engine` field per platform)
- [x] `if-no-files-found: error` ensures missing artifacts fail the job
- [ ] Self-test: runs on all platforms where native execution is possible

### 6.3 Real Packaging

**Goal:** Produce installable artifacts, not echo stubs.

- [x] **RPM:** `build/packaging/rpm/cloudflared-fips.spec` with systemd unit; CI runs `rpmbuild`
- [x] **DEB:** `build/packaging/deb/` with DEBIAN/control, postinst, prerm; `build-deb.sh` runs `dpkg-deb`
- [x] **OCI container:** CI invokes `podman build -f build/Dockerfile.fips` (podman/docker required)
- [x] **MSI (Windows):** `build/packaging/windows/cloudflared-fips.wxs` for WiX v4; CI runs `wix build`
- [x] **macOS .pkg:** `build/packaging/macos/build-pkg.sh` using `pkgbuild` + `productbuild` with optional codesigning
- [x] All packages include: binary, self-test, sample config, build manifest
- [x] All packages run self-test as post-install verification

### 6.4 Real SBOM Generation

**Goal:** Full dependency-level SBOMs flagging crypto operations.

- [x] `scripts/generate-sbom.sh`: uses `cyclonedx-gomod` when available, falls back to `go mod` graph
- [x] Generates CycloneDX 1.5 and SPDX 2.3 SBOMs with real module dependencies
- [x] Post-process: `crypto-audit.json` lists all `crypto/*` imports and annotates FIPS module
- [x] CI installs `cyclonedx-gomod` and calls `generate-sbom.sh` (replaces echo stubs)
- [x] SBOM hashes computed by script; `sbom_sha256` in manifest is real

### 6.5 Dashboard — Wire SSE to Frontend

**Goal:** Real-time compliance updates from Go backend to React frontend.

- [x] Add `useComplianceSSE()` React hook (`dashboard/src/hooks/useComplianceSSE.ts`)
- [x] Parse SSE events (full snapshot + incremental patch), update dashboard state
- [x] Show connection status indicator (connected / reconnecting / disconnected) with Live toggle
- [x] Fallback to mock data when SSE disabled or disconnected (air-gap friendly)
- [ ] Add visual indicator when data refreshes (subtle flash or timestamp update)

### 6.6 Dashboard — Honesty Indicators

**Goal:** Every checklist item shows HOW it was verified.

- [x] Add `VerificationMethod` type and `verificationMethodInfo` to `compliance.ts`:
  - `"direct"` — measured locally (self-test, binary hash, OS FIPS mode)
  - `"api"` — queried from Cloudflare API (Access policy, cipher config)
  - `"probe"` — TLS handshake inspection (negotiated cipher, TLS version)
  - `"inherited"` — relies on provider's FedRAMP authorization
  - `"reported"` — client-reported via WARP/device posture
- [x] Display `VerificationBadge` component next to severity badge in `ChecklistItem`
- [x] Update mock data with correct verification methods for all 39 items
- [x] Tooltip on badge explains what each method means (via `title` attribute)

### 6.7 Live Compliance Checks — Local System

**Goal:** Replace mock data with real system queries for Segment 2 and 3 items.

- [x] `checkBoringCryptoActive()` — detects active FIPS backend via `fipsbackend.Detect()`
- [x] `checkOSFIPSMode()` — reads `/proc/sys/crypto/fips_enabled` on Linux
- [x] `checkBinaryIntegrity()` — SHA-256 of running binary vs manifest `binary_sha256`
- [x] `checkCipherSuites()` — verifies all TLS suites are FIPS-approved
- [x] `checkTunnelProtocol()` / `checkTunnelRedundancy()` — queries cloudflared metrics endpoint
- [x] `checkLocalTLSEnabled()` / `checkLocalCipherSuite()` / `checkLocalCertificateValid()` — TLS probes ingress targets
- [x] `checkConfigDrift()` — verifies config file present (baseline hash comparison requires external CM)
- [x] `checkFIPSBackend()` — reports active backend name, CMVP cert, 140-2 vs 140-3 status
- [x] `LiveChecker` with functional options: `WithManifestPath`, `WithConfigPath`, `WithMetricsAddr`, `WithIngressTargets`
- [x] `cmd/dashboard/main.go` updated with `--config`, `--metrics-addr`, `--ingress-targets` flags

### 6.8 Live Compliance Checks — Cloudflare API

**Goal:** Replace mock data with real Cloudflare API queries for Edge items.

- [x] `pkg/cfapi/` with rate-limited, caching Cloudflare API client (bearer token auth)
- [x] `checkAccessPolicy()` — queries `/zones/{zone_id}/access/apps`
- [x] `checkCipherRestriction()` — queries `/zones/{zone_id}/settings/ciphers`, validates FIPS-approved names
- [x] `checkMinTLSVersion()` — queries `/zones/{zone_id}/settings/min_tls_version`, requires 1.2+
- [x] `checkEdgeCertificate()` — queries `/zones/{zone_id}/ssl/certificate_packs`, checks expiry
- [x] `checkHSTS()` — queries `/zones/{zone_id}/settings/security_header`
- [x] `checkTunnelHealth()` — queries `/accounts/{account_id}/cfd_tunnel/{tunnel_id}`
- [x] Token via `--cf-api-token` flag or `CF_API_TOKEN` env var
- [x] In-memory cache with configurable TTL (default 60s) to respect API limits

### 6.9 Live Compliance Checks — Client-Side FIPS Detection

**Goal:** Detect whether connecting clients use FIPS-capable TLS.

- [x] **TLS ClientHello inspection:** `pkg/clientdetect/Inspector` with `GetConfigForClient` callback
  - Detects ChaCha20-Poly1305 absence as FIPS signal
  - Checks for RC4/DES/3DES (banned) and AES-GCM (required)
  - Logs full cipher suite list per connection with rolling buffer (max 1000)
- [x] **JA4 fingerprinting:** Simplified JA4-style hash from ClientHello (version, cipher count, ALPN, sorted cipher hash)
  - `KnownFIPSFingerprints` map for matching (requires population from tested clients)
- [x] **Device posture API:** `PostureCollector` with HTTP handlers
  - `POST /api/v1/posture` — agents report OS FIPS mode, OS type, MDM enrollment, disk encryption
  - `GET /api/v1/posture` — list all device postures
  - `GET /api/v1/clients` — TLS inspection results + FIPS stats
- [x] `ComplianceChecker` produces "Client Posture" section (8 items) from TLS + posture data
- [ ] Populate `KnownFIPSFingerprints` from tested FIPS clients (Windows FIPS, RHEL, macOS)

### 6.10 Deployment Tier Support

**Goal:** Product supports all three deployment tiers documented in architecture.

#### Tier 1: Standard Cloudflare Tunnel (works today with 6.7 + 6.8)
- [x] Config option: `deployment_tier: standard`
- [x] Dashboard shows Tier 1 honesty indicators (edge items marked "inherited")

#### Tier 2: Regional Services + Keyless SSL
- [x] Config option: `deployment_tier: regional_keyless`
- [ ] Cloudflare API checks verify Regional Services is active for the zone
- [ ] Cloudflare API checks verify Keyless SSL is configured
- [x] Dashboard shows HSM status and key location (via DeploymentTierBadge)
- [ ] Documentation: setup guide for Keyless SSL + Cloudflare Tunnel integration

#### Tier 3: Self-Hosted FIPS Edge Proxy
- [x] `cmd/fips-proxy/main.go` — lightweight Go reverse proxy built with BoringCrypto
  - TLS termination with FIPS cipher enforcement
  - ClientHello inspection and JA4 fingerprinting built in
  - Forwards to origin or to Cloudflare for backend security services
  - Logs all TLS metadata for compliance dashboard consumption
- [x] Config option: `deployment_tier: self_hosted`
- [x] Dashboard connects to FIPS proxy for client-side TLS metadata
- [x] Dockerfile for deploying FIPS proxy in GovCloud (AWS/Azure/Google)
- [ ] Terraform/CloudFormation templates for GovCloud deployment (stretch goal)

### 6.11 FIPS 140-3 Migration

**Goal:** Smooth transition before September 21, 2026 deadline.

- [ ] Verify BoringCrypto FIPS 140-3 (#4735) works with current `GOEXPERIMENT=boringcrypto` integration
  - Check if Go ships the 140-3 certified `.syso` or the older 140-2 version
  - If 140-2 only: build from BoringSSL source at the 140-3 certified tag
- [ ] Track Go native FIPS 140-3 CMVP validation status (currently MIP, CAVP A6650)
  - When validated: add `GoNativeBackend` as primary recommendation
  - Update build matrix to use `GODEBUG=fips140=on` instead of `GOEXPERIMENT=boringcrypto`
- [x] Dashboard: show FIPS standard version (140-2 vs 140-3) prominently
- [x] Dashboard: show days until 140-2 sunset with warning if still using 140-2 module — SunsetBanner component with countdown, urgency colors, progress bar
- [x] `pkg/fipsbackend/migration.go`: MigrationStatus with urgency levels, days countdown, recommended actions per backend
- [x] API: `/api/v1/migration` returns current migration status, `/api/v1/migration/backends` returns all backend info
- [ ] Update AO documentation templates with 140-3 references and migration guidance
- [ ] Test all three deployment tiers with 140-3 modules

### 6.12 Artifact Signing

**Goal:** Cryptographically sign all build artifacts.

- [x] Container images: sign with cosign (Sigstore) in CI — `sign-artifacts` job with `sigstore/cosign-installer@v3`
- [x] Binaries and packages: sign with GPG key stored in GitHub Actions secrets — `scripts/sign-artifacts.sh` + CI job
- [x] `pkg/signing/signing.go`: GPGSign, GPGVerify, CosignSign, CosignVerify, SignatureManifest types
- [x] API: `/api/v1/signatures` returns signature manifest
- [ ] Publish public key for verification
- [ ] Self-test verifies binary signature at startup (optional, configurable)
- [x] Build manifest includes signature hashes — `signatures.json` generated per artifact directory
- [ ] Document key rotation procedure in AO package

### Implementation Priority

| Priority | Phase | Rationale |
|----------|-------|-----------|
| **P0 — Now** | 6.2 Fix CI builds | Broken builds undermine credibility |
| **P0 — Now** | 6.5 Wire SSE frontend | Dashboard feels static without it |
| **P0 — Now** | 6.6 Honesty indicators | Core differentiator — transparency |
| **P1 — Next** | 6.7 Local system checks | Replaces mock data with real values |
| **P1 — Next** | 6.1 Modular crypto backend | Required for multi-platform |
| **P1 — Next** | 6.3 Real packaging | Users need installable artifacts |
| **P2 — Soon** | 6.8 Cloudflare API integration | Fills edge section with real data |
| **P2 — Soon** | 6.4 Real SBOM | Government procurement requirement |
| **P2 — Soon** | 6.9 Client FIPS detection | End-to-end observability |
| **P3 — Plan** | 6.10 Deployment tiers | Infrastructure-dependent |
| **P3 — Plan** | 6.11 FIPS 140-3 migration | Deadline Sept 2026 |
| **P3 — Plan** | 6.12 Artifact signing | Trust chain completion |

---

## Technical Stack

| Component | Technology | Status | Roadmap |
|-----------|-----------|--------|---------|
| cloudflared build | Go 1.24 + `GOEXPERIMENT=boringcrypto`, RHEL UBI 9 | [x] Dockerfile.fips works e2e; build.sh orchestrates | — |
| Modular crypto backend | BoringCrypto / Go native / systemcrypto / RHEL OpenSSL | [ ] Not started | 6.1 |
| Dashboard backend | Go (sidecar binary) | [x] HTTP handlers, SSE endpoint, JSON export — static data | 6.7, 6.8 |
| Dashboard frontend | React + TypeScript + Tailwind | [x] Built, all 39 items, expandable, screenshots | 6.5, 6.6 |
| Self-test suite | Go (KATs, cipher validation, BoringCrypto detection) | [x] Real NIST vectors, real runtime checks | 6.1 |
| Local compliance checks | syscalls, cloudflared metrics, binary hash | [ ] Not started | 6.7 |
| Cloudflare API integration | Access, cipher config, tunnel health, certificates | [ ] Not started | 6.8 |
| Client FIPS detection | ClientHello inspection, JA4 fingerprint, WARP posture | [ ] Not started | 6.9 |
| FIPS edge proxy | Go reverse proxy with BoringCrypto for Tier 3 | [x] `cmd/fips-proxy/`, `build/Dockerfile.fips-proxy` | 6.10 |
| CI — compliance | GitHub Actions: lint, test, docs check, manifest | [x] Fully implemented | — |
| CI — build | GitHub Actions: 6-entry matrix, RHEL UBI 9 | [x] Partial — linux works; macOS/Windows broken | 6.2 |
| Packaging | rpmbuild / dpkg-deb / WiX / OCI / .pkg | [ ] All echo stubs | 6.3 |
| SBOM | CycloneDX + SPDX via real tooling | [ ] Schema stubs only | 6.4 |
| Artifact signing | cosign / GPG | [x] `pkg/signing/`, CI job, `sign-artifacts.sh` | 6.12 |
| SSE real-time | Go backend → React EventSource | [ ] Backend done; frontend not wired | 6.5 |
| Honesty indicators | Verification method badges on dashboard items | [ ] Not started | 6.6 |
| FIPS 140-3 migration | Module migration before Sept 2026 sunset | [x] `migration.go`, sunset banner, API endpoints | 6.11 |
| Doc generation | Go templates → Markdown → PDF via pandoc | [ ] Not started | — |

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
│   ├── Dockerfile.fips-proxy    # FIPS edge proxy container (Tier 3)
│   ├── build.sh                 # Build orchestrator (clones upstream)
│   ├── cloudflared-fips.spec    # [planned] RPM spec file
│   ├── debian/                  # [planned] DEB packaging (control, postinst, prerm)
│   └── patches/README.md
├── cmd/
│   ├── selftest/main.go         # Standalone self-test CLI
│   ├── dashboard/main.go        # Dashboard server (localhost-only)
│   └── fips-proxy/main.go       # FIPS edge proxy (Tier 3)
├── internal/
│   ├── selftest/                # Self-test orchestrator, ciphers, KATs
│   ├── compliance/              # Compliance state aggregation
│   ├── dashboard/               # HTTP API handlers + SSE
│   └── tlsprobe/                # [planned] TLS ClientHello inspector, JA4 fingerprinting
├── pkg/
│   ├── buildinfo/               # Linker-injected build metadata
│   ├── manifest/                # Build manifest types + read/write
│   ├── fipsbackend/             # Modular FIPS crypto backend (BoringCrypto/GoNative/SystemCrypto) + 140-3 migration
│   ├── cfapi/                   # Cloudflare API client (zone settings, Access, tunnel health)
│   ├── clientdetect/            # TLS ClientHello inspector, JA4 fingerprinting, device posture
│   ├── deployment/              # Deployment tier config (standard/regional_keyless/self_hosted)
│   └── signing/                 # Artifact signing (GPG + cosign) and verification
├── dashboard/                   # React + TypeScript + Tailwind (Vite)
│   └── src/
│       ├── types/compliance.ts  # TS types matching spec schema
│       ├── data/mockData.ts     # Mock data: all 39 checklist items
│       ├── hooks/               # useComplianceSSE (SSE real-time updates)
│       ├── components/          # StatusBadge, ChecklistItem, SunsetBanner, DeploymentTierBadge, etc.
│       └── pages/DashboardPage.tsx
├── configs/
│   ├── cloudflared-fips.yaml    # Sample tunnel + FIPS config
│   └── build-manifest.json      # Sample manifest (spec schema)
├── scripts/
│   ├── check-fips.sh            # Post-build FIPS validation
│   ├── generate-manifest.sh     # Produce build-manifest.json
│   ├── verify-boring.sh         # Verify BoringCrypto symbols
│   ├── sign-artifacts.sh         # CI artifact signing (GPG + cosign)
│   └── take-screenshots.cjs     # Headless dashboard screenshots
├── docs/
│   ├── ao-narrative.md          # SSP template
│   ├── crypto-module-usage.md   # Operation → Algorithm → Module → Cert
│   ├── fips-justification-letter.md  # AO justification template
│   ├── client-hardening-guide.md     # Windows/RHEL/macOS/MDM guide
│   ├── continuous-monitoring-plan.md # Ongoing compliance verification
│   ├── incident-response-addendum.md # Crypto failure procedures
│   ├── control-mapping.md       # NIST 800-53 control mapping
│   ├── architecture-diagram.md  # Mermaid diagrams
│   ├── deployment-tier-guide.md # [planned] Tier 1/2/3 deployment setup guides
│   └── screenshots/             # Dashboard screenshots (5 PNGs)
└── .github/workflows/
    ├── fips-build.yml           # 6-entry matrix (10 platform targets)
    └── compliance-check.yml     # PR validation
```

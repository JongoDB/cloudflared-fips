# Authorization to Operate (AO) Narrative

## System Security Plan — cloudflared-fips

### 1. System Description

cloudflared-fips is a FIPS 140-2/3 compliant build of Cloudflare's `cloudflared` tunnel client, designed for deployment in federal and regulated environments requiring validated cryptographic modules.

The system operates across three network segments:

1. **Client to Cloudflare Edge** — End-user devices connect to the Cloudflare global network via WARP client or direct HTTPS, using FIPS-validated TLS
2. **Cloudflare Edge** — Cloudflare's network applies Zero Trust access policies, DNS/HTTP filtering, and DDoS protection; edge crypto relies on Cloudflare's FedRAMP Moderate authorization
3. **Tunnel (Edge to Origin)** — cloudflared establishes an encrypted tunnel from Cloudflare edge to the origin server using a modular FIPS crypto backend

### 2. Cryptographic Module Description

The system supports multiple FIPS cryptographic backends, selected based on deployment platform:

#### BoringCrypto Module (Linux — primary)

| Attribute | Value |
|-----------|-------|
| Module Name | BoringCrypto |
| FIPS 140-2 Certificate | #3678, #4407 |
| FIPS 140-3 Certificate | **#4735** |
| Validation Level | Level 1 |
| Platform | Linux amd64/arm64 |
| Build Flag | `GOEXPERIMENT=boringcrypto` |
| CMVP URL (140-2) | https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4407 |
| CMVP URL (140-3) | https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4735 |

The BoringCrypto module is statically linked into the cloudflared binary via Go's `GOEXPERIMENT=boringcrypto` build flag. The validated crypto travels with the binary and does **not** depend on host OS OpenSSL.

#### Go Cryptographic Module (macOS/Windows — cross-platform)

| Attribute | Value |
|-----------|-------|
| Module Name | Go Cryptographic Module |
| FIPS 140-3 Status | CAVP A6650 (CMVP validation pending) |
| Platform | All (Linux, macOS, Windows) |
| Build Flag | `GODEBUG=fips140=on` |
| CGO Required | No |

Go 1.24+ includes a native FIPS 140-3 module that has passed CAVP testing (cert A6650) and is on the CMVP MIP list. This is used for macOS and Windows builds where BoringCrypto is not available.

#### FIPS 140-2 to 140-3 Migration

All FIPS 140-2 certificates move to the CMVP Historical List on **September 21, 2026**. After this date, only FIPS 140-3 validated modules are acceptable for new federal acquisitions.

| Backend | 140-2 Status | 140-3 Status | Migration Path |
|---------|-------------|-------------|----------------|
| BoringCrypto | #3678, #4407 (active until 2026-09-21) | **#4735 (validated)** | Update to 140-3 certified BoringSSL tag |
| Go Native | N/A | CAVP A6650 (CMVP pending) | Automatic when CMVP validates |
| System Crypto | Platform-dependent | Platform-dependent | Update OS/platform modules |

#### Approved Algorithms

| Algorithm | Standard | Usage |
|-----------|----------|-------|
| AES-128-GCM | FIPS 197, SP 800-38D | Data encryption (TLS 1.2/1.3) |
| AES-256-GCM | FIPS 197, SP 800-38D | Data encryption (TLS 1.2/1.3) |
| SHA-256 | FIPS 180-4 | Message digest, certificate verification |
| SHA-384 | FIPS 180-4 | Message digest (TLS 1.2) |
| HMAC-SHA-256 | FIPS 198-1 | Message authentication |
| ECDSA P-256 | FIPS 186-4 | Digital signatures |
| ECDSA P-384 | FIPS 186-4 | Digital signatures |
| RSA-2048+ | FIPS 186-4 | Digital signatures, key exchange |
| ECDH P-256 | SP 800-56A | Key agreement |
| ECDH P-384 | SP 800-56A | Key agreement |

### 3. FIPS Compliance Statement

This build of cloudflared:

- **Uses only FIPS 140-2/3 validated cryptographic modules** for all cryptographic operations
- **Supports modular FIPS backends**: BoringCrypto (Linux, 140-2 + 140-3 validated), Go native (cross-platform, 140-3 CAVP), System Crypto (platform-dependent)
- **Enforces TLS 1.2 as minimum protocol version** per NIST SP 800-52 Rev. 2
- **Restricts cipher suites** to FIPS-approved algorithms only (AES-GCM; ChaCha20-Poly1305 excluded)
- **Performs power-on self-tests** (Known Answer Tests) with backend-specific dispatch to verify cryptographic module integrity at startup
- **Generates build manifests** with cryptographic provenance including FIPS certificate references
- **Verifies QUIC cipher safety**: ensures quic-go packet encryption routes through the validated module (see `docs/quic-go-crypto-audit.md`)
- **Optional binary signature verification**: GPG signature check at startup (configurable)
- **Is built on RHEL UBI 9** (Linux) with system-level FIPS crypto policies, or native runners (macOS/Windows) with `GODEBUG=fips140=on`

### 4. Segment-Level Security Controls

#### Segment 1: Client to Cloudflare Edge

- Device posture enforcement via Cloudflare WARP
- Client certificate authentication (mTLS)
- FIPS-compliant client cryptography
- Endpoint disk encryption verification

#### Segment 2: Cloudflare Edge

- Zero Trust access policies (Cloudflare Access)
- DNS filtering and HTTP inspection (Cloudflare Gateway)
- FIPS-approved cipher suites on edge TLS termination
- DNSSEC for DNS integrity
- Audit logging to SIEM

#### Segment 3: Tunnel (Edge to Origin)

- cloudflared built with FIPS-validated module (BoringCrypto #4407/#4735 on Linux, Go native FIPS on macOS/Windows)
- TLS 1.2+ enforced on tunnel connections
- QUIC packet encryption verified to route through FIPS module (AES-GCM only; ChaCha20 excluded)
- Origin TLS certificate validation
- Non-root process execution
- Build manifest with SBOM and integrity verification
- Optional GPG binary signature verification at startup

### 5. Continuous Monitoring

The compliance dashboard provides real-time visibility into the FIPS compliance status across all three segments. Self-tests can be re-executed at any time to verify ongoing cryptographic module integrity.

### 6. Risk Acceptance

_[To be completed by the Authorizing Official]_

Residual risks and their mitigations:

| Risk | Mitigation | Residual Risk |
|------|-----------|---------------|
| FIPS 140-2 sunset (Sept 21, 2026) | BoringCrypto 140-3 (#4735) already validated; Go native 140-3 on MIP list. Dashboard shows countdown and migration urgency. | Low |
| OS not in FIPS mode | Self-test warns on non-FIPS OS; documented in deployment guide | Medium |
| Origin service not using TLS | Defense-in-depth; tunnel encryption protects in transit | Low |
| Cloudflare edge crypto not independently FIPS-validated | Inherited through FedRAMP Moderate; Tier 2 (Keyless SSL) eliminates key material exposure; Tier 3 (self-hosted proxy) eliminates dependency | Medium (Tier 1), Low (Tier 2/3) |
| QUIC ChaCha20-Poly1305 bypass | Cipher suites restricted to AES-GCM; self-test verifies. ChaCha20 only negotiated if client forces it (FIPS clients always prefer AES-GCM). | Low |
| QUIC retry fixed nonce | Protocol requirement (RFC 9001). Not a security weakness — uses publicly known keys for integrity, not confidentiality. Compatible with `GOEXPERIMENT=boringcrypto` and `GODEBUG=fips140=on`. | Low |
| HKDF algorithm in pure Go | HKDF extract/expand logic not in BoringCrypto, but underlying HMAC/SHA primitives are. Cryptographic strength depends on hash function, not the E/E logic. | Low |

---

_Document prepared for AO review. Template follows SSP format per NIST SP 800-18._

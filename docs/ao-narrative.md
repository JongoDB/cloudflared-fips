# Authorization to Operate (AO) Narrative

## System Security Plan — cloudflared-fips

### 1. System Description

cloudflared-fips is a FIPS 140-2 compliant build of Cloudflare's `cloudflared` tunnel client, designed for deployment in federal and regulated environments requiring validated cryptographic modules.

The system operates across three network segments:

1. **Client to Cloudflare Edge** — End-user devices connect to the Cloudflare global network via WARP client or direct HTTPS, using FIPS-validated TLS
2. **Cloudflare Edge** — Cloudflare's network applies Zero Trust access policies, DNS/HTTP filtering, and DDoS protection using FIPS-validated edge cryptography
3. **Tunnel (Edge to Origin)** — cloudflared establishes an encrypted tunnel from Cloudflare edge to the origin server, using BoringCrypto (FIPS 140-2 cert #4407)

### 2. Cryptographic Module Description

#### BoringCrypto Module

| Attribute | Value |
|-----------|-------|
| Module Name | BoringCrypto |
| FIPS Certificate | #4407 |
| Validation Level | FIPS 140-2 Level 1 |
| Validated Date | 2023-11-01 |
| CMVP URL | https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4407 |

The BoringCrypto module is linked into the cloudflared binary via Go's `GOEXPERIMENT=boringcrypto` build flag, which replaces Go's standard library cryptographic implementations with the FIPS-validated BoringCrypto library.

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

- **Uses only FIPS 140-2 validated cryptographic modules** for all cryptographic operations
- **Enforces TLS 1.2 as minimum protocol version** per NIST SP 800-52 Rev. 2
- **Restricts cipher suites** to FIPS-approved algorithms only
- **Performs power-on self-tests** (Known Answer Tests) to verify cryptographic module integrity at startup
- **Generates build manifests** with cryptographic provenance including FIPS certificate references
- **Is built on RHEL UBI 9** with system-level FIPS crypto policies

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

- cloudflared built with BoringCrypto (FIPS 140-2 cert #4407)
- TLS 1.2+ enforced on tunnel connections
- Origin TLS certificate validation
- Non-root process execution
- Build manifest with SBOM and integrity verification

### 5. Continuous Monitoring

The compliance dashboard provides real-time visibility into the FIPS compliance status across all three segments. Self-tests can be re-executed at any time to verify ongoing cryptographic module integrity.

### 6. Risk Acceptance

_[To be completed by the Authorizing Official]_

Residual risks and their mitigations:

| Risk | Mitigation | Residual Risk |
|------|-----------|---------------|
| BoringCrypto certification expiry | Monitor CMVP status; plan migration to Go native FIPS module when validated | Low |
| OS not in FIPS mode | Self-test warns on non-FIPS OS; documented in deployment guide | Medium |
| Origin service not using TLS | Defense-in-depth; tunnel encryption protects in transit | Low |

---

_Document prepared for AO review. Template follows SSP format per NIST SP 800-18._

# NIST 800-53 Control Mapping

## cloudflared-fips Implementation Mapping

This document maps NIST SP 800-53 Rev. 5 security controls to their implementations in the cloudflared-fips project.

---

### SC-8: Transmission Confidentiality and Integrity

**Control**: Protect the confidentiality and integrity of transmitted information.

| Implementation | Segment | Evidence |
|----------------|---------|----------|
| TLS 1.2+ enforced on all tunnel connections | Tunnel | `cloudflared-fips.yaml` `min-tls-version: "1.2"` |
| FIPS-approved cipher suites only | All segments | Self-test `cipher_suites` check, cipher allow-list in config |
| QUIC/HTTP2 tunnel protocol | Tunnel | Protocol configuration in cloudflared config |
| mTLS client certificates | Client-Edge | Access mTLS policy configuration |
| Origin TLS certificate validation | Tunnel-Origin | `noTLSVerify: false` in ingress rules |

**Self-Test Checks**: `cipher_suites`, `boring_crypto_linked`

---

### SC-13: Cryptographic Protection

**Control**: Implement FIPS-validated cryptography in accordance with applicable laws, regulations, and policies.

| Implementation | Segment | Evidence |
|----------------|---------|----------|
| BoringCrypto (FIPS 140-2 cert #4407) | Tunnel | Binary symbol verification, build manifest |
| `GOEXPERIMENT=boringcrypto` build flag | Build | Dockerfile.fips, Makefile |
| Known Answer Tests at startup | Tunnel | Self-test KAT results (AES-GCM, SHA-256, SHA-384, HMAC, ECDSA, RSA) |
| FIPS-only cipher suite enforcement | All segments | Cipher allow-list, self-test verification |
| RHEL UBI 9 FIPS-validated OpenSSL | Build | Dockerfile.fips base image |

**Self-Test Checks**: `boring_crypto_linked`, `kat_AES-128-GCM`, `kat_AES-256-GCM`, `kat_SHA-256`, `kat_SHA-384`, `kat_HMAC-SHA-256`, `kat_ECDSA-P256`, `kat_RSA-2048`

---

### SC-12: Cryptographic Key Establishment and Management

**Control**: Establish and manage cryptographic keys when cryptography is employed within the system.

| Implementation | Segment | Evidence |
|----------------|---------|----------|
| Tunnel credentials stored with mode 600 | Tunnel | Deployment documentation, self-test check |
| ECDH P-256/P-384 for TLS key exchange | Tunnel | Curve preference configuration |
| Certificate-based authentication | Client-Edge | Access mTLS configuration |
| Automatic certificate rotation (Cloudflare managed) | Edge | Cloudflare dashboard configuration |

**Dashboard Section**: Tunnel items `tn-7` (Tunnel Credentials Secured)

---

### IA-7: Cryptographic Module Authentication

**Control**: Implement mechanisms for authentication to a cryptographic module that meet FIPS requirements.

| Implementation | Segment | Evidence |
|----------------|---------|----------|
| BoringCrypto FIPS 140-2 Level 1 | Tunnel | CMVP certificate #4407 |
| Power-on self-tests | Tunnel | KAT execution at binary startup |
| Build manifest with FIPS certificate references | Build | `build-manifest.json` `fips_certificates` array |

**Self-Test Checks**: All KAT checks, `boring_crypto_linked`

---

### SA-11: Developer Testing and Evaluation

**Control**: Require the developer to create and implement a plan for security assessment.

| Implementation | Segment | Evidence |
|----------------|---------|----------|
| Automated self-test suite | Tunnel | `cmd/selftest`, `internal/selftest/` |
| CI pipeline with lint, test, and compliance checks | Build | `.github/workflows/compliance-check.yml` |
| BoringCrypto symbol verification | Build | `scripts/verify-boring.sh` |
| SBOM generation | Build | SPDX SBOM in build pipeline |
| Dependency vulnerability scanning | Build | `govulncheck` in CI |
| Container image scanning | Build | Trivy/Grype in CI pipeline |

**CI Jobs**: `go-lint-test`, `dashboard-lint-build`, `manifest-validation`, `shell-syntax`

---

### CM-14: Signed Components

**Control**: Prevent the installation of software and firmware components without verification that the component has been digitally signed.

| Implementation | Segment | Evidence |
|----------------|---------|----------|
| Build manifest with binary SHA-256 hash | Build | `build-manifest.json` `binary.sha256` |
| SBOM with integrity hash | Build | `build-manifest.json` `sbom.sha256` |
| Artifact signing (cosign integration planned) | Build | CI pipeline signing step |
| Reproducible build pipeline | Build | Pinned versions in Dockerfile.fips |

**Dashboard Section**: Build & Supply Chain items `bs-1` through `bs-6`

---

## Controls Summary

| Control | Status | Coverage |
|---------|--------|----------|
| SC-8 | Implemented | Full — all three segments |
| SC-13 | Implemented | Full — validated crypto, self-tests, cipher enforcement |
| SC-12 | Implemented | Partial — application-level key management; Cloudflare-managed keys at edge |
| IA-7 | Implemented | Full — FIPS 140-2 Level 1 validated module |
| SA-11 | Implemented | Full — automated testing, CI/CD, vulnerability scanning |
| CM-14 | Partially Implemented | Binary hashing complete; artifact signing pending |

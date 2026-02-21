# Cryptographic Module Usage Document

## cloudflared-fips — Validated Module Mapping

This document maps every cryptographic operation in cloudflared-fips to its
validated module and CMVP certificate number.

---

### Key Statement

**No cryptographic algorithms are implemented in the application layer.**
All cryptographic operations are delegated to FIPS 140-2/3 validated modules.
The active module is selected automatically based on platform and build configuration.

---

## Operation → Algorithm → Validated Module → Certificate

| Operation | Algorithm | Validated Module | CMVP Certificate |
|-----------|-----------|-----------------|-------------------|
| TLS 1.2 key exchange | ECDH P-256 | BoringCrypto | #3678 |
| TLS 1.2 key exchange | ECDH P-384 | BoringCrypto | #3678 |
| TLS 1.3 key exchange | ECDH P-256 | BoringCrypto | #3678 |
| TLS record encryption | AES-128-GCM | BoringCrypto | #3678 |
| TLS record encryption | AES-256-GCM | BoringCrypto | #3678 |
| TLS handshake signature | ECDSA P-256 | BoringCrypto | #3678 |
| TLS handshake signature | ECDSA P-384 | BoringCrypto | #3678 |
| TLS handshake signature | RSA-2048 | BoringCrypto | #3678 |
| TLS handshake signature | RSA-4096 | BoringCrypto | #3678 |
| TLS PRF (1.2) | HMAC-SHA-256 | BoringCrypto | #3678 |
| TLS PRF (1.2) | HMAC-SHA-384 | BoringCrypto | #3678 |
| TLS HKDF (1.3) | HKDF-SHA-256 | BoringCrypto | #3678 |
| TLS HKDF (1.3) | HKDF-SHA-384 | BoringCrypto | #3678 |
| Certificate verification | SHA-256 | BoringCrypto | #3678 |
| Certificate verification | SHA-384 | BoringCrypto | #3678 |
| QUIC packet protection | AES-128-GCM | BoringCrypto | #3678 |
| QUIC header protection | AES-128-ECB | BoringCrypto | #3678 |
| Self-test KAT | AES-128-GCM | BoringCrypto | #3678 |
| Self-test KAT | AES-256-GCM | BoringCrypto | #3678 |
| Self-test KAT | SHA-256 | BoringCrypto | #3678 |
| Self-test KAT | SHA-384 | BoringCrypto | #3678 |
| Self-test KAT | HMAC-SHA-256 | BoringCrypto | #3678 |
| Self-test KAT | ECDSA P-256 | BoringCrypto | #3678 |
| Self-test KAT | RSA-2048 | BoringCrypto | #3678 |
| OS-level TLS (build host) | AES-256-GCM | RHEL OpenSSL | #4349 |
| OS-level TLS (build host) | SHA-256 | RHEL OpenSSL | #4349 |

---

## Segment Coverage

### Segment 1: Client → Cloudflare Edge

| Client OS | Crypto Module | Certificate | Notes |
|-----------|--------------|-------------|-------|
| Windows (FIPS GPO) | Windows CNG | #2357+ | GPO enforced |
| RHEL (fips=1) | OpenSSL FIPS | #3842, #4349 | Kernel parameter |
| macOS | CommonCrypto | #3856+ | Always active |

**Honest assessment:** This segment is NOT controlled by cloudflared. FIPS compliance
is enforced at the client OS level via Group Policy (Windows), kernel parameter (RHEL),
or is always active (macOS). Chrome on Linux uses BoringSSL, not the OS OpenSSL — this
is a documented limitation.

### Segment 2: Cloudflare Edge → cloudflared (Tunnel)

| Component | Crypto Module | Certificate |
|-----------|--------------|-------------|
| cloudflared binary | BoringCrypto | #3678 |
| quic-go TLS | BoringCrypto (via GOEXPERIMENT) | #3678 |
| Go crypto/tls | BoringCrypto (via GOEXPERIMENT) | #3678 |

**This is the segment this product directly controls.** All crypto operations route
through BoringCrypto via `GOEXPERIMENT=boringcrypto`.

### Segment 3: cloudflared → Local Service

| Scenario | Crypto Module | Certificate |
|----------|--------------|-------------|
| Loopback (localhost) | N/A | Out of scope (AO decision) |
| Networked + TLS | BoringCrypto | #3678 |
| Networked + no TLS | None | Non-compliant |

**Honest assessment:** Loopback connections may be out of scope for FIPS depending on
AO interpretation. Networked connections without TLS are non-compliant. Both scenarios
are documented for AO decision.

---

## quic-go Crypto Audit

Full audit: `docs/quic-go-crypto-audit.md`

The `quic-go` library used by cloudflared for QUIC tunnel connections uses Go's
`crypto/tls.QUICConn` API (since v0.37). When built with `GOEXPERIMENT=boringcrypto`,
all `crypto/*` packages are replaced by BoringCrypto implementations.

| Operation | Routes through FIPS module? | Notes |
|-----------|---------------------------|-------|
| TLS 1.3 handshake | **Yes** (via `crypto/tls.QUICConn`) | RSA/ECDSA/ECDH via BoringCrypto |
| AES-GCM packet encryption | **Yes** (`crypto/aes` + `crypto/cipher`) | Both 128 and 256 |
| AES header protection | **Yes** (`crypto/aes`) | ECB block encrypt |
| ChaCha20-Poly1305 encryption | **No** (`golang.org/x/crypto`) | Mitigated: cipher suites restricted to AES-GCM |
| ChaCha20 header protection | **No** (`golang.org/x/crypto`) | Same mitigation |
| HKDF extract/expand | **Partial** | Algorithm in pure Go; underlying HMAC/SHA via BoringCrypto |
| QUIC retry integrity tag | **Protocol conflict** | Fixed nonce per RFC 9001; incompatible with `fips140=only` but OK with `boringcrypto` and `fips140=on` |

### Known Deviations

1. **HKDF partial bypass (low risk):** The HKDF algorithm logic (`golang.org/x/crypto/hkdf`) does not route through BoringCrypto, but its underlying HMAC and SHA primitives do. The cryptographic strength depends on the hash function, not the extract/expand logic. Go issue [#47234](https://github.com/golang/go/issues/47234) proposes adding HKDF to BoringCrypto.

2. **QUIC retry fixed nonce (protocol-level):** RFC 9001 Section 5.8 requires QUIC retry integrity tags to use hard-coded, fixed AES-GCM nonces. This violates FIPS 140-3 nonce requirements but is not a security weakness — the retry keys are publicly known (published in the RFC) and the tag provides integrity, not confidentiality. `GOEXPERIMENT=boringcrypto` and `GODEBUG=fips140=on` do not enforce this restriction; only `GODEBUG=fips140=only` does (which panics on QUIC retry).

---

## FIPS Backend Selection

| Platform | Backend | Build Flag | FIPS Standard | Certificate |
|----------|---------|-----------|---------------|-------------|
| Linux amd64/arm64 | BoringCrypto | `GOEXPERIMENT=boringcrypto` | 140-2 / 140-3 | #4407 / #4735 |
| macOS (all arch) | Go Native | `GODEBUG=fips140=on` | 140-3 (pending) | CAVP A6650 |
| Windows (all arch) | Go Native | `GODEBUG=fips140=on` | 140-3 (pending) | CAVP A6650 |
| Windows (MS Go) | System Crypto | `GOEXPERIMENT=systemcrypto` | 140-2/3 | CNG (platform) |

---

## Key Rotation

See `docs/key-rotation-procedure.md` for the artifact signing key rotation procedure.

---

## References

- CMVP Certificate #3678: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/3678
- CMVP Certificate #4407: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4407
- CMVP Certificate #4735: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4735
- CMVP Certificate #4349: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4349
- CAVP A6650: https://csrc.nist.gov/projects/cryptographic-algorithm-validation-program/details?validation=A6650
- NIST SP 800-52 Rev. 2: Guidelines for TLS Implementations
- NIST SP 800-57: Key Management
- NIST SP 800-53 Rev. 5: Security and Privacy Controls
- quic-go FIPS issues: https://github.com/quic-go/quic-go/issues/4894, https://github.com/quic-go/quic-go/issues/4958

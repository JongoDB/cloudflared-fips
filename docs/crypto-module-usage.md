# Cryptographic Module Usage Document

## cloudflared-fips — Validated Module Mapping

This document maps every cryptographic operation in cloudflared-fips to its
validated module and CMVP certificate number.

---

### Key Statement

**No cryptographic algorithms are implemented in the application layer.**
All cryptographic operations are delegated to FIPS 140-2 validated modules.

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

The `quic-go` library used by cloudflared for QUIC tunnel connections uses Go's
`crypto/tls` package for its TLS 1.3 handshake. When built with
`GOEXPERIMENT=boringcrypto`, all `crypto/*` packages are replaced by BoringCrypto
implementations. Verified:

- `quic-go` does not implement its own TLS stack
- `quic-go` delegates to `crypto/tls` for handshake and key derivation
- `quic-go` uses `crypto/aes` for packet protection (replaced by BoringCrypto)
- No independent crypto implementations that bypass the validated module

---

## References

- CMVP Certificate #3678: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/3678
- CMVP Certificate #4349: https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4349
- NIST SP 800-52 Rev. 2: Guidelines for TLS Implementations
- NIST SP 800-57: Key Management
- NIST SP 800-53 Rev. 5: Security and Privacy Controls

# quic-go Cryptographic Audit for FIPS Compliance

**Date:** 2026-02-21
**Scope:** Does quic-go route TLS/crypto operations through BoringCrypto (`GOEXPERIMENT=boringcrypto`)?
**Library:** `github.com/quic-go/quic-go` (current, requires Go 1.25+)
**Verdict:** Mostly yes, with two known gaps and one protocol-level FIPS incompatibility.

---

## Summary

| Category | Routes through BoringCrypto? | Risk |
|----------|------------------------------|------|
| TLS 1.3 handshake | Yes (via `crypto/tls.QUICConn`) | None |
| AES-GCM packet encryption | Yes (`crypto/aes` + `crypto/cipher`) | None |
| AES header protection | Yes (`crypto/aes`) | None |
| SHA-256/384 hashing | Yes (`crypto` hash interface) | None |
| HMAC (within HKDF) | Yes (via hash primitives) | None |
| Random number generation | Yes (`crypto/rand`) | None |
| Certificate operations | Yes (via `crypto/tls`) | None |
| **ChaCha20-Poly1305 encryption** | **No** (`golang.org/x/crypto`) | **Mitigatable** |
| **ChaCha20 header protection** | **No** (`golang.org/x/crypto`) | **Mitigatable** |
| HKDF extract/expand logic | Partial (algorithm in pure Go, primitives via BoringCrypto) | Low |
| **QUIC retry integrity tag** | **Protocol conflict** (fixed nonce violates FIPS 140-3) | **Cannot mitigate** |

---

## 1. TLS Handshake: Uses `crypto/tls.QUICConn` (safe)

Since v0.37 (Go 1.21+), quic-go uses Go's standard `crypto/tls.QUICConn` API. The relevant code is in `internal/handshake/crypto_setup.go`:

- Creates connections via `tls.QUICClient(&tls.QUICConfig{...})` and `tls.QUICServer(&tls.QUICConfig{...})`
- Processes `tls.QUICEvent` objects for key exchange, certificate verification, and handshake completion
- Enforces `MinVersion = tls.VersionTLS13` in `tls_config.go`

The old `qtls-go1-XX` forks are no longer used. The TLS 1.3 state machine runs entirely inside `crypto/tls`, meaning RSA/ECDSA signatures, ECDH key exchange, and certificate validation all route through BoringCrypto.

## 2. Packet Encryption: AES-GCM is safe, ChaCha20 is not

quic-go creates AEAD ciphers in `internal/handshake/cipher_suite.go`:

| Cipher Suite | Implementation | BoringCrypto? |
|---|---|---|
| `TLS_AES_128_GCM_SHA256` | `crypto/aes.NewCipher()` + `crypto/cipher.NewGCM()` | **Yes** |
| `TLS_AES_256_GCM_SHA384` | `crypto/aes.NewCipher()` + `crypto/cipher.NewGCM()` | **Yes** |
| `TLS_CHACHA20_POLY1305_SHA256` | `golang.org/x/crypto/chacha20poly1305.New()` | **No** |

When ChaCha20-Poly1305 is negotiated, packet encryption uses a pure Go implementation from `golang.org/x/crypto` that does not route through BoringCrypto. ChaCha20-Poly1305 is not a FIPS-approved algorithm.

**Mitigation:** Restrict cipher suites to AES-GCM only in your `tls.Config`:
```go
tlsConfig := &tls.Config{
    CipherSuites: []uint16{
        tls.TLS_AES_128_GCM_SHA256,
        tls.TLS_AES_256_GCM_SHA384,
    },
}
```

Note: cloudflared already restricts to FIPS-approved cipher suites when using our FIPS build, so ChaCha20-Poly1305 should never be negotiated. The self-test validates this.

## 3. Header Protection: Same split as packet encryption

In `internal/handshake/header_protector.go`:

| Cipher Suite | Header Protection | BoringCrypto? |
|---|---|---|
| AES-GCM | `crypto/aes.NewCipher()` ECB block encrypt | **Yes** |
| ChaCha20-Poly1305 | `golang.org/x/crypto/chacha20.NewUnauthenticatedCipher()` | **No** |

Same mitigation: restrict to AES-GCM cipher suites.

## 4. Key Derivation (HKDF): Partial bypass, low risk

quic-go uses `golang.org/x/crypto/hkdf` in several files:
- `internal/handshake/hkdf.go`
- `internal/handshake/initial_aead.go`
- `internal/handshake/token_protector.go`

The HKDF extract-and-expand algorithm logic is pure Go (not BoringCrypto). However, HKDF internally calls HMAC, and HMAC calls the hash function (SHA-256 or SHA-384). When BoringCrypto is enabled, these underlying hash and HMAC operations **do** route through BoringCrypto.

**Risk assessment:** Low. The cryptographic strength comes from the hash/HMAC primitives, which are BoringCrypto-backed. The HKDF logic itself is just extract/expand — it doesn't introduce new cryptographic primitives.

**Note:** Go issue [#47234](https://github.com/golang/go/issues/47234) proposes adding HKDF to BoringCrypto. Go 1.24+ added `crypto/hkdf` to the standard library, which may get BoringCrypto backing in future versions.

## 5. Initial Packet Protection: Safe (AES-GCM via BoringCrypto)

Per RFC 9001, initial QUIC packets use keys derived from the connection ID (before TLS completes). In `internal/handshake/initial_aead.go`:

1. HKDF-Extract: `hkdf.Extract(crypto.SHA256.New, connID, salt)` — hash via BoringCrypto
2. HKDF-Expand-Label: custom function using `golang.org/x/crypto/hkdf` — hash via BoringCrypto
3. AES-128-GCM cipher: `crypto/aes` + `crypto/cipher` — **routes through BoringCrypto**

## 6. QUIC Retry: Protocol-level FIPS incompatibility

**This is the most important finding.**

RFC 9001 Section 5.8 requires QUIC retry integrity tags to use **hard-coded, fixed AES-GCM nonces**. In `internal/handshake/retry.go`, quic-go initializes `cipher.NewGCM` with these fixed keys/nonces per the RFC.

FIPS 140-3 prohibits `cipher.NewGCM` with fixed/arbitrary IVs. Go 1.24's `GODEBUG=fips140=only` mode **panics** when this code path is hit.

**Current state:**
- [PR #4916](https://github.com/quic-go/quic-go/pull/4916) made retry AEAD initialization lazy (prevents import-time panic)
- But actual retry operations still cannot be performed in `fips140=only` mode
- `GOEXPERIMENT=boringcrypto` does **not** enforce this restriction (only `GODEBUG=fips140=only` does), so our Linux builds are unaffected
- `GODEBUG=fips140=on` (used for macOS/Windows) also does not enforce this restriction

**Risk assessment:**
- **For `GOEXPERIMENT=boringcrypto` (Linux):** No runtime impact. BoringCrypto does not reject fixed nonces.
- **For `GODEBUG=fips140=only` (strict mode):** Incompatible. QUIC retry will panic.
- **For `GODEBUG=fips140=on` (our macOS/Windows builds):** No runtime impact. This mode does not enforce nonce restrictions.

**AO documentation note:** The retry integrity tag uses a fixed nonce per the QUIC RFC. This is a deviation from FIPS 140-3 nonce requirements but is not a security weakness — it's a deterministic integrity check with publicly known keys, not an encryption operation protecting confidentiality.

## 7. Token Protection: Safe

In `internal/handshake/token_protector.go`:
- SHA-256 hashing: `crypto/sha256` — via BoringCrypto
- HKDF key derivation: `golang.org/x/crypto/hkdf` — hash primitives via BoringCrypto
- AES-256-GCM encryption: `crypto/aes` + `crypto/cipher` — via BoringCrypto
- Random nonce: `crypto/rand` — via BoringCrypto

---

## Recommendations

### 1. Restrict cipher suites to AES-GCM only (critical)

Ensure cloudflared-fips `tls.Config` excludes `TLS_CHACHA20_POLY1305_SHA256`:

```go
CipherSuites: []uint16{
    tls.TLS_AES_128_GCM_SHA256,
    tls.TLS_AES_256_GCM_SHA384,
}
```

This ensures all packet encryption and header protection route through BoringCrypto. Our self-test already validates FIPS-only cipher suites.

### 2. Add quic-go cipher restriction check to self-test

Add a runtime check verifying that the QUIC transport's TLS config does not include ChaCha20-Poly1305. This should be part of the startup self-test.

### 3. Document the HKDF partial bypass

HKDF algorithm logic from `golang.org/x/crypto/hkdf` does not route through BoringCrypto, but its underlying HMAC/SHA primitives do. This should be documented in the AO package as an acceptable deviation — the cryptographic strength depends on the hash function, not the extract/expand logic.

### 4. Document the retry nonce deviation

The QUIC retry integrity tag's fixed nonce is a protocol requirement (RFC 9001) and cannot be changed. Document this in the AO package as a non-security-impacting deviation from FIPS 140-3 nonce guidance:
- The retry keys are publicly known (published in the RFC)
- The retry tag provides integrity, not confidentiality
- An attacker who knows the keys can forge retry packets regardless of nonce policy

### 5. Do NOT use `GODEBUG=fips140=only` with QUIC

Strict FIPS-only mode is incompatible with the QUIC protocol. Use `GOEXPERIMENT=boringcrypto` (Linux) or `GODEBUG=fips140=on` (macOS/Windows) instead.

---

## References

- [quic-go #4894: FIPS Compliance Issues with Go 1.24](https://github.com/quic-go/quic-go/issues/4894)
- [quic-go #4958: FIPS 140 Compatibility](https://github.com/quic-go/quic-go/issues/4958)
- [quic-go and Go versions wiki](https://github.com/quic-go/quic-go/wiki/quic-go-and-Go-versions)
- [Go #47234: Use BoringCrypto for HKDF](https://github.com/golang/go/issues/47234)
- [Go FIPS 140-3 documentation](https://go.dev/doc/security/fips140)
- [RFC 9001: Using TLS to Secure QUIC](https://www.rfc-editor.org/rfc/rfc9001)
- [RFC 9001 Section 5.8: Retry Integrity](https://www.rfc-editor.org/rfc/rfc9001#section-5.8)

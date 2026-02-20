# FIPS Compliance Justification Letter

## Template for Authorizing Official

---

**To:** [Authorizing Official Name and Title]

**From:** [System Owner / ISSO Name]

**Date:** [Date]

**Subject:** FIPS 140-2 Compliance Justification for cloudflared-fips Deployment

---

### 1. Purpose

This letter provides the formal justification for deploying cloudflared-fips as
a FIPS 140-2 compliant component within [System Name]. cloudflared-fips uses a
leveraged validated module approach — it does not implement any cryptographic
algorithms itself, but exclusively delegates all cryptographic operations to
FIPS 140-2 validated modules.

### 2. Approach: Leveraged Validated Modules

cloudflared-fips uses the following validated cryptographic modules:

| Module | CMVP Certificate | Validation Level | Status |
|--------|-----------------|------------------|--------|
| BoringCrypto (BoringSSL) | #3678 | FIPS 140-2 Level 1 | Active |
| RHEL OpenSSL FIPS Provider | #4349 | FIPS 140-2 Level 1 | Active |

This approach is consistent with:

- **NIST FIPS 140-2 Implementation Guidance** (Section 8): Applications that use
  validated cryptographic modules without implementing their own cryptographic
  algorithms are considered compliant when they correctly invoke the validated module.

- **NIST SP 800-175B** (Section 3): "The use of a validated cryptographic module
  satisfies the FIPS 140 validation requirement for the system."

- **Industry precedent**: RHEL, Windows, most enterprise software (including
  commercial Cloudflare products) use this same leveraged module approach.
  No application-level CMVP validation is performed; compliance is achieved
  through correct use of validated modules.

### 3. Cryptographic Module Integration

cloudflared-fips is compiled with Go's `GOEXPERIMENT=boringcrypto` build flag,
which replaces all of Go's standard library cryptographic implementations
(`crypto/*` packages) with calls to the BoringCrypto module. This is not a
wrapper or shim — the Go compiler directly links against the validated
BoringCrypto C library.

**Verification mechanisms:**

1. Binary symbol inspection confirms `_goboringcrypto_` symbols are present
2. Known Answer Tests (KAT) execute at startup to verify module integrity
3. Runtime cipher suite enumeration confirms only FIPS-approved suites
4. Build manifest records the exact BoringSSL version and certificate number

### 4. Three-Segment Analysis

#### Segment 1: Client → Cloudflare Edge

**Honest assessment:** This segment is NOT controlled by cloudflared-fips.
FIPS compliance is enforced at the client OS level:

- Windows: GPO "System cryptography: Use FIPS compliant algorithms" (CNG cert #2357+)
- RHEL: `fips=1` kernel boot parameter (OpenSSL cert #3842/#4349)
- macOS: CommonCrypto always active (cert #3856+)

**Mitigation:** Cloudflare Access device posture integration with MDM (Intune, Jamf)
enforces client FIPS compliance as a precondition for tunnel access. Non-compliant
devices are denied before reaching the tunnel.

**Known limitation:** Chrome on Linux uses BoringSSL, not the OS OpenSSL library.
This is documented in the Client Endpoint Hardening Guide.

#### Segment 2: Cloudflare Edge → cloudflared (Tunnel)

**This is the segment cloudflared-fips directly controls.** All cryptographic
operations (TLS handshake, QUIC packet protection, key derivation) use
BoringCrypto (CMVP #3678).

#### Segment 3: cloudflared → Local Service

**AO decision required:** If the local service connection is loopback (localhost),
it may be out of scope for FIPS. If it traverses a network segment, it must use
TLS with FIPS-approved ciphers. Both configurations are supported and documented.

### 5. Continuous Monitoring

The compliance dashboard provides real-time visibility into FIPS compliance
status across all three segments. The dashboard:

- Runs power-on self-tests at startup
- Monitors OS FIPS mode status
- Verifies binary integrity against known-good hashes
- Tracks certificate validity and cipher suite negotiation
- Exports compliance reports in JSON and PDF for audit

### 6. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| BoringCrypto CMVP certificate expiry | Low | High | Monitor CMVP status; plan migration path |
| Client OS not in FIPS mode | Medium | Medium | Enforce via MDM + Access device posture |
| Chrome on Linux BoringSSL gap | Medium | Low | Document limitation; recommend Firefox with NSS FIPS |
| quic-go crypto bypass | Low | High | Audited — uses crypto/tls, replaced by BoringCrypto |
| Origin service unencrypted | Medium | Medium | Document loopback vs networked; AO scope decision |

### 7. Conclusion

cloudflared-fips meets FIPS 140-2 requirements for Segment 2 (tunnel) through
exclusive use of the BoringCrypto validated module. Client-side compliance
(Segment 1) is enforced through OS policy and Cloudflare Access device posture.
Origin-side compliance (Segment 3) is configurable and documented for AO review.

This approach is consistent with NIST guidance and industry precedent for
leveraged validated module deployments.

---

**Recommended AO Action:** [Accept / Accept with Conditions / Deny]

**Conditions (if applicable):**
- [ ] _[List any conditions for acceptance]_

---

_This template follows NIST FIPS 140-2 Implementation Guidance and SP 800-37
(Risk Management Framework) authorization process._

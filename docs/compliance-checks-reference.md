# Compliance Checks Reference -- cloudflared-fips

Version: 0.6.0 | Last updated: 2026-03-03

## Purpose

This document is a complete reference for every compliance check performed by the cloudflared-fips compliance dashboard. It is intended for Authorizing Officials (AOs), security assessors, and compliance engineers evaluating cloudflared-fips for authorization to operate (ATO) within a federal information system boundary.

Each check maps to one or more NIST SP 800-53 Rev 5 controls. Checks run automatically via the dashboard's continuous monitoring engine (SSE push every 30 seconds) and produce one of four statuses:

| Status | Meaning |
|--------|---------|
| **Pass** | The system meets the requirement. |
| **Warning** | The system partially meets the requirement or data is incomplete. |
| **Fail** | The system does not meet the requirement. Remediation required. |
| **Unknown** | The check could not determine compliance (missing configuration, unreachable service). |

Verification methods indicate *how* each check obtains its data:

| Method | Description |
|--------|-------------|
| **Direct** | Measured locally via system calls, file reads, or binary introspection. |
| **API** | Queried from the Cloudflare API using a bearer token. |
| **Probe** | Determined via active TLS handshake or network connection test. |
| **Inherited** | Relies on the provider's FedRAMP authorization; not independently verifiable. |
| **Reported** | Client-reported via WARP device posture agent or MDM integration. |

---

## Summary

| # | Section | Check Count | Critical | High | Medium |
|---|---------|-------------|----------|------|--------|
| 1 | Client Posture | 8 | 3 | 2 | 3 |
| 2 | Cloudflare Edge | 11 | 5 | 5 | 1 |
| 3 | Tunnel -- cloudflared | 12 | 6 | 3 | 3 |
| 4 | Local Service | 4 | 0 | 3 | 1 |
| 5 | Build and Supply Chain | 7 | 2 | 2 | 3 |
| 6 | Security Operations | 13 | 3 | 7 | 3 |
| | **Total** | **55** | **19** | **22** | **14** |

---

## 1. Client Posture (8 checks)

Segment 1: Client OS/Browser to Cloudflare Edge (HTTPS). These checks assess whether connecting client devices use FIPS-validated cryptography. The cloudflared-fips product does not control client endpoints but can observe and report their posture.

### cp-1: Client OS FIPS Mode

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, CM-6 |
| **Verification** | Reported |
| **Code** | `pkg/clientdetect/checker.go:checkClientOSFIPS` |

**What it checks:** Queries device posture reports from WARP agents to determine if connecting client devices have OS-level FIPS mode enabled.

**Criteria:**

- **Pass:** All reporting devices have FIPS mode enabled.
- **Warning:** Some devices report FIPS disabled, or no device posture reports are available.
- **Fail:** N/A (degrades to Warning due to reliance on client-reported data).

**Remediation:** Enable FIPS mode on client operating systems. Deploy the FIPS policy via MDM (Intune GPO for Windows, `fips=1` boot parameter for RHEL/Linux). See `docs/client-hardening-guide.md`.

---

### cp-2: Client OS Type and Version

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | CM-8, SI-2 |
| **Verification** | Reported |
| **Code** | `pkg/clientdetect/checker.go:checkClientOSType` |

**What it checks:** Reports the operating system types and versions of connecting client devices from device posture data.

**Criteria:**

- **Pass:** One or more devices are reporting OS information.
- **Warning:** No device posture reports available.

**Remediation:** Deploy the WARP agent on all managed endpoints to collect OS information. Ensure endpoints run FIPS-capable OS versions (see CLAUDE.md Host OS FIPS Validation Matrix).

---

### cp-3: Browser TLS Capabilities

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-13, SC-8 |
| **Verification** | Probe |
| **Code** | `pkg/clientdetect/checker.go:checkBrowserTLS` |

**What it checks:** Analyzes TLS ClientHello messages from connecting clients to determine whether they offer only FIPS-approved cipher suites. The presence of ChaCha20-Poly1305, RC4, or DES/3DES in the ClientHello indicates the client is not in FIPS mode.

**Criteria:**

- **Pass:** All inspected clients offer only FIPS-approved ciphers.
- **Warning:** Some clients offer non-FIPS ciphers, or no clients have been inspected yet. Inspection requires the Tier 3 FIPS proxy or WARP agent.

**Remediation:** Enable FIPS mode on client browsers. On Windows, enable the FIPS GPO ("System cryptography: Use FIPS compliant algorithms"). On Linux, enable OS FIPS mode (`fips=1`). Note: Chrome on Linux uses BoringSSL regardless of OS FIPS mode -- recommend Firefox with NSS FIPS mode.

---

### cp-4: Negotiated Cipher Suite (Client to Edge)

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, SC-8 |
| **Verification** | Probe |
| **Code** | `pkg/clientdetect/checker.go:checkNegotiatedCipher` |

**What it checks:** Reports the cipher suites offered by recently connected clients based on TLS ClientHello inspection. Determines whether all recent connections used FIPS-approved cipher negotiation.

**Criteria:**

- **Pass:** All recent connections from FIPS-capable clients.
- **Warning:** Some recent connections from non-FIPS-capable clients, or no recent connections inspected.

**Remediation:** Restrict edge cipher suites via the Cloudflare API to force FIPS-approved negotiation. Deploy the Tier 3 FIPS proxy for full ClientHello inspection.

---

### cp-5: TLS Version (Client to Edge)

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-8, SC-13 |
| **Verification** | Probe |
| **Code** | `pkg/clientdetect/checker.go:checkTLSVersion` |

**What it checks:** Verifies that the Cloudflare edge enforces a minimum TLS version. Clients below the configured minimum are rejected at the edge before reaching the tunnel.

**Criteria:**

- **Pass:** Edge minimum TLS version is set to 1.2 or higher.

**Remediation:** Set the minimum TLS version to 1.2 in Cloudflare zone settings (SSL/TLS > Edge Certificates > Minimum TLS Version). TLS 1.0 and 1.1 are prohibited under NIST SP 800-52 Rev 2.

---

### cp-6: Cloudflare Access Device Posture

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | AC-17, CM-8 |
| **Verification** | Reported |
| **Code** | `pkg/clientdetect/checker.go:checkDevicePosture` |

**What it checks:** Checks whether Cloudflare Access device posture checks are receiving reports from managed endpoints.

**Criteria:**

- **Pass:** One or more devices are reporting posture data.
- **Warning:** No posture reports available.

**Remediation:** Configure device posture checks in the Cloudflare Zero Trust dashboard (Devices > Device Posture). Deploy the WARP agent on all managed endpoints.

---

### cp-7: MDM Enrollment Verified

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | CM-8, CM-2 |
| **Verification** | Reported |
| **Code** | `pkg/clientdetect/checker.go:checkMDMEnrolled` |

**What it checks:** Checks whether connecting devices are enrolled in a Mobile Device Management (MDM) solution such as Microsoft Intune or Jamf Pro.

**Criteria:**

- **Pass:** All reporting devices are MDM-enrolled.
- **Warning:** Some devices are not MDM-enrolled, or no device reports available.

**Remediation:** Enroll all devices in your MDM solution. Add an MDM enrollment check to the Cloudflare Access device posture policy. Configure MDM integration using `--mdm-provider` (intune or jamf).

---

### cp-8: Client Certificate (mTLS)

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | IA-2, IA-5 |
| **Verification** | Probe |
| **Code** | `pkg/clientdetect/checker.go:checkClientCertificate` |

**What it checks:** Checks whether mutual TLS (mTLS) is configured for strong device identity via client certificates.

**Criteria:**

- **Warning:** mTLS not configured. This is an optional enhancement for zero-trust access.
- **Pass:** mTLS configured with a trusted CA.

**Remediation:** Configure mTLS in Cloudflare Access with a trusted CA. Issue client certificates to managed devices.

---

## 2. Cloudflare Edge (11 checks)

These checks verify the configuration of Cloudflare's edge infrastructure. Items are verified via the Cloudflare API or rely on Cloudflare's FedRAMP Moderate authorization (inherited). Cloudflare does not hold its own CMVP certificate; edge crypto module FIPS validation is inherited through FedRAMP, not independently verifiable.

### ce-1: Cloudflare Access Policy

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | AC-3, AC-17 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkAccessPolicy` |

**What it checks:** Queries the Cloudflare Access API (`/zones/{zone_id}/access/apps`) to verify at least one Access application is configured for the zone.

**Criteria:**

- **Pass:** One or more Access applications are configured.
- **Fail:** No Access applications found.
- **Unknown:** API query failed.

**Remediation:** Configure a Cloudflare Access application at dash.cloudflare.com > Zero Trust > Access > Applications. Access policies enforce authentication before requests reach the tunnel.

---

### ce-2: Identity Provider Connected

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | IA-2, IA-8 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkIdentityProvider` |

**What it checks:** Infers whether an identity provider (IdP) is connected to Cloudflare Access. Access applications require an IdP, so the presence of configured applications indicates an IdP is connected.

**Criteria:**

- **Pass:** Access applications exist (IdP must be configured).
- **Unknown:** No Access applications configured; cannot determine IdP status.

**Remediation:** Configure an identity provider (Okta, Azure AD, Google Workspace, etc.) in Zero Trust > Settings > Authentication.

---

### ce-3: Authentication Method

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | IA-2, IA-8 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkAuthMethod` |

**What it checks:** Reports the authentication protocol (SAML 2.0, OIDC, or other) used by the configured identity provider. The specific method is determined by the IdP configuration.

**Criteria:**

- **Unknown:** Authentication method is determined by IdP configuration and is not directly queryable via Cloudflare API.

**Remediation:** Configure SAML 2.0 or OpenID Connect (OIDC) in the Access IdP settings.

---

### ce-4: MFA Enforced

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | IA-2(1), IA-2(2) |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkMFAEnforced` |

**What it checks:** Checks whether multi-factor authentication is enforced. MFA enforcement is configured at the IdP level, not directly queryable via the Cloudflare API.

**Criteria:**

- **Unknown:** MFA enforcement must be verified at the IdP (Okta, Azure AD) and via Access policy rules requiring MFA.

**Remediation:** Enforce MFA in your identity provider. Add an Access policy requiring MFA as a condition for access. This is required by NIST SP 800-63B for all government systems.

---

### ce-5: Cipher Suite Restriction

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, SC-8 |
| **Verification** | Inherited |
| **Code** | `pkg/cfapi/checker.go:checkCipherRestriction` |

**What it checks:** Queries the Cloudflare API (`/zones/{zone_id}/settings/ciphers`) for the zone's configured cipher suite restrictions. Validates that all configured ciphers match known FIPS-approved names.

Note: This verifies the *configuration*, not that the underlying crypto module is FIPS-validated. Cloudflare's edge uses BoringSSL, but the CMVP certificate belongs to Google (#4735), not Cloudflare. FIPS validation of the edge crypto module is inherited through Cloudflare's FedRAMP Moderate authorization.

**Criteria:**

- **Pass:** Custom cipher restriction set; all cipher names are FIPS-approved.
- **Warning:** Custom ciphers set but some may not be FIPS-approved, or no custom cipher restriction (Cloudflare defaults include non-FIPS ciphers).
- **Unknown:** API query failed.

**Remediation:** Set cipher suites via API: `PATCH /zones/{id}/settings/ciphers` with a FIPS-approved list (ECDHE-RSA-AES128-GCM-SHA256, ECDHE-ECDSA-AES128-GCM-SHA256, ECDHE-RSA-AES256-GCM-SHA384, ECDHE-ECDSA-AES256-GCM-SHA384).

---

### ce-6: Minimum TLS Version Enforced

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-8, SC-13 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkMinTLSVersion` |

**What it checks:** Queries the Cloudflare API (`/zones/{zone_id}/settings/min_tls_version`) for the zone's minimum TLS version setting.

**Criteria:**

- **Pass:** Minimum TLS version is 1.2 or 1.3.
- **Fail:** Minimum TLS version is 1.0 or 1.1 (prohibited by NIST SP 800-52 Rev 2).
- **Warning:** Unknown TLS version value.
- **Unknown:** API query failed.

**Remediation:** Set the minimum TLS version to 1.2 via the API or the Cloudflare dashboard: SSL/TLS > Edge Certificates > Minimum TLS Version.

---

### ce-7: Edge Certificate Valid

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-12, IA-5 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkEdgeCertificate` |

**What it checks:** Queries the Cloudflare API (`/zones/{zone_id}/ssl/certificate_packs`) for SSL certificate pack status and expiry dates.

**Criteria:**

- **Pass:** At least one active certificate pack with a valid expiry (more than 30 days remaining).
- **Warning:** Certificate expires within 30 days, or no active certificate pack found.
- **Fail:** Certificate is expired, or no certificate packs exist.
- **Unknown:** API query failed.

**Remediation:** Check SSL/TLS > Edge Certificates in the Cloudflare dashboard. Renew or re-order if approaching expiry.

---

### ce-8: HSTS Enforced

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-8, SC-23 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkHSTS` |

**What it checks:** Queries the Cloudflare API (`/zones/{zone_id}/settings/security_header`) for HTTP Strict Transport Security (HSTS) settings.

**Criteria:**

- **Pass:** HSTS is enabled (reports max-age, includeSubdomains, and preload settings).
- **Fail:** HSTS is not enabled.
- **Unknown:** API query failed.

**Remediation:** Enable HSTS in the Cloudflare dashboard: SSL/TLS > Edge Certificates > HTTP Strict Transport Security. Recommended: max-age=31536000, includeSubdomains=true, preload=true.

---

### ce-9: Tunnel Health

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-8, CP-8 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkTunnelHealth` |

**What it checks:** Queries the Cloudflare API (`/accounts/{account_id}/cfd_tunnel/{tunnel_id}`) for tunnel connection status.

**Criteria:**

- **Pass:** Tunnel status is "healthy" (reports number of active connections).
- **Warning:** Tunnel status is "degraded" (some connections down).
- **Fail:** Tunnel status is neither healthy nor degraded (e.g., inactive, down).
- **Unknown:** Account ID or Tunnel ID not configured, or API query failed.

**Remediation:** Check cloudflared logs. Restart the tunnel: `cloudflared tunnel run`. Verify network connectivity to Cloudflare edge IPs.

---

### ce-10: Keyless SSL (HSM Key Protection)

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-12, SC-13, SC-12(1) |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkKeylessSSL` |

**What it checks:** Queries the Cloudflare API for Keyless SSL configurations. Keyless SSL keeps private keys in customer-controlled FIPS 140-2 Level 3 HSMs (AWS CloudHSM, Azure Dedicated HSM, Entrust nShield, Fortanix, Google Cloud HSM, IBM Cloud HSM). Key operations flow through the cloudflared tunnel via PKCS#11.

**Criteria:**

- **Pass:** One or more active and enabled Keyless SSL configurations found.
- **Warning:** Keyless SSL configurations exist but are not active/enabled, or Keyless SSL is not configured (Tier 1 architecture -- Cloudflare holds private keys).
- **Unknown:** API query failed.

**Remediation:** Configure Keyless SSL: upload only the public certificate to Cloudflare, deploy the Keyless SSL module alongside cloudflared, connect to your HSM via PKCS#11. See `docs/deployment-tier-guide.md` for HSM vendor setup.

---

### ce-11: Regional Services (Data Locality)

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SC-8, PE-18 |
| **Verification** | API |
| **Code** | `pkg/cfapi/checker.go:checkRegionalServices` |

**What it checks:** Queries the Cloudflare API for Regional Services (Data Localization Suite) configuration. Regional Services restricts TLS termination and data processing to specific geographic regions (e.g., US-only for FedRAMP).

**Criteria:**

- **Pass:** Regional Services enabled -- TLS processing restricted to compliant data centers.
- **Warning:** Regional Services not enabled (traffic may be processed at any Cloudflare data center worldwide).
- **Unknown:** API query failed.

**Remediation:** Enable Regional Services (Data Localization Suite) in the Cloudflare dashboard to restrict processing to US FedRAMP-authorized data centers. Combined with Keyless SSL (ce-10), this provides Cloudflare's FIPS 140 Level 3 reference architecture.

---

## 3. Tunnel -- cloudflared (12 checks)

Segment 2: Cloudflare Edge to cloudflared tunnel daemon. This is the segment the cloudflared-fips product directly controls. All checks in this section use direct measurement of the running binary, operating system state, and local configuration.

### t-1: FIPS Crypto Backend Active

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, IA-7 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkBoringCryptoActive` |

**What it checks:** Detects the active FIPS crypto backend via `fipsbackend.Detect()`. Confirms a FIPS-validated crypto provider has replaced Go's standard crypto library. Reports the backend name, CMVP certificate number, and FIPS standard (140-2 or 140-3).

**Criteria:**

- **Pass:** A FIPS backend is detected and active (BoringCrypto on Linux, Go native FIPS on macOS/Windows).
- **Fail:** No FIPS backend detected; standard Go crypto is in use.

**Remediation:** Build with `GOEXPERIMENT=boringcrypto` (Linux) or `GODEBUG=fips140=on` (cross-platform). Use the provided Dockerfile.fips or build.sh.

---

### t-2: OS FIPS Mode (Server)

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-13, CM-6 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkOSFIPSMode` |

**What it checks:** Reads `/proc/sys/crypto/fips_enabled` on Linux to determine if the host operating system has FIPS mode enabled at the kernel level.

**Criteria:**

- **Pass:** `/proc/sys/crypto/fips_enabled` contains `1`.
- **Warning:** FIPS mode is not enabled, the file is not readable, or the OS is not Linux.

**Remediation:** Enable OS FIPS mode: `fips-mode-setup --enable && reboot` (RHEL/CentOS/Rocky/Alma). For containers, the host kernel must have FIPS enabled. The cloudflared-fips binary uses statically linked BoringCrypto and does not depend on OS OpenSSL, but an AO will require the entire system stack to be in FIPS mode.

---

### t-3: FIPS Self-Test

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, IA-7 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkFIPSSelfTest` |

**What it checks:** Runs Known Answer Tests (KATs) against NIST CAVP vectors for all FIPS-approved algorithms: AES-128-GCM, AES-256-GCM, SHA-256, SHA-384, HMAC-SHA-256, ECDSA P-256 sign/verify, and RSA-2048 sign/verify. These are power-up self-tests required by FIPS 140-2 Section 4.9.

**Criteria:**

- **Pass:** All KATs produce the expected output.
- **Fail:** Any KAT produces incorrect output, or the self-test suite fails to run. This indicates the crypto module is corrupted or compromised.

**Remediation:** Re-run `make selftest`. If failures persist, rebuild the binary from clean source. A KAT failure is a critical event that should trigger incident response per `docs/incident-response-addendum.md`.

---

### t-4: Tunnel Protocol

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SC-8, SC-13 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkTunnelProtocol` |

**What it checks:** Queries the cloudflared metrics endpoint (`http://{metricsAddr}/metrics`) to detect the active tunnel protocol (QUIC or HTTP/2). Falls back to `/proc` process detection if metrics are unavailable.

**Criteria:**

- **Pass:** Tunnel is active (metrics endpoint reachable or cloudflared process detected).
- **Unknown:** No cloudflared process detected and metrics endpoint unreachable.

**Remediation:** Start the tunnel: `cloudflared tunnel run`. Enable the metrics endpoint with `--metrics localhost:2000` for detailed protocol reporting. If the QUIC cipher audit is incomplete, set `protocol: http2` in the config to force HTTP/2.

---

### t-5: TLS Version (Edge to Tunnel)

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-8, SC-13 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkTLSVersion` |

**What it checks:** Inspects the Go TLS configuration to verify the minimum TLS version is set to 1.2 or higher, per NIST SP 800-52 Rev 2.

**Criteria:**

- **Pass:** `MinVersion` is TLS 1.2 or higher in the FIPS TLS configuration.
- **Fail:** `MinVersion` allows TLS 1.0 or 1.1.

**Remediation:** Set `MinVersion: tls.VersionTLS12` in the TLS config. BoringCrypto enforces this by default.

---

### t-6: Cipher Suites (Tunnel)

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, SC-8 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkCipherSuites` |

**What it checks:** Enumerates all cipher suites in the Go TLS stack and verifies each is FIPS-approved. With BoringCrypto or Go native FIPS active, the FIPS module blocks non-approved suites at runtime even if they appear in the static registry.

**Criteria:**

- **Pass:** All cipher suites are FIPS-approved, or a FIPS backend is active and enforces cipher restrictions at the module level.
- **Fail:** Non-FIPS cipher suites detected and no FIPS backend is active to block them.

**Remediation:** Build with `GOEXPERIMENT=boringcrypto` (Linux) or `GODEBUG=fips140=on` (cross-platform). The FIPS module will restrict ciphers to AES-GCM with ECDHE key exchange.

---

### t-7: Key Exchange (ECDHE)

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-12, SC-13 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkKeyExchange` |

**What it checks:** Verifies that the TLS configuration specifies ECDHE with NIST-approved curves (P-256, P-384) for key exchange, providing forward secrecy.

**Criteria:**

- **Pass:** `CurvePreferences` is set in the TLS configuration with NIST curves.
- **Warning:** No explicit curve preferences set (Go defaults may include non-NIST curves).

**Remediation:** Set `CurvePreferences` to `[]tls.CurveID{tls.CurveP256, tls.CurveP384}` in the TLS configuration.

---

### t-8: Tunnel Certificate Valid

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-12, IA-5 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkCertificateValidity` |

**What it checks:** Checks for a valid tunnel certificate (`cert.pem`) at standard locations (`~/.cloudflared/cert.pem`, `/etc/cloudflared/cert.pem`). For token-based tunnels, verifies that the cloudflared process is running with a JWT token.

**Criteria:**

- **Pass:** Certificate file found, or cloudflared is running with token-based authentication (JWT).
- **Unknown:** No certificate found and no cloudflared process detected.

**Remediation:** Re-authenticate: `cloudflared tunnel login`. For token-based tunnels, verify the token is valid in the Cloudflare dashboard.

---

### t-9: Tunnel Redundancy

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SC-8, CP-8 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkTunnelRedundancy` |

**What it checks:** Checks whether multiple tunnel connections are established to different Cloudflare edge servers for high availability. Queries the metrics endpoint, falling back to process detection.

**Criteria:**

- **Pass:** Tunnel connections active (metrics endpoint reachable or cloudflared process running; default is 4 redundant connections).
- **Unknown:** No cloudflared process detected.

**Remediation:** Check cloudflared logs for connection errors. Verify the network allows outbound connections to Cloudflare edge IPs (UDP 7844 for QUIC, TCP 443 for HTTP/2 fallback).

---

### t-10: Binary Integrity

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SI-7, CM-14 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkBinaryIntegrity` |

**What it checks:** Computes the SHA-256 hash of the running binary (auto-detected via `/proc/self/exe` on Linux) and compares it to the `binary_sha256` field in the build manifest (`configs/build-manifest.json`).

**Criteria:**

- **Pass:** Binary hash matches the manifest hash.
- **Warning:** Manifest has no binary hash to compare (manifest was generated without the binary).
- **Fail:** Binary hash does not match the manifest hash (possible tampering or corruption).
- **Unknown:** Binary path not available (non-Linux) or manifest not found.

**Remediation:** Re-download or rebuild the binary. Verify the manifest was not tampered with. Regenerate the manifest: `make manifest`.

---

### t-11: Config Drift Detection

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | CM-3, CM-6 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkConfigDrift` |

**What it checks:** Verifies the configuration file is present and readable. Full drift detection (comparing to a known-good baseline hash) requires integration with an external configuration management system.

**Criteria:**

- **Pass:** Configuration file exists and is accessible.
- **Unknown:** No config path specified, or config file not found.

**Remediation:** Compare the current configuration to the known-good baseline in your configuration management system. Configuration changes can weaken FIPS posture (e.g., disabling self-test, changing the cipher list).

---

### t-12: FIPS Crypto Module Identified

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, IA-7 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkFIPSBackend` |

**What it checks:** Identifies the active FIPS cryptographic module using `fipsbackend.DetectInfo()`. Reports the module display name, CMVP certificate number, and FIPS standard version (140-2 or 140-3).

**Criteria:**

- **Pass:** A validated FIPS module is active (e.g., "BoringCrypto | CMVP: #4735 | Standard: FIPS 140-3").
- **Warning:** A FIPS module is active but its CMVP validation status is uncertain (e.g., Go native FIPS with pending CMVP).
- **Fail:** No FIPS cryptographic module detected.

**Remediation:** Build with `GOEXPERIMENT=boringcrypto` (Linux) or `GODEBUG=fips140=on` (cross-platform). The AO must know the exact CMVP certificate number for the authorization package.

---

## 4. Local Service (4 checks)

Segment 3: cloudflared to local origin service. These checks probe the last hop between cloudflared and the application it protects. Loopback connections (localhost, 127.0.0.1, ::1) within the deployment boundary do not require TLS per AO interpretation (NIST SP 800-52 Section 3.5).

### l-1: Local TLS Enabled

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-8, SC-13 |
| **Verification** | Probe |
| **Code** | `internal/compliance/live.go:checkLocalTLSEnabled` |

**What it checks:** Attempts a TLS handshake with the first configured ingress target to determine if the local origin service accepts TLS connections. Loopback addresses are automatically passed without a TLS probe.

**Criteria:**

- **Pass:** TLS handshake succeeds, or the target is a loopback address (TLS not required within deployment boundary).
- **Fail:** The local service does not accept TLS connections (non-loopback target).
- **Unknown:** No ingress targets configured.

**Remediation:** Configure the local service to use TLS. Update ingress rules to use `https://` origin URLs. Use `--ingress-targets host:port` to specify the origin endpoint.

---

### l-2: Local Service Cipher Suite

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-13, SC-8 |
| **Verification** | Probe |
| **Code** | `internal/compliance/live.go:checkLocalCipherSuite` |

**What it checks:** Connects to the local origin service via TLS and inspects the negotiated cipher suite. Verifies the negotiated cipher is FIPS-approved (AES-GCM with ECDHE). Loopback addresses are automatically passed.

**Criteria:**

- **Pass:** Negotiated cipher is FIPS-approved (e.g., TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256), or target is loopback.
- **Fail:** Negotiated cipher is not FIPS-approved.
- **Unknown:** TLS handshake failed or no ingress targets configured.

**Remediation:** Configure the local service to prefer FIPS-approved cipher suites (AES-128-GCM or AES-256-GCM with ECDHE key exchange). Remove non-FIPS ciphers from the service's TLS configuration.

---

### l-3: Local Service Certificate Valid

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SC-12, IA-5 |
| **Verification** | Probe |
| **Code** | `internal/compliance/live.go:checkLocalCertificateValid` |

**What it checks:** Connects to the local origin service via TLS and checks the peer certificate for expiry. Loopback addresses are automatically passed.

**Criteria:**

- **Pass:** Certificate is valid and not expired, or target is loopback.
- **Warning:** No peer certificates presented.
- **Fail:** Certificate has expired.
- **Unknown:** TLS handshake failed or no ingress targets configured.

**Remediation:** Renew the local service certificate. Use a CA-issued certificate or an internal PKI. Automate renewal with certbot/ACME.

---

### l-4: Local Service Reachable

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-8, CP-8 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkServiceReachable` |

**What it checks:** Performs a TCP connection test to the first configured ingress target with a 3-second timeout.

**Criteria:**

- **Pass:** TCP connection succeeds.
- **Fail:** Cannot reach the target address.
- **Unknown:** No ingress targets configured.

**Remediation:** Check that the origin service is running and listening on the configured port. Verify no firewall rules block local connections.

---

## 5. Build and Supply Chain (7 checks)

These checks verify the integrity, provenance, and compliance status of the deployed binary and its build artifacts.

### b-1: Build Manifest Present

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SA-11, CM-14 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkBuildManifestPresent` |

**What it checks:** Verifies that the build manifest JSON file (`configs/build-manifest.json`) is present and parseable. The manifest contains cryptographic provenance: upstream cloudflared version, commit hash, FIPS certificate numbers, binary SHA-256 hash, and build timestamp.

**Criteria:**

- **Pass:** Manifest file exists and is valid JSON.
- **Fail:** Manifest file not found or cannot be parsed.

**Remediation:** Run `make manifest` to regenerate. Include the manifest in the deployment package. The manifest is generated automatically by CI.

---

### b-2: SBOM Available

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SA-11, SR-4 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkSBOMPresent` |

**What it checks:** Searches for a Software Bill of Materials (SBOM) file in CycloneDX or SPDX format at standard paths: `sbom.cyclonedx.json`, `sbom.spdx.json`, and `/etc/cloudflared-fips/` and `/opt/cloudflared-fips/` variants.

**Criteria:**

- **Pass:** An SBOM file found at one of the standard paths.
- **Warning:** No SBOM file found.

**Remediation:** Run `scripts/generate-sbom.sh` to generate CycloneDX 1.5 and SPDX 2.3 SBOMs. Executive Order 14028 requires SBOMs for software used in government systems.

---

### b-3: Reproducible Build

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SA-11, SI-7 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkReproducibleBuild` |

**What it checks:** Reads the binary's embedded `runtime/debug.BuildInfo` to verify deterministic build flags are present. Checks for `-trimpath` (removes absolute paths for reproducibility) and VCS revision info.

**Criteria:**

- **Pass:** Binary built with `-trimpath` and VCS info embedded.
- **Warning:** Binary missing `-trimpath`; builds may not be byte-reproducible.
- **Unknown:** Build info not available in the binary.

**Remediation:** Use deterministic build flags: `-trimpath`, `CGO_ENABLED=1`, `GOFLAGS=-mod=vendor`. All official builds use `SOURCE_DATE_EPOCH`, `-s -w -buildid=` for full reproducibility.

---

### b-4: Artifact Signature Valid

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SI-7, SA-11 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkSignatureValid` |

**What it checks:** Searches for a detached GPG signature (`.sig` or `.asc`) alongside the binary, or a `signatures.json` file at `/etc/cloudflared-fips/`.

**Criteria:**

- **Pass:** An artifact signature file found.
- **Warning:** No artifact signature found.
- **Unknown:** Binary path not available.

**Remediation:** Sign artifacts in CI with GPG (binaries/packages) or cosign (container images). See `scripts/sign-artifacts.sh` and `docs/key-rotation-procedure.md`.

---

### b-5: FIPS Certificates Listed

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, SA-11 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkFIPSCertsListed` |

**What it checks:** Parses the build manifest and verifies the `fips_certificates` array is populated with CMVP certificate entries for all crypto modules.

**Criteria:**

- **Pass:** One or more FIPS certificates listed in the manifest.
- **Fail:** No FIPS certificates listed, or manifest not found.

**Remediation:** Ensure `scripts/generate-manifest.sh` includes the `fips_certificates` array. The AO requires certificate numbers for the authorization package. Current certificates: BoringCrypto #4735 (FIPS 140-3), RHEL OpenSSL #4746/#4857 (FIPS 140-3).

---

### b-6: Upstream Version Tracked

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SA-11, SI-2 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkUpstreamVersion` |

**What it checks:** Verifies the upstream cloudflared version is recorded in the build manifest's `cloudflared_upstream_version` field.

**Criteria:**

- **Pass:** Upstream version field is populated.
- **Warning:** Upstream version not recorded or manifest not found.

**Remediation:** Update the upstream version in the build configuration and regenerate the manifest. Tracking the upstream version ensures security patches are applied promptly.

---

### b-7: FIPS Module Sunset Status

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-13, SA-11 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkFIPSModuleSunset` |

**What it checks:** Checks the active FIPS module against the FIPS 140-2 sunset deadline (September 21, 2026). After that date, all FIPS 140-2 certificates move to the CMVP Historical List and only FIPS 140-3 modules are valid for new federal acquisitions.

**Criteria:**

- **Pass:** Using FIPS 140-3, or sunset is more than 180 days away (urgency: none or low).
- **Warning:** Sunset is 90-180 days away (urgency: medium or high). Migration planning required.
- **Fail:** FIPS 140-2 module is past sunset or within 90 days (urgency: critical).
- **Unknown:** Cannot determine migration status.

**Remediation:** Upgrade to a FIPS 140-3 module. Options: BoringCrypto #4735 (build with Go 1.24+), or Go native FIPS (`GODEBUG=fips140=on`, CMVP pending). See `pkg/fipsbackend/migration.go` for migration status tracking and `/api/v1/migration` for programmatic queries.

---

## 6. Security Operations (13 checks)

Operational security controls covering audit logging, authentication, alerting, credential lifecycle, and security policy enforcement.

### so-1: Audit Log Active

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | AU-2, AU-3 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkAuditLogActive` |

**What it checks:** Verifies that the audit logger is configured, the audit log file is writable, and events are being recorded.

**Criteria:**

- **Pass:** Audit log is active and has recorded events.
- **Warning:** Audit log file exists but contains no events.
- **Fail:** No audit logger configured.

**Remediation:** Start the dashboard with `--audit-log /var/log/cloudflared-fips/audit.json`. Ensure the log directory exists and is writable by the cloudflared-fips service account.

---

### so-2: Dashboard Authentication

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | AC-2, AC-3 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkDashboardAuth` |

**What it checks:** Checks whether Bearer token authentication is enabled for the dashboard API.

**Criteria:**

- **Pass:** Token authentication is enabled.
- **Warning:** Dashboard API is in open mode (localhost-only, no token required).

**Remediation:** Set `--dashboard-token` or the `DASHBOARD_TOKEN` environment variable. While the dashboard binds to localhost by default, token authentication provides defense-in-depth against local privilege escalation.

---

### so-3: Failed Auth Monitoring

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | AC-7, SI-4 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkFailedAuthMonitoring` |

**What it checks:** Verifies that both authentication and audit logging are active, enabling failed authentication attempt tracking with IP-based lockout (5 attempts per 5 minutes).

**Criteria:**

- **Pass:** Both authentication and audit logging are active; failed attempts are logged with IP lockout.
- **Warning:** Audit logging active but auth disabled (no failed attempts to monitor), or neither is configured.

**Remediation:** Enable both `--dashboard-token` and `--audit-log` for full auth monitoring with lockout.

---

### so-4: Alerting Configured

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | CA-7, SI-4 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkAlertingConfigured` |

**What it checks:** Checks whether webhook alerting is configured for compliance events. Reports the number of configured webhook endpoints.

**Criteria:**

- **Pass:** One or more webhook endpoints configured.
- **Warning:** No webhooks configured (alerts are logged locally only, with no external notification).

**Remediation:** Set `--alert-webhook` to one or more webhook URLs (e.g., Slack incoming webhook, PagerDuty Events API, SIEM webhook endpoint).

---

### so-5: Secrets at Rest

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-28, SC-12 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkSecretsAtRest` |

**What it checks:** Scans secret files (tokens, keys, certificates) at default locations and configured paths for file permission violations. Checks for world-readable permissions and group permissions wider than 0640.

Default secret file locations checked:
- `/etc/cloudflared-fips/token`
- `/etc/cloudflared-fips/node-api-key`
- `/etc/cloudflared-fips/cloudflared-fips.yaml`
- `/etc/cloudflared/cert.pem`
- `~/.cloudflared/cert.pem`

Additional paths scanned via `--secrets-paths`: any file ending in `.pem`, `.key`, `.token`, or containing `secret`, `credential`, `api-key`, `node-api-key`.

**Criteria:**

- **Pass:** All secret files have restricted permissions (no world-readable, group at most 040).
- **Warning:** Some files have wider-than-ideal permissions but are not world-readable.
- **Fail:** One or more secret files are world-readable (other bits set).

**Remediation:** `chmod 0640` on all secret files; `chown` to the `cloudflared-fips` user and group.

---

### so-6: Tunnel Token Expiry

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SC-12, IA-5 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkTunnelTokenExpiry` |

**What it checks:** Checks the tunnel authentication token for approaching expiry. For PEM certificate files, parses the X.509 certificate and checks the NotAfter date. For opaque token files, uses file modification time as a proxy for age. For runtime JWT tokens (passed via `--token` flag), reports that the token is managed by Cloudflare.

**Criteria:**

- **Pass:** Token/certificate valid with more than 30 days remaining, or runtime JWT token in use.
- **Warning:** Token file modified more than 365 days ago, or certificate expires within 30 days.
- **Fail:** Certificate has expired.
- **Unknown:** No token file found and no cloudflared process detected.

**Remediation:** Rotate the tunnel token via the Cloudflare dashboard or API before expiry. Use `--token-path` to specify the token file location.

---

### so-7: TLS Certificate Expiry

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **NIST Controls** | SC-12, IA-5 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkTLSCertExpiry` |

**What it checks:** Parses all TLS certificates at configured paths (`--cert-paths`) and checks their X.509 NotAfter dates.

**Criteria:**

- **Pass:** All certificates valid with more than 30 days remaining.
- **Warning:** Any certificate expires within 30 days.
- **Fail:** Any certificate has expired.
- **Unknown:** No certificate paths configured, or no parseable certificates found.

**Remediation:** Renew certificates before expiry. Automate renewal with certbot/ACME. Use `--cert-paths` to monitor all TLS certificates.

---

### so-8: Fleet Token Age

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | SC-12, IA-5 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkFleetTokenAge` |

**What it checks:** Checks the age of fleet enrollment API key files at `/etc/cloudflared-fips/node-api-key` and `/var/lib/cloudflared-fips/node-api-key` based on file modification time.

**Criteria:**

- **Pass:** Key file is less than 90 days old.
- **Warning:** Key file is more than 90 days old (rotation recommended).
- **Fail:** Key file is more than 180 days old.
- **Unknown:** No fleet enrollment key found (not in fleet mode or key managed externally).

**Remediation:** Rotate fleet enrollment tokens every 90 days via the fleet API. Generate new tokens and re-enroll nodes.

---

### so-9: Upstream cloudflared Integrity

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | SI-7, SA-11 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkUpstreamIntegrity` |

**What it checks:** Computes the SHA-256 hash of the upstream cloudflared binary at standard paths (`/usr/local/bin/cloudflared`, `/usr/bin/cloudflared`) and compares it to the expected checksum provided via `--upstream-checksum`.

**Criteria:**

- **Pass:** Hash matches the expected checksum.
- **Warning:** Binary found but no expected checksum configured.
- **Fail:** Hash does not match (possible tampering or unexpected version).
- **Unknown:** Upstream binary not found at standard paths, or cannot read the binary.

**Remediation:** Download cloudflared from the official Cloudflare release page and verify its checksum. Set `--upstream-checksum` to the expected SHA-256 hash.

---

### so-10: Log Forwarding (SIEM)

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | AU-4, AU-9 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkLogForwarding` |

**What it checks:** Checks whether audit logs are forwarded to an external SIEM via syslog, providing out-of-band log integrity protection.

**Criteria:**

- **Pass:** Syslog forwarding is active (SIEM integration).
- **Warning:** Audit log active but no syslog forwarding (local log only).
- **Fail:** No audit logging configured.

**Remediation:** Set `--syslog-addr` to forward audit events to your SIEM (e.g., `--syslog-addr tcp://siem.example.com:514`).

---

### so-11: Audit Log Integrity

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | AU-9 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkAuditLogIntegrity` |

**What it checks:** Checks audit log file permissions to prevent unauthorized modification or reading.

**Criteria:**

- **Pass:** Audit log permissions are 0640 or more restrictive.
- **Warning:** Audit log permissions are wider than 0640 but not world-accessible, or audit logger not configured.
- **Fail:** Audit log is world-accessible (other bits set).

**Remediation:** `chmod 0640` on audit log files; `chown` to `cloudflared-fips:cloudflared-fips`.

---

### so-12: Compliance Scan Recent

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **NIST Controls** | CA-7 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkComplianceScanRecent` |

**What it checks:** Records the timestamp of the current compliance scan and reports its age. The dashboard performs scans on every page load and SSE refresh cycle (30-second interval).

**Criteria:**

- **Pass:** Last scan was less than 5 minutes ago.
- **Warning:** Last scan was 5 to 30 minutes ago.
- **Fail:** Last scan was more than 30 minutes ago.

**Remediation:** The dashboard performs scans automatically on every page load and SSE refresh (30s). If this check fails, the dashboard may not be running or the SSE connection may be disconnected.

---

### so-13: Security Policy Enforced

| Field | Value |
|-------|-------|
| **Severity** | High |
| **NIST Controls** | PL-1 |
| **Verification** | Direct |
| **Code** | `internal/compliance/live.go:checkSecurityPolicyEnforced` |

**What it checks:** Reports the current security policy enforcement mode, which controls whether non-compliant requests are blocked, logged, or ignored.

**Criteria:**

- **Pass:** Enforcement mode is `enforce` (non-compliant requests are blocked).
- **Warning:** Enforcement mode is `audit` (violations logged but not blocked) or `disabled` (no enforcement active).

**Remediation:** Set `enforcement_mode: enforce` in the configuration file or use `--enforcement-mode enforce` on the command line. Start with `audit` mode to observe violations, then switch to `enforce` once the environment is validated.

---

## Appendix A: NIST SP 800-53 Rev 5 Control Cross-Reference

The following table lists every NIST control referenced by compliance checks in this product, with the checks that map to each control.

| Control | Title | Checks |
|---------|-------|--------|
| AC-2 | Account Management | so-2 |
| AC-3 | Access Enforcement | ce-1, so-2 |
| AC-7 | Unsuccessful Logon Attempts | so-3 |
| AC-17 | Remote Access | ce-1, cp-6 |
| AU-2 | Audit Events | so-1 |
| AU-3 | Content of Audit Records | so-1 |
| AU-4 | Audit Storage Capacity | so-10 |
| AU-9 | Protection of Audit Information | so-10, so-11 |
| CA-7 | Continuous Monitoring | so-4, so-12 |
| CM-2 | Baseline Configuration | cp-7 |
| CM-3 | Configuration Change Control | t-11 |
| CM-6 | Configuration Settings | t-2, t-11, cp-1 |
| CM-8 | System Component Inventory | cp-2, cp-6, cp-7 |
| CM-14 | Signed Components | t-10, b-1 |
| CP-8 | Telecommunications Services | t-9, l-4, ce-9 |
| IA-2 | Identification and Authentication | ce-2, ce-3, ce-4, cp-8 |
| IA-2(1) | MFA to Privileged Accounts | ce-4 |
| IA-2(2) | MFA to Non-Privileged Accounts | ce-4 |
| IA-5 | Authenticator Management | t-8, ce-7, so-6, so-7, so-8, cp-8 |
| IA-7 | Cryptographic Module Authentication | t-1, t-3, t-12 |
| IA-8 | Identification and Authentication (Non-Org Users) | ce-2, ce-3 |
| PE-18 | Location of System Components | ce-11 |
| PL-1 | Planning Policy and Procedures | so-13 |
| SA-11 | Developer Testing and Evaluation | b-1, b-2, b-3, b-4, b-5, b-6, b-7, so-9 |
| SC-8 | Transmission Confidentiality and Integrity | t-4, t-5, t-6, t-9, l-1, l-2, l-4, ce-5, ce-6, ce-8, ce-9, ce-11, cp-3, cp-4, cp-5 |
| SC-12 | Cryptographic Key Establishment and Management | t-7, t-8, l-3, ce-7, ce-10, so-5, so-6, so-7, so-8 |
| SC-12(1) | Availability | ce-10 |
| SC-13 | Cryptographic Protection | t-1, t-3, t-5, t-6, t-7, t-12, l-1, l-2, ce-5, ce-6, ce-10, cp-1, cp-3, cp-4, cp-5, b-5, b-7 |
| SC-23 | Session Authenticity | ce-8 |
| SC-28 | Protection of Information at Rest | so-5 |
| SI-2 | Flaw Remediation | cp-2, b-6 |
| SI-4 | System Monitoring | so-3, so-4 |
| SI-7 | Software, Firmware, and Information Integrity | t-10, b-3, b-4, so-9 |
| SR-4 | Provenance | b-2 |

---

## Appendix B: API Endpoints for Programmatic Access

All compliance data is available via the dashboard REST API:

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/compliance` | Full compliance report (all sections) |
| `GET /api/v1/compliance/export` | Compliance report formatted for export |
| `GET /api/v1/selftest` | Self-test results (KATs) |
| `GET /api/v1/manifest` | Build manifest data |
| `GET /api/v1/backend` | Active FIPS backend information |
| `GET /api/v1/migration` | FIPS 140-2 sunset migration status |
| `GET /api/v1/migration/backends` | All backend info with migration details |
| `GET /api/v1/health` | Service health check |
| `GET /api/v1/signatures` | Artifact signature manifest |

SSE endpoint for real-time updates: `GET /api/v1/compliance/stream`

---

## Appendix C: Source File Index

| File | Checks |
|------|--------|
| `internal/compliance/live.go` | t-1 through t-12, l-1 through l-4, b-1 through b-7, so-1 through so-13 |
| `internal/compliance/types.go` | Type definitions (Status, Section, ChecklistItem, VerificationMethod) |
| `internal/compliance/checker.go` | Report aggregation and overall status computation |
| `pkg/cfapi/checker.go` | ce-1 through ce-11 |
| `pkg/clientdetect/checker.go` | cp-1 through cp-8 |
| `pkg/fipsbackend/` | Backend detection, migration status (used by t-1, t-12, b-7) |
| `internal/selftest/` | KAT runners, cipher validation (used by t-3, t-5, t-6) |
| `pkg/audit/` | Audit logger (used by so-1, so-3, so-10, so-11) |
| `pkg/alerts/` | Alert manager (used by so-4) |

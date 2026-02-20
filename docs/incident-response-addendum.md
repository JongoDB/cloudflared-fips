# Incident Response Addendum

## Cryptographic Compliance Failure Procedures

This addendum extends the organization's Incident Response Plan with procedures
specific to cloudflared-fips cryptographic compliance failures.

---

### 1. Scope

This addendum covers incidents where a compliance dashboard check transitions
to red (non-compliant), indicating a potential failure in the FIPS-validated
cryptographic chain.

### 2. Incident Categories

#### Category A: Cryptographic Module Failure
- Self-test KAT failure
- BoringCrypto symbols missing from running binary
- Binary integrity hash mismatch

**Severity:** Critical
**Impact:** All tunnel traffic may be using non-validated cryptography

#### Category B: Configuration Compliance Failure
- OS FIPS mode disabled
- Non-FIPS cipher suite negotiated
- TLS version below 1.2

**Severity:** High
**Impact:** Specific connections may not meet FIPS requirements

#### Category C: Availability Failure
- Tunnel disconnected
- Origin service unreachable
- Dashboard unreachable

**Severity:** Medium (from compliance perspective)
**Impact:** Cannot verify compliance status; service disrupted

#### Category D: Client Posture Failure
- Client OS FIPS mode not verified
- MDM enrollment lapsed
- Device posture check failed

**Severity:** Medium
**Impact:** Client-to-edge segment may not be FIPS-compliant

---

### 3. Response Procedures

#### Category A: Cryptographic Module Failure

**Immediate (0-1 hour):**
1. Export compliance report (JSON) from dashboard for evidence
2. Stop the affected cloudflared tunnel instance
3. Notify the ISSO and system owner
4. Isolate affected system from production traffic

**Investigation (1-4 hours):**
5. Compare running binary hash against build-manifest.json `binary_sha256`
6. Run self-test manually: `/usr/local/bin/selftest`
7. Check for unauthorized binary replacement: `rpm -V cloudflared-fips` or `dpkg -V`
8. Review system logs for tampering indicators
9. Check if OS or package update replaced the FIPS binary with non-FIPS version

**Recovery (4-24 hours):**
10. Redeploy known-good FIPS binary from trusted artifact store
11. Verify self-test passes on redeployed binary
12. Verify dashboard shows all-green for tunnel section
13. Restore traffic to the tunnel
14. Document incident timeline and root cause

**Reporting:**
15. File incident report per organizational IR plan
16. Report to AO if cryptographic operations occurred with non-validated module
17. Update continuous monitoring documentation

#### Category B: Configuration Compliance Failure

**Immediate (0-4 hours):**
1. Export compliance report from dashboard
2. Identify the specific non-compliant configuration
3. Assess whether in-flight traffic was affected

**Remediation:**
4. For OS FIPS mode: `fips-mode-setup --enable && reboot`
5. For cipher suite: Update cloudflared-fips.yaml, restart tunnel
6. For TLS version: Update min-tls-version in config, restart tunnel
7. Verify dashboard shows green after remediation

**Reporting:**
8. Document configuration drift cause and remediation
9. Update baseline configuration in version control
10. Notify ISSO if non-compliant traffic was processed

#### Category C: Availability Failure

**Immediate:**
1. Follow standard availability incident response procedures
2. Note: While tunnel is down, compliance cannot be verified
3. Document the gap in compliance monitoring

**Recovery:**
4. Restore tunnel connectivity
5. Verify dashboard shows all checks passing
6. Review any connections established during the outage

#### Category D: Client Posture Failure

**Immediate:**
1. Cloudflare Access device posture should have blocked the connection
2. If connection was allowed despite posture failure, this is a policy misconfiguration
3. Review Access policy configuration

**Remediation:**
4. Update device posture requirements in Cloudflare Access
5. Work with endpoint team to re-enroll device in MDM
6. Verify FIPS mode is re-enabled on the client endpoint

---

### 4. Evidence Preservation

For all incidents, preserve the following:

| Evidence | Source | Format |
|----------|--------|--------|
| Compliance report | Dashboard export | JSON |
| Self-test results | selftest binary | JSON (stdout) |
| System logs | journald/syslog | Text |
| Binary hash | sha256sum | Text |
| Build manifest | build-manifest.json | JSON |
| Network captures | tcpdump (if applicable) | PCAP |
| Dashboard screenshots | Browser | PNG |
| Cloudflare audit logs | Cloudflare API | JSON |

### 5. Communication Templates

#### ISSO Notification (Category A)

```
Subject: [CRITICAL] FIPS Cryptographic Module Failure — cloudflared-fips

A FIPS cryptographic module failure has been detected on [hostname].
Self-test result: [FAILED]
Binary integrity: [MISMATCH/OK]
Timestamp: [ISO 8601]

Immediate action: Tunnel stopped, system isolated.
Investigation in progress. Full compliance report attached.
```

#### AO Notification (Category A, if non-validated crypto was used)

```
Subject: [SECURITY] Potential Non-Validated Cryptographic Operation

System: [System Name]
Component: cloudflared-fips on [hostname]
Duration: [start time] to [detection time]
Impact: Tunnel traffic may have used non-FIPS-validated cryptography

Root cause: [TBD / description]
Remediation: [completed / in progress]
Full incident report to follow within [24/48/72] hours.
```

---

### 6. Post-Incident Review

After every Category A or B incident:

1. Conduct post-incident review within 5 business days
2. Update this addendum if new failure modes are identified
3. Update continuous monitoring plan if detection was delayed
4. Update self-test suite if the failure mode was not covered
5. Update build pipeline if the root cause was a build issue

---

_This addendum follows NIST SP 800-61 (Incident Handling Guide) and
integrates with NIST SP 800-53 IR-4 (Incident Handling) and IR-6
(Incident Reporting) controls._

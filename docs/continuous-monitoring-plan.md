# Continuous Monitoring Plan

## cloudflared-fips — Ongoing FIPS Compliance Verification

---

### 1. Overview

This plan establishes the continuous monitoring process for maintaining FIPS 140-2
compliance of the cloudflared-fips deployment. The compliance dashboard serves as
the primary continuous monitoring tool.

### 2. Dashboard as Continuous Monitoring Tool

The cloudflared-fips compliance dashboard provides real-time monitoring of all
FIPS compliance properties across three connection segments:

| Monitoring Area | Update Frequency | Data Source |
|----------------|-----------------|-------------|
| Client posture | Per-connection | Cloudflare Access API, MDM API |
| Edge configuration | Every 5 minutes | Cloudflare API |
| Tunnel crypto status | Real-time (SSE) | Local self-test, /metrics |
| Local service status | Every 30 seconds | TCP/TLS probe |
| Build integrity | At startup + daily | Binary hash, manifest |
| Configuration drift | Every 5 minutes | Config file comparison |

### 3. Validated Module Status Monitoring

#### Process for Re-Verifying on cloudflared Updates

When upstream cloudflared releases a new version:

1. **Check BoringCrypto compatibility**
   - Verify the new cloudflared version still works with `GOEXPERIMENT=boringcrypto`
   - Run: `make build-fips` with the new upstream tag
   - Verify BoringCrypto symbols: `scripts/verify-boring.sh`

2. **Run full self-test suite**
   - Execute: `make selftest`
   - Verify all KATs pass
   - Verify cipher suite list contains only FIPS-approved suites

3. **Audit dependency changes**
   - Compare `go.sum` between versions
   - Run `govulncheck` for known vulnerabilities
   - Check for new crypto-related dependencies that may bypass BoringCrypto

4. **Regenerate SBOM**
   - Generate new CycloneDX and SPDX SBOMs
   - Flag any new dependencies that perform cryptographic operations

5. **Update build manifest**
   - Run: `make manifest`
   - Verify manifest reflects new upstream version and commit

6. **Deploy to staging and verify dashboard**
   - All checklist items should be green or yellow (with documented reasons)
   - No new red items compared to previous version

#### CMVP Certificate Monitoring

| Module | Certificate | Expiry | Monitoring Action |
|--------|-----------|--------|-------------------|
| BoringCrypto | #3678 | Check CMVP quarterly | If revoked/expired, migrate to Go native FIPS (when validated) |
| RHEL OpenSSL | #4349 | Check CMVP quarterly | Update RHEL version if certificate changes |
| Windows CNG | #2357+ | Check CMVP quarterly | Document in client hardening guide |

**CMVP Status Check URL:** https://csrc.nist.gov/projects/cryptographic-module-validation-program/validated-modules

### 4. Vulnerability Response Process

#### Severity Classification

| Severity | Crypto Impact | Response Time | Action |
|----------|--------------|---------------|--------|
| Critical | Validated module compromised | 24 hours | Emergency rebuild + redeploy |
| High | Cipher suite vulnerability | 72 hours | Rebuild with updated module |
| Medium | Non-crypto dependency vuln | 7 days | Standard patch cycle |
| Low | Informational | 30 days | Next scheduled update |

#### Response Steps

1. **Identify** — `govulncheck` in CI, CMVP alerts, vendor advisories
2. **Assess** — Determine if the vulnerability affects FIPS-validated operations
3. **Mitigate** — Apply patch, update module, or restrict cipher suites
4. **Verify** — Run full self-test suite, verify dashboard all-green
5. **Document** — Update SBOM, build manifest, compliance report
6. **Report** — Notify AO if the vulnerability affected validated module

### 5. Scheduled Activities

| Activity | Frequency | Owner | Artifact |
|----------|-----------|-------|----------|
| Dashboard review | Daily | Operations | Screenshot/export |
| Self-test execution | At every restart | Automated | JSON log |
| CMVP certificate check | Quarterly | Security | Status report |
| Upstream version assessment | Monthly | Engineering | Compatibility report |
| Full compliance audit | Annually | ISSO | Compliance report (JSON + PDF) |
| SBOM regeneration | Every build | CI/CD | CycloneDX + SPDX |
| Dependency vulnerability scan | Weekly | CI/CD | govulncheck report |
| Configuration drift check | Continuous | Dashboard | Real-time alert |

### 6. Alerting

The dashboard generates alerts when any compliance check transitions from
green to yellow or red:

| Alert | Trigger | Channel | Escalation |
|-------|---------|---------|------------|
| Self-test failure | Any KAT fails | Syslog + email | Immediate — ISSO |
| OS FIPS mode disabled | /proc/sys/crypto/fips_enabled != 1 | Dashboard + syslog | 24h — Operations |
| Binary integrity mismatch | Hash != manifest | Dashboard + syslog | Immediate — Security |
| Certificate expiry < 30 days | Edge cert check | Dashboard + email | 7 days — Operations |
| Configuration drift detected | Config != baseline | Dashboard | 24h — Operations |

### 7. Documentation Updates

When any of the following change, update the corresponding AO documentation:

| Change | Documents to Update |
|--------|-------------------|
| BoringCrypto version | Crypto Module Usage, Build Manifest, SSP |
| CMVP certificate status | Justification Letter, Crypto Module Usage |
| New upstream dependency | SBOM, Crypto Module Usage |
| Cipher suite change | Crypto Module Usage, Control Mapping |
| New platform target | Build Manifest, CI Pipeline |

---

_This plan follows NIST SP 800-137 (Continuous Monitoring) and integrates
with the Risk Management Framework per SP 800-37._

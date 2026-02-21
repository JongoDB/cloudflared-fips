# cloudflared-fips Demo Walkthrough

Step-by-step script for demonstrating the cloudflared-fips compliance dashboard and product capabilities.

## Prerequisites

- Node.js 18+ installed
- Repository cloned locally

## 1. Start the Dashboard

```bash
cd dashboard
npm install   # first time only
npm run dev
```

Open http://localhost:5173 in a browser.

## 2. Overview — First Impressions

Point out the three key elements visible immediately:

- **Sunset banner** at the top: shows the FIPS 140-2 end-of-life countdown (September 21, 2026) with urgency color coding. This is the product's migration awareness feature.
- **Deployment tier badge**: shows the current tier (Standard / Regional Keyless / Self-Hosted). Explains the three-tier architecture.
- **Compliance summary bar**: 41 total checks with pass/warn/fail breakdown and overall compliance percentage.

## 3. FIPS Cryptographic Module Card

The card at the top of the dashboard shows:

- **Active backend**: BoringCrypto (or Go Native FIPS, depending on build)
- **CMVP certificate number**: e.g., #4407 (140-2) or #4735 (140-3)
- **FIPS standard badge**: color-coded 140-2 (amber) vs 140-3 (green)
- **Validation status**: whether the module holds a current CMVP certificate

Explain: "This tells the AO exactly which validated cryptographic module is protecting the tunnel, and whether it meets current or legacy FIPS standards."

## 4. Build Manifest Panel

Click **Build Manifest** to expand.

Show:
- Build version, commit hash, build timestamp
- Upstream cloudflared version being wrapped
- Crypto engine and BoringSSL version
- FIPS certificate list with algorithm details
- Target platform and package format
- SBOM hash and binary integrity hash

Explain: "Every build produces this manifest. It's the chain of custody from source code to deployed binary."

## 5. Walk Each Section

### Client Posture (8 items)

- OS FIPS mode, OS type/version, browser TLS capabilities
- Negotiated cipher suite, TLS version
- Access device posture, MDM enrollment, client certificate

Explain: "We can't control client-side crypto, but we can detect and report it. These items use ClientHello inspection and device posture APIs."

### Cloudflare Edge (11 items)

- Access policy, IdP, auth method, MFA
- Cipher restriction, min TLS version, HSTS
- Edge certificate validity, tunnel health
- Keyless SSL status, Regional Services

Explain: "We verify everything we can about the Cloudflare edge via API and TLS probing. Items we can't independently verify are marked 'Inherited' — relying on Cloudflare's FedRAMP authorization."

### Tunnel — cloudflared (12 items)

- BoringCrypto active, OS FIPS mode, self-test result
- Protocol, TLS version, cipher suite
- Tunnel auth, redundancy, uptime
- Binary integrity, version, heartbeat

Explain: "This is our segment — the one we directly control. Every item here is verified locally via self-test, system calls, or metrics queries."

### Local Service (4 items)

- Connection type (loopback vs networked)
- TLS enabled, cipher suite, service reachability

Explain: "Segment 3 — cloudflared to the origin. If networked, we actively probe the TLS connection to verify FIPS cipher compliance."

### Build & Supply Chain (6 items)

- SBOM verified, build reproducibility
- BoringCrypto version, RHEL OpenSSL module
- Configuration drift, last compliance scan

Explain: "Supply chain integrity. The SBOM is auto-generated at build time with every crypto dependency flagged."

## 6. Expand a Checklist Item

Click any item (e.g., "BoringCrypto Active") to expand it. Show the four fields:

- **What**: what this check verifies
- **Why**: why it matters for FIPS compliance, with NIST/FIPS reference
- **Remediation**: what to do if this check fails
- **NIST Reference**: applicable control (e.g., SC-13, IA-7)

Explain: "Every red item has a concrete remediation path. This isn't just a dashboard — it's a runbook."

## 7. Verification Badges

Point out the colored badges next to each item's severity indicator:

| Badge | Color | Meaning |
|-------|-------|---------|
| Direct | Green | Measured locally (self-test, binary hash) |
| API | Blue | Queried from Cloudflare API |
| Probe | Purple | TLS handshake inspection |
| Inherited | Amber | Relies on provider's FedRAMP authorization |
| Reported | Gray | Client-reported via device posture |

Explain: "This is our honesty layer. We don't claim to verify things we can't. Each badge tells the AO exactly how this data was obtained."

## 8. Export Compliance Report

Click **Export JSON** to download the full compliance report as a JSON file.

Explain: "This is machine-readable compliance data. It feeds into SIEM systems, GRC platforms, or the AO's evidence package."

## 9. Architecture — Three Segments

Reference `docs/architecture-diagram.md` or draw the three segments:

```
Client ──[Segment 1]──> Cloudflare Edge ──[Segment 2]──> cloudflared-fips ──[Segment 3]──> Origin
```

Explain: "We built observability for all three segments. We control Segment 2, verify Segments 1 and 3, and are transparent about the Cloudflare edge gap."

## 10. Deployment Tiers

Walk through the three deployment tiers:

- **Tier 1 (Standard)**: Cloudflare tunnel — edge crypto inherited via FedRAMP
- **Tier 2 (Regional Keyless)**: Keyless SSL + HSM — private keys never touch Cloudflare
- **Tier 3 (Self-Hosted)**: FIPS proxy in GovCloud — full control over every TLS termination point

Explain: "The product scales from 'good enough for most federal use cases' to 'DoD IL5 with zero trust gaps.'"

## 11. AO Documentation Package

Show the 11 documents in `docs/`:

1. `ao-narrative.md` — System Security Plan template
2. `crypto-module-usage.md` — Operation-to-module mapping
3. `fips-justification-letter.md` — AO justification for leveraged validation
4. `client-hardening-guide.md` — Windows/RHEL/macOS FIPS setup
5. `continuous-monitoring-plan.md` — Ongoing compliance verification
6. `incident-response-addendum.md` — Crypto failure procedures
7. `control-mapping.md` — NIST 800-53 controls
8. `architecture-diagram.md` — Visual architecture
9. `deployment-tier-guide.md` — Tier setup with HSM guides
10. `key-rotation-procedure.md` — Signing key rotation
11. `quic-go-crypto-audit.md` — QUIC protocol FIPS analysis

Explain: "This is the AO package. These templates, combined with the dashboard's live evidence, give an Authorizing Official everything they need to make an authorization decision."

## Closing

"cloudflared-fips makes FIPS compliance for Cloudflare Tunnel transparent, verifiable, and auditable — from client to origin, with honest reporting about what we can and cannot verify."

# Deployment Tier Guide

This guide covers the three deployment tiers for cloudflared-fips, with detailed setup instructions for Tier 2 (Cloudflare's official FIPS 140 Level 3 reference architecture).

## Tier Overview

| | Tier 1: Standard | Tier 2: FIPS 140 L3 (Keyless SSL) | Tier 3: Self-Hosted |
|---|---|---|---|
| **Edge crypto** | Inherited (FedRAMP) | Inherited + HSM keys | Fully controlled |
| **Key management** | Cloudflare-managed | Customer HSM (L3) | Customer-managed |
| **Private keys** | On Cloudflare edge | Never leave HSM | On your server |
| **Data locality** | Global | Regional (US FedRAMP DCs) | Your GovCloud |
| **Config** | `deployment_tier: standard` | `deployment_tier: regional_keyless` | `deployment_tier: self_hosted` |
| **Complexity** | Low | Medium | High |

---

## Tier 1: Standard Cloudflare Tunnel

```
Client → Cloudflare Edge → cloudflared-fips → Origin
```

### Setup

1. Build or install cloudflared-fips (RPM, DEB, or binary)
2. Run self-test: `cloudflared-fips selftest`
3. Configure tunnel: `cloudflared tunnel create <name>`
4. Set `deployment_tier: standard` in `cloudflared-fips.yaml`
5. Start dashboard: `cloudflared-fips dashboard --config cloudflared-fips.yaml`

### What you control

- Segment 2b (tunnel binary): FIPS-validated via BoringCrypto
- Segment 3 (origin): your local service TLS configuration

### What you inherit

- Segment 2a (Cloudflare edge): FedRAMP Moderate authorized, but edge crypto module not independently FIPS-validated. Dashboard shows these items as "Inherited."

---

## Tier 2: Cloudflare's FIPS 140 Level 3 Architecture

This is [Cloudflare's official reference architecture for FIPS 140 Level 3 compliance](https://developers.cloudflare.com/reference-architecture/diagrams/security/fips-140-3/).

```
Client → Cloudflare Edge (Regional Services)
              ↓ key operation request
         cloudflared-fips tunnel (carries app traffic + key ops)
              ↓
         Keyless SSL Module
              ↓ PKCS#11
         Customer HSM (FIPS 140-2 Level 3)
```

### Why this architecture matters

In standard Cloudflare TLS, Cloudflare holds your private keys on their edge servers. With Keyless SSL:

1. You upload **only the public certificate** to Cloudflare
2. The **private key stays in your HSM** (FIPS 140-2 Level 3)
3. During every TLS handshake, Cloudflare sends the key operation through the **cloudflared tunnel** to the Keyless SSL module
4. The Keyless module forwards to the HSM via **PKCS#11**
5. The HSM performs the signing operation and returns the result
6. The signed result flows back through the tunnel to Cloudflare

**The tunnel carries both application traffic AND cryptographic key operations.** This is why a FIPS-validated tunnel binary is critical — it protects the key operation channel.

### Prerequisites

- Cloudflare Enterprise plan (Keyless SSL and Regional Services require Enterprise)
- cloudflared-fips installed and running
- A FIPS 140-2 Level 3 HSM (see supported HSMs below)
- The Keyless SSL module (`gokeyless`) deployed alongside cloudflared

### Step 1: Enable Regional Services

Regional Services restricts TLS processing to compliant data centers.

1. In the Cloudflare dashboard, go to **Account > Configurations > Data Localization**
2. Select the region (e.g., "US" for FedRAMP)
3. Apply to the relevant zones

Or via API:
```bash
curl -X PATCH "https://api.cloudflare.com/client/v4/zones/{zone_id}/cache/regional_tiered_cache" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"value": "on"}'
```

### Step 2: Deploy and configure the HSM

#### AWS CloudHSM

```bash
# Create a CloudHSM cluster in your GovCloud VPC
aws cloudhsmv2 create-cluster \
  --hsm-type hsm1.medium \
  --subnet-ids subnet-xxxx \
  --region us-gov-west-1

# Initialize and activate the cluster
# Generate a key pair on the HSM
# Export the public key for Cloudflare
```

Configuration for gokeyless:
```yaml
# /etc/keyless/gokeyless.yaml
private_key_stores:
  - uri: "pkcs11:module-path=/opt/cloudhsm/lib/libcloudhsm_pkcs11.so;pin-value=<CU_user>:<password>"
```

#### Azure Dedicated HSM

```bash
# Create a Dedicated HSM in Azure Government
az dedicated-hsm create \
  --resource-group myHSMGroup \
  --name myDedicatedHSM \
  --location usgovvirginia \
  --sku SafeNet-Luna-Network-HSM-A790
```

Configuration:
```yaml
private_key_stores:
  - uri: "pkcs11:module-path=/usr/safenet/lunaclient/lib/libCryptoki2_64.so;pin-value=<partition_password>"
```

#### Azure Managed HSM

```bash
az keyvault create --hsm-name myManagedHSM \
  --resource-group myHSMGroup \
  --location usgovvirginia \
  --administrators <object-id>
```

#### Entrust nShield Connect

Configuration:
```yaml
private_key_stores:
  - uri: "pkcs11:module-path=/opt/nfast/toolkits/pkcs11/libcknfast.so;pin-value=<module_protection_password>"
```

#### Fortanix Data Security Manager

Configuration:
```yaml
private_key_stores:
  - uri: "pkcs11:module-path=/opt/fortanix/pkcs11/libpkcs11-sdkms.so;pin-value=<api_key>"
```

#### Google Cloud HSM

```bash
# Create a key ring and key in Cloud HSM
gcloud kms keyrings create my-keyring \
  --location us \
  --project my-govcloud-project

gcloud kms keys create my-tls-key \
  --keyring my-keyring \
  --location us \
  --purpose asymmetric-signing \
  --default-algorithm rsa-sign-pkcs1-2048-sha256 \
  --protection-level hsm
```

Configuration:
```yaml
private_key_stores:
  - uri: "pkcs11:module-path=/usr/lib/x86_64-linux-gnu/libkmsp11.so"
```

#### IBM Cloud HSM

Configuration:
```yaml
private_key_stores:
  - uri: "pkcs11:module-path=/usr/lib/pkcs11/PKCS11_API.so;pin-value=<hsm_password>"
```

#### SoftHSMv2 (development/testing only)

**Not for production.** SoftHSMv2 is a software HSM for development and testing.

```bash
# Install SoftHSMv2
apt-get install softhsm2

# Initialize a token
softhsm2-util --init-token --slot 0 --label "keyless-test" --pin 1234 --so-pin 5678

# Generate a key pair
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --keypairgen --key-type rsa:2048 \
  --id 01 --label "tls-key"
```

Configuration:
```yaml
private_key_stores:
  - uri: "pkcs11:module-path=/usr/lib/softhsm/libsofthsm2.so;pin-value=1234"
```

### Step 3: Deploy the Keyless SSL module

The Keyless SSL module (`gokeyless`) is a software proxy between Cloudflare and your HSM.

```bash
# Install gokeyless
# See: https://developers.cloudflare.com/ssl/keyless-ssl/

# Configure with your HSM's PKCS#11 module
cat > /etc/keyless/gokeyless.yaml <<EOF
hostname: keyless.example.com
port: 2407

# HSM PKCS#11 configuration (choose your HSM above)
private_key_stores:
  - uri: "pkcs11:module-path=/path/to/pkcs11/module.so;pin-value=<pin>"

# TLS to Cloudflare origin pull certificate
cloudflare_origin_cert: /etc/keyless/origin-pull-ca.pem
cloudflare_origin_key: /etc/keyless/origin-pull-key.pem
EOF

# Start the keyless module
gokeyless -config /etc/keyless/gokeyless.yaml
```

### Step 4: Configure Keyless SSL in Cloudflare

```bash
# Upload your certificate (public key only — private key stays in HSM)
curl -X POST "https://api.cloudflare.com/client/v4/zones/{zone_id}/keyless_certificates" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "host": "keyless.example.com",
    "port": 2407,
    "certificate": "<PEM-encoded certificate>",
    "name": "Production HSM Key"
  }'
```

### Step 5: Configure cloudflared-fips

```yaml
# cloudflared-fips.yaml
deployment_tier: regional_keyless

tunnel: <TUNNEL_UUID>
credentials-file: /etc/cloudflared/credentials.json

# The tunnel carries both app traffic and Keyless SSL key operations
protocol: quic

fips:
  self-test-on-start: true
  fail-on-self-test-failure: true
```

### Step 6: Verify

```bash
# Start the dashboard
cloudflared-fips dashboard \
  --config cloudflared-fips.yaml \
  --cf-api-token $CF_API_TOKEN \
  --cf-zone-id $CF_ZONE_ID \
  --deployment-tier regional_keyless

# Check Keyless SSL status (ce-10) and Regional Services (ce-11) in dashboard
# Both should show "Pass" when properly configured
```

### Security considerations

- **Tunnel is a critical path:** A compromised tunnel binary could intercept Keyless SSL key operations. This is why FIPS validation of the tunnel binary matters.
- **HSM network isolation:** The HSM should be in a private subnet accessible only from the Keyless SSL module. Do not expose it to the internet.
- **Key operation latency:** Each TLS handshake requires a round trip through the tunnel to the HSM. Expect ~5-15ms additional latency per handshake.
- **HSM availability:** HSM failure = no new TLS handshakes. Use HSM clustering for high availability.

---

## Tier 3: Self-Hosted FIPS Edge Proxy

```
Client → FIPS Proxy (GovCloud) → [optional: Cloudflare WAF/DDoS] → cloudflared-fips → Origin
```

### Setup

1. Build the FIPS proxy: `docker build -f build/Dockerfile.fips-proxy -t fips-proxy .`
2. Deploy in GovCloud (AWS GovCloud, Azure Government, or Google Assured Workloads)
3. Configure TLS certificate and upstream

```bash
docker run -p 443:443 -p 8081:8081 \
  -v /path/to/cert.pem:/etc/ssl/proxy.crt:ro \
  -v /path/to/key.pem:/etc/ssl/proxy.key:ro \
  fips-proxy:latest \
  --upstream http://cloudflared:8080
```

### What you control

- **Everything.** Client TLS terminates at your FIPS proxy with a validated module (BoringCrypto). Full ClientHello inspection and JA4 fingerprinting built in.
- Optionally use Cloudflare on the backend for WAF/DDoS protection, but client TLS is your proxy's responsibility.

### Configuration

```yaml
# cloudflared-fips.yaml
deployment_tier: self_hosted

fips:
  self-test-on-start: true
  fail-on-self-test-failure: true
```

---

## PKCS#11 Reference

PKCS#11 (Public-Key Cryptography Standards #11) is the standard API for HSM communication. The Keyless SSL module uses PKCS#11 to:

1. **Sign** TLS handshake data with the private key stored in the HSM
2. **Decrypt** pre-master secrets (for RSA key exchange)
3. **Perform ECDH** key agreement operations

### PKCS#11 URI format

```
pkcs11:module-path=/path/to/pkcs11/module.so;pin-value=<pin>[;token=<label>][;id=<key-id>]
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `module-path` | Yes | Path to the PKCS#11 shared library (.so) |
| `pin-value` | Yes | PIN or password for the HSM token |
| `token` | No | Token label (if multiple tokens) |
| `id` | No | Key identifier (hex-encoded) |
| `slot-id` | No | Slot number (if multiple slots) |

### Supported PKCS#11 modules

| HSM | PKCS#11 Library Path | FIPS Level |
|-----|---------------------|------------|
| AWS CloudHSM | `/opt/cloudhsm/lib/libcloudhsm_pkcs11.so` | Level 3 |
| Azure Dedicated HSM (SafeNet Luna) | `/usr/safenet/lunaclient/lib/libCryptoki2_64.so` | Level 3 |
| Entrust nShield Connect | `/opt/nfast/toolkits/pkcs11/libcknfast.so` | Level 3 |
| Fortanix SDKMS | `/opt/fortanix/pkcs11/libpkcs11-sdkms.so` | Level 3 |
| Google Cloud HSM | `/usr/lib/x86_64-linux-gnu/libkmsp11.so` | Level 3 |
| IBM Cloud HSM | `/usr/lib/pkcs11/PKCS11_API.so` | Level 3 |
| SoftHSMv2 (dev only) | `/usr/lib/softhsm/libsofthsm2.so` | None |

### Testing with SoftHSMv2

For development and CI testing without hardware HSMs:

```bash
# Install
apt-get install -y softhsm2 opensc

# Initialize token
softhsm2-util --init-token --slot 0 --label "test" --pin 1234 --so-pin 5678

# Generate RSA key
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --keypairgen --key-type rsa:2048 \
  --id 01 --label "tls-key"

# Generate ECDSA P-256 key
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --keypairgen --key-type EC:secp256r1 \
  --id 02 --label "tls-ecdsa-key"

# Verify
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --list-objects
```

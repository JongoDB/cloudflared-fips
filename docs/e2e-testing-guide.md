# End-to-End Testing Guide

This guide covers wiring cloudflared-fips to a real Cloudflare tunnel for live compliance testing.

## Prerequisites

### Cloudflare Account

- A Cloudflare account with at least one zone (domain)
- A Cloudflare Tunnel created via `cloudflared tunnel create <name>`
- Tunnel credentials file (JSON) stored locally
- DNS route configured for the tunnel (`cloudflared tunnel route dns <tunnel-name> <hostname>`)

### API Token

Create a Cloudflare API token at https://dash.cloudflare.com/profile/api-tokens with these permissions:

| Permission | Scope | Required For |
|-----------|-------|-------------|
| Zone — Zone Settings — Read | Specific zone | Cipher suite, TLS version, HSTS checks |
| Zone — SSL and Certificates — Read | Specific zone | Edge certificate checks |
| Zone — Access: Apps and Policies — Read | Specific zone | Access policy, IdP, MFA checks |
| Account — Cloudflare Tunnel — Read | Specific account | Tunnel health, status checks |
| Account — Access: Apps and Policies — Read | Specific account | Account-level Access checks |

### IDs

Gather these values from the Cloudflare dashboard or CLI:

```bash
# Zone ID — visible on the zone overview page
export ZONE_ID="your-zone-id"

# Account ID — visible in the URL when logged into the dashboard
export ACCOUNT_ID="your-account-id"

# Tunnel ID — from tunnel creation or `cloudflared tunnel list`
export TUNNEL_ID="your-tunnel-id"

# API token — from the token creation step
export CF_API_TOKEN="your-api-token"
```

### Origin Service

A local HTTP(S) service for cloudflared to proxy to. For basic testing:

```bash
# Simple HTTP server on port 8080
python3 -m http.server 8080
```

For TLS testing (Segment 3 compliance):

```bash
# Generate a self-signed cert
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes \
  -subj "/CN=localhost"

# Run a TLS server (Python 3)
python3 -c "
import http.server, ssl
server = http.server.HTTPServer(('localhost', 8443), http.server.SimpleHTTPRequestHandler)
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain('cert.pem', 'key.pem')
server.socket = ctx.wrap_socket(server.socket, server_side=True)
server.serve_forever()
"
```

### Software

- Go 1.24+ (for building the dashboard backend)
- Node.js 18+ (for the React dashboard)
- `cloudflared` installed and authenticated

## Step 1: Configure the Tunnel

Edit `configs/cloudflared-fips.yaml` with your real tunnel credentials:

```yaml
tunnel: <your-tunnel-id>
credentials-file: /path/to/tunnel-credentials.json

ingress:
  - hostname: your-hostname.example.com
    service: http://localhost:8080
  - service: http_status:404

# FIPS-specific settings
fips:
  deployment_tier: standard
  verify_signature: false
  backend: auto
```

## Step 2: Start the Cloudflare Tunnel

In a separate terminal:

```bash
cloudflared tunnel --config configs/cloudflared-fips.yaml run
```

Verify it connects:

```bash
cloudflared tunnel info <tunnel-name>
```

The tunnel exposes a metrics endpoint at `http://localhost:20241/metrics` by default. Verify:

```bash
curl -s http://localhost:20241/metrics | head -20
```

## Step 3: Start the Go Dashboard Backend

```bash
go run ./cmd/dashboard \
  --cf-api-token "$CF_API_TOKEN" \
  --cf-zone-id "$ZONE_ID" \
  --cf-account-id "$ACCOUNT_ID" \
  --cf-tunnel-id "$TUNNEL_ID" \
  --metrics-addr http://localhost:20241 \
  --ingress-targets http://localhost:8080 \
  --config configs/cloudflared-fips.yaml
```

The backend starts on `http://127.0.0.1:8080` and serves the API + SSE endpoints.

For TLS-enabled origin testing:

```bash
go run ./cmd/dashboard \
  --cf-api-token "$CF_API_TOKEN" \
  --cf-zone-id "$ZONE_ID" \
  --cf-account-id "$ACCOUNT_ID" \
  --cf-tunnel-id "$TUNNEL_ID" \
  --metrics-addr http://localhost:20241 \
  --ingress-targets https://localhost:8443 \
  --config configs/cloudflared-fips.yaml
```

## Step 4: Start the React Dashboard

In another terminal:

```bash
cd dashboard
npm run dev
```

Opens at http://localhost:5173. The Vite dev server proxies API requests to the Go backend.

## Step 5: Verify Live Data

1. Toggle **Live** in the dashboard header to enable SSE streaming
2. The connection status indicator should show "Connected" (green)
3. Watch for data updates — items should transition from mock data to real values

## Test Matrix

### Tunnel Section (Segment 2)

| Check | How to Verify | Expected Result |
|-------|--------------|-----------------|
| BoringCrypto Active | Build with `GOEXPERIMENT=boringcrypto` on Linux | Pass (shows cert number) |
| OS FIPS Mode | Run on RHEL with `fips=1` boot param | Pass (reads `/proc/sys/crypto/fips_enabled`) |
| FIPS Self-Test | Runs at startup automatically | Pass (KATs + cipher validation) |
| Tunnel Protocol | Check cloudflared logs for QUIC/HTTP2 | Shows actual protocol |
| TLS Version | Metrics endpoint reports negotiated TLS | 1.2 or 1.3 |
| Cipher Suite | Metrics endpoint reports negotiated cipher | FIPS-approved AES-GCM |
| Tunnel Authenticated | cloudflared connects to edge | Pass |
| Tunnel Redundancy | cloudflared opens 4 connections by default | Shows connection count |
| Binary Integrity | SHA-256 of running binary vs manifest | Pass if manifest matches |

### Edge Section (Cloudflare API)

| Check | How to Verify | Expected Result |
|-------|--------------|-----------------|
| Access Policy Active | Configure an Access app for the hostname | Pass (shows app name) |
| Cipher Suite Restriction | Set zone cipher suites via API/dashboard | Pass if FIPS-only ciphers |
| Min TLS Version | Set zone minimum TLS to 1.2 | Pass |
| HSTS Enforced | Enable HSTS in zone settings | Pass |
| Edge Certificate Valid | Zone has active certificate | Pass (shows expiry) |
| Tunnel Health | Tunnel is running and connected | Pass (shows status) |

### Local Service Section (Segment 3)

| Check | How to Verify | Expected Result |
|-------|--------------|-----------------|
| Service Connection Type | Loopback (localhost) or networked | Shows actual type |
| TLS Enabled | Use `https://` ingress target | Pass for HTTPS origins |
| Cipher Suite FIPS Approved | TLS probe against origin | Shows negotiated cipher |
| Service Reachable | Origin is running | Pass |

### Client Section (Segment 1)

| Check | How to Verify | Expected Result |
|-------|--------------|-----------------|
| Client OS FIPS Mode | Connect from a FIPS-enabled OS | Detected via ClientHello |
| Browser TLS Capabilities | Inspect ClientHello cipher list | Shows available ciphers |
| Negotiated Cipher Suite | Check server-side TLS metadata | Shows actual cipher used |

## Troubleshooting

### Dashboard shows all mock data

- Verify the Go backend is running and reachable
- Check the Vite proxy configuration in `dashboard/vite.config.ts`
- Ensure `CF_API_TOKEN` has correct permissions

### Cloudflare API checks fail

- Verify the API token hasn't expired
- Check that zone ID and account ID are correct
- Look at Go backend logs for API error messages
- Rate limiting: the client caches results for 60s by default

### Tunnel checks show unknown

- Confirm cloudflared is running and the metrics endpoint is accessible
- Default metrics address is `http://localhost:20241`
- Try `curl http://localhost:20241/metrics` to verify

### SSE connection drops

- The dashboard auto-reconnects with exponential backoff
- Check browser developer tools Network tab for SSE connection status
- Verify the Go backend SSE endpoint is responsive: `curl -N http://127.0.0.1:8080/api/v1/compliance/stream`

### OS FIPS mode shows disabled

- Linux: verify with `cat /proc/sys/crypto/fips_enabled` (should be `1`)
- RHEL: `fips-mode-setup --check`
- The self-test binary must run on the FIPS-enabled host, not in a non-FIPS container

## Advanced Testing

### Test FIPS Self-Test Failures

Force a self-test failure to verify fail-closed behavior:

```bash
# Run standalone self-test with a bad manifest path
go run ./cmd/selftest --manifest-path /nonexistent/manifest.json
```

### Test Tier 2 (Keyless SSL)

Requires Keyless SSL configured with an HSM. See `docs/deployment-tier-guide.md` for HSM setup instructions.

### Test Tier 3 (Self-Hosted Proxy)

Deploy the FIPS proxy in a GovCloud environment:

```bash
# Build the proxy
docker build -f build/Dockerfile.fips-proxy -t fips-proxy .

# Run locally for testing
docker run -p 443:443 fips-proxy
```

See `deploy/terraform/main.tf` for AWS GovCloud ECS Fargate deployment.

### Test Client FIPS Detection

Connect from different client configurations and observe the Client Posture section:

1. **Windows FIPS mode**: Enable via GPO, connect with Edge/Chrome
2. **RHEL FIPS mode**: Boot with `fips=1`, connect with Firefox
3. **macOS**: Connect normally (CommonCrypto always active)
4. **Non-FIPS client**: Connect from a standard browser — should show warning items

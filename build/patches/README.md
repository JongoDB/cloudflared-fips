# Patches Directory

Place `.patch` files in this directory to apply modifications to cloudflared source during the FIPS build.

## Purpose

Patches can be used to:

- Restrict cipher suites to FIPS-approved-only
- Modify TLS configuration defaults
- Apply security hardening
- Fix compatibility issues with specific Go/BoringCrypto versions

## Usage

Patches are applied in alphabetical order during the Docker build. Name them with a numeric prefix for ordering:

```
001-restrict-ciphers.patch
002-enforce-tls12-minimum.patch
```

## Creating Patches

```bash
# From the cloudflared source directory:
git diff > ../build/patches/001-my-change.patch
```

Patches must apply cleanly against the cloudflared tag specified in `Dockerfile.fips`.

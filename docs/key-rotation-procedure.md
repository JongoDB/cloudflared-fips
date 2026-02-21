# Key Rotation Procedure

## Artifact Signing Key Rotation

This document describes the procedure for rotating the GPG and cosign signing keys used for cloudflared-fips build artifacts.

---

### 1. Overview

| Key Type | Purpose | Storage | Rotation Frequency |
|----------|---------|---------|-------------------|
| **GPG signing key** | Signs binaries, RPMs, DEBs, macOS .pkg | GitHub Actions secret `GPG_PRIVATE_KEY` | Annually or on compromise |
| **cosign (Sigstore)** | Signs container images | Keyless (OIDC identity) | Automatic (ephemeral) |

### 2. GPG Key Rotation

#### 2.1 Generate a new signing key

```bash
# Generate a new 4096-bit RSA key (no expiry, or set expiry per policy)
gpg --batch --gen-key <<EOF
Key-Type: RSA
Key-Length: 4096
Subkey-Type: RSA
Subkey-Length: 4096
Name-Real: cloudflared-fips Release Signing
Name-Email: security@example.com
Expire-Date: 1y
%no-protection
%commit
EOF

# List keys to find the new key ID
gpg --list-keys --keyid-format long
```

#### 2.2 Export the new key

```bash
# Export private key (for GitHub Actions secret)
gpg --armor --export-secret-keys <NEW_KEY_ID> > private-key.asc

# Export public key (for distribution)
gpg --armor --export <NEW_KEY_ID> > public-key.asc
```

#### 2.3 Update GitHub Actions secrets

1. Go to **Settings > Secrets and variables > Actions** in the GitHub repository
2. Update `GPG_PRIVATE_KEY` with the contents of `private-key.asc`
3. Update `GPG_SIGNING_KEY_ID` with the new key ID

#### 2.4 Publish the new public key

1. Upload `public-key.asc` to the project's releases page
2. Update the public key URL in `pkg/signing/signing.go` (the `PublicKey` field of `SignatureManifest`)
3. Add the new public key to the project's keyserver entries (if applicable)

#### 2.5 Transition period

During key rotation, both old and new keys should be valid:

1. Sign one release with **both** old and new keys
2. Announce the key rotation in release notes
3. Provide the new public key fingerprint
4. After the dual-signed release, revoke the old key

```bash
# Sign with both keys during transition
gpg --detach-sign --armor --local-user <OLD_KEY_ID> -o artifact.sig.old artifact
gpg --detach-sign --armor --local-user <NEW_KEY_ID> -o artifact.sig artifact
```

#### 2.6 Revoke the old key

```bash
# Generate revocation certificate
gpg --gen-revoke <OLD_KEY_ID> > revocation.asc

# Import the revocation
gpg --import revocation.asc

# Upload revoked key to keyserver
gpg --send-keys <OLD_KEY_ID>
```

### 3. cosign Key Rotation

cosign uses keyless signing by default (Sigstore/Fulcio), which generates ephemeral keys tied to the CI identity (GitHub Actions OIDC). No manual key rotation is required.

If using a static cosign key (`--key` flag):

```bash
# Generate new cosign key pair
cosign generate-key-pair

# Update the private key in GitHub Actions secrets
# Update the public key in the verification docs
```

### 4. Key Rotation Checklist

- [ ] Generate new GPG key pair (4096-bit RSA)
- [ ] Update GitHub Actions secrets (`GPG_PRIVATE_KEY`, `GPG_SIGNING_KEY_ID`)
- [ ] Publish new public key to releases page
- [ ] Update `SignatureManifest.PublicKey` URL in code
- [ ] Dual-sign at least one transitional release
- [ ] Announce rotation in release notes with new key fingerprint
- [ ] Revoke old key after transition period
- [ ] Update this document with new key details
- [ ] Verify self-test binary signature verification works with new key

### 5. Emergency Key Rotation (Compromise)

If a signing key is compromised:

1. **Immediately** revoke the compromised key
2. Generate a new key pair
3. Update all GitHub Actions secrets
4. Re-sign all artifacts from the most recent release
5. Publish a security advisory listing:
   - Compromised key fingerprint
   - Affected releases (if any)
   - New key fingerprint
   - Verification steps for users
6. Contact CMVP if the compromise affects FIPS validation artifacts

### 6. NIST 800-53 Mapping

| Control | Implementation |
|---------|---------------|
| SC-12 (Cryptographic Key Establishment) | GPG key generation with 4096-bit RSA |
| SC-12(1) (Availability) | Key backup in secure offline storage |
| SC-17 (Public Key Infrastructure Certificates) | GPG public key distribution via releases |
| CM-14 (Signed Components) | All build artifacts signed; self-test verifies at startup |

---

_Document prepared for AO review. Follows key management guidelines from NIST SP 800-57._

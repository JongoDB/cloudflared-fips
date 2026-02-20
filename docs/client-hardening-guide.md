# Client Endpoint Hardening Guide

## FIPS Mode Configuration for Client Endpoints

This guide provides step-by-step instructions for enabling FIPS-compliant
cryptography on client endpoints that connect to cloudflared-fips protected
services through Cloudflare's network.

---

## Windows

### Enable FIPS Mode via Group Policy (GPO)

1. Open **Group Policy Editor** (`gpedit.msc`) or create a new GPO in Active Directory
2. Navigate to: `Computer Configuration → Windows Settings → Security Settings → Local Policies → Security Options`
3. Enable: **"System cryptography: Use FIPS compliant algorithms for encryption, hashing, and signing"**
4. Reboot the system

**Effect:** Forces Windows CNG (Cryptography Next Generation) as the crypto provider
for all applications. This includes:
- Chrome and Edge (use Windows CNG automatically)
- .NET applications
- PowerShell remoting
- RDP connections

**CMVP Certificate:** Windows CNG — #2357 (among others, varies by Windows version)

### Verification

```powershell
# Check FIPS mode via registry
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy" -Name "Enabled"
# Expected: Enabled = 1

# Check via PowerShell
[System.Security.Cryptography.CryptoConfig]::AllowOnlyFipsAlgorithms
# Expected: True
```

### Firefox on Windows — NSS FIPS Mode

Firefox uses NSS (Network Security Services), not Windows CNG. It requires
separate FIPS mode configuration:

1. Open Firefox, navigate to `about:config`
2. Search for `security.OCSP.enabled` — set to `1`
3. Open **Options → Privacy & Security → Certificates → Security Devices**
4. Select **NSS Internal PKCS #11 Module**
5. Click **Enable FIPS**
6. Restart Firefox

**Verification:** Navigate to `about:config` and check `security.enterprise_roots.enabled` = `true`

### MDM Policy Template (Intune)

```json
{
  "name": "FIPS Compliance Policy",
  "description": "Enforces FIPS mode on Windows endpoints",
  "settings": [
    {
      "settingDefinition": "device_configuration_system_cryptography_use_fips",
      "value": "enabled"
    },
    {
      "settingDefinition": "device_configuration_bitlocker_encryption",
      "value": "required"
    },
    {
      "settingDefinition": "device_configuration_minimum_os_version",
      "value": "10.0.19044"
    }
  ]
}
```

---

## RHEL / CentOS / Fedora (Linux)

### Enable FIPS Mode

#### During Installation (Recommended)
Add `fips=1` to the kernel boot parameters during OS installation.

#### Post-Installation
```bash
# RHEL 8/9
sudo fips-mode-setup --enable
sudo reboot

# Verify
cat /proc/sys/crypto/fips_enabled
# Expected: 1

fips-mode-setup --check
# Expected: "FIPS mode is enabled."
```

**Effect:** Enables FIPS mode system-wide:
- OpenSSL uses only FIPS-validated algorithms
- SSH uses only FIPS-approved ciphers
- All crypto libraries respect the system FIPS policy

**CMVP Certificates:**
- RHEL 8 OpenSSL: #3842
- RHEL 9 OpenSSL: #4349

### Verification

```bash
# Check kernel FIPS mode
cat /proc/sys/crypto/fips_enabled
# Expected: 1

# Check OpenSSL FIPS provider
openssl list -providers
# Should show "fips" provider

# Check available ciphers are FIPS-only
openssl ciphers -v 'FIPS' | head -10

# Verify system crypto policy
update-crypto-policies --show
# Expected: FIPS or FIPS:OSPP
```

### Known Limitation: Chrome on Linux

**Chrome on Linux uses BoringSSL, not the system OpenSSL library.** This means
Chrome's TLS operations do NOT go through the RHEL FIPS-validated OpenSSL module,
even when the system is in FIPS mode.

**Mitigations:**
- Use Firefox with NSS FIPS mode enabled (see below)
- Document this limitation in the SSP
- Cloudflare edge still enforces FIPS cipher suites on its side
- Access device posture can verify browser TLS capabilities

### Firefox on Linux — NSS FIPS Mode

```bash
# Enable FIPS mode in Firefox's NSS database
modutil -fips true -dbdir ~/.mozilla/firefox/*.default-release/

# Verify
modutil -chkfips -dbdir ~/.mozilla/firefox/*.default-release/
# Expected: "FIPS mode enabled."
```

---

## macOS

### FIPS Status

Apple's CommonCrypto / Secure Transport maintains FIPS 140-2 validation
(certificate #3856 among others) and is **always active** — no explicit FIPS
mode switch is required.

**CMVP Certificate:** Apple CoreCrypto — #3856+ (varies by macOS version)

This makes macOS acceptable for:
- Development environments
- Potentially compliant client deployments (verify with AO)

### Verification

```bash
# Check macOS version (determines which CoreCrypto cert applies)
sw_vers

# Check available TLS cipher suites
nscurl --ats-diagnostics https://your-service.example.com 2>/dev/null | head -20

# Verify system uses Secure Transport
security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain | head -5
```

### Note on Homebrew OpenSSL

If Homebrew is installed, applications may use Homebrew's OpenSSL instead of
Apple's Secure Transport. Homebrew OpenSSL is NOT FIPS-validated.

```bash
# Check if Homebrew OpenSSL is installed
brew list openssl 2>/dev/null && echo "WARNING: Homebrew OpenSSL found"

# Verify which OpenSSL Python/Ruby use
python3 -c "import ssl; print(ssl.OPENSSL_VERSION)"
```

---

## MDM Policy Templates

### Intune (Windows)

Deploy FIPS enforcement via Intune compliance policy:

1. **Endpoint Manager → Devices → Compliance Policies → Create Policy**
2. Add system configuration requirement for FIPS mode
3. Set non-compliant action: **Block access** (integrates with Cloudflare Access device posture)

### Jamf (macOS)

Deploy verification via Jamf smart group:

1. **Smart Computer Group**: macOS version >= 13.0
2. **Extension Attribute**: Verify CoreCrypto validation status
3. **Compliance Policy**: Report to Cloudflare Access device posture API

### Cloudflare Access Device Posture Integration

Configure Access device posture checks that verify FIPS mode:

1. **Zero Trust Dashboard → Settings → WARP Client → Device Posture**
2. Add check: **OS Version** (minimum versions with current FIPS validation)
3. Add check: **Disk Encryption** (BitLocker/FileVault)
4. Add check: **Domain Joined** (for GPO-managed Windows endpoints)
5. Create Access policy requiring all posture checks to pass

---

## Summary Table

| OS | FIPS Mode | How to Enable | CMVP Cert | Browser Notes |
|----|-----------|---------------|-----------|---------------|
| Windows 10/11 | GPO setting | Group Policy | CNG #2357+ | Chrome/Edge use CNG; Firefox needs NSS config |
| RHEL 8 | Kernel param | `fips-mode-setup --enable` | OpenSSL #3842 | Chrome uses BoringSSL (gap); use Firefox |
| RHEL 9 | Kernel param | `fips-mode-setup --enable` | OpenSSL #4349 | Chrome uses BoringSSL (gap); use Firefox |
| macOS 13+ | Always active | N/A | CoreCrypto #3856+ | Safari uses Secure Transport; Chrome uses BoringSSL |

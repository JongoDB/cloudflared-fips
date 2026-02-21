%define _name cloudflared-fips
%define _version %{getenv:VERSION}
%define _release 1

Name:           %{_name}
Version:        %{_version}
Release:        %{_release}%{?dist}
Summary:        FIPS 140-2 compliant Cloudflare Tunnel client

License:        Apache-2.0
URL:            https://github.com/cloudflared-fips/cloudflared-fips
Source0:        %{_name}-%{_version}.tar.gz

Requires:       openssl-libs >= 3.0
# RHEL/CentOS/Alma/Rocky 9+ for FIPS-validated OpenSSL
Requires:       crypto-policies

%description
FIPS 140-2 compliant build of Cloudflare Tunnel (cloudflared) using
BoringCrypto (CMVP #4407). Includes runtime self-test suite, compliance
dashboard, and build manifest with full cryptographic provenance.

This package:
- Installs the FIPS-validated cloudflared binary
- Runs FIPS self-tests on install (post-install verification)
- Installs sample configuration and build manifest
- Creates a systemd service unit for automatic startup

%prep
# No source extraction — binary is pre-built in CI

%install
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/etc/cloudflared
mkdir -p %{buildroot}/usr/lib/systemd/system
mkdir -p %{buildroot}/usr/share/cloudflared-fips

# Binary
install -m 0755 %{_sourcedir}/cloudflared %{buildroot}/usr/local/bin/cloudflared

# Self-test binary
install -m 0755 %{_sourcedir}/selftest %{buildroot}/usr/local/bin/cloudflared-selftest

# Config and manifest
install -m 0644 %{_sourcedir}/cloudflared-fips.yaml %{buildroot}/etc/cloudflared/config.yaml.sample
install -m 0644 %{_sourcedir}/build-manifest.json %{buildroot}/usr/share/cloudflared-fips/build-manifest.json

# Systemd unit
cat > %{buildroot}/usr/lib/systemd/system/cloudflared.service <<'UNIT'
[Unit]
Description=Cloudflare Tunnel (FIPS)
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
ExecStartPre=/usr/local/bin/cloudflared-selftest
ExecStart=/usr/local/bin/cloudflared tunnel run
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
Environment=GODEBUG=fips140=on

[Install]
WantedBy=multi-user.target
UNIT

%post
echo "Running FIPS self-test..."
/usr/local/bin/cloudflared-selftest || {
    echo "WARNING: FIPS self-test failed. Check /usr/local/bin/cloudflared-selftest output."
    echo "The binary may not be FIPS-compliant on this system."
}

# Reload systemd
systemctl daemon-reload 2>/dev/null || true

echo ""
echo "cloudflared-fips installed successfully."
echo "  Config sample: /etc/cloudflared/config.yaml.sample"
echo "  Build manifest: /usr/share/cloudflared-fips/build-manifest.json"
echo "  Systemd unit: systemctl enable --now cloudflared"

%preun
if [ "$1" = "0" ]; then
    # Full uninstall (not upgrade)
    systemctl stop cloudflared 2>/dev/null || true
    systemctl disable cloudflared 2>/dev/null || true
fi

%postun
systemctl daemon-reload 2>/dev/null || true

%files
/usr/local/bin/cloudflared
/usr/local/bin/cloudflared-selftest
%config(noreplace) /etc/cloudflared/config.yaml.sample
/usr/share/cloudflared-fips/build-manifest.json
/usr/lib/systemd/system/cloudflared.service

%changelog
* Fri Feb 21 2026 cloudflared-fips maintainers
- Initial FIPS 140-2 compliant package

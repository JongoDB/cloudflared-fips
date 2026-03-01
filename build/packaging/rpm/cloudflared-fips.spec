%define _name cloudflared-fips
%define _version %{getenv:VERSION}
%define _release 1

Name:           %{_name}
Version:        %{_version}
Release:        %{_release}%{?dist}
Summary:        FIPS 140-3 compliant Cloudflare Tunnel client with fleet management

License:        Apache-2.0
URL:            https://github.com/cloudflared-fips/cloudflared-fips
Source0:        %{_name}-%{_version}.tar.gz

Requires:       openssl-libs >= 3.0
# RHEL/CentOS/Alma/Rocky 9+ for FIPS-validated OpenSSL
Requires:       crypto-policies

%description
FIPS 140-3 compliant build of Cloudflare Tunnel (cloudflared) using
BoringCrypto (CMVP #4735). Ships all fleet binaries for zero-trust
network fabric deployment with 4 roles: controller, server, proxy, client.

Included binaries:
  cloudflared-fips-selftest   — FIPS self-test suite (KATs, cipher validation)
  cloudflared-fips-dashboard  — Compliance dashboard + fleet controller API
  cloudflared-fips-tui        — Interactive setup wizard + live status monitor
  cloudflared-fips-proxy      — Tier 3 self-hosted FIPS edge proxy
  cloudflared-fips-agent      — Lightweight endpoint FIPS posture agent
  cloudflared-fips-provision  — Multi-role provisioning script

Role selection happens at provision time via the TUI wizard or provision script.

%prep
# No source extraction — binaries are pre-built in CI

%install
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/etc/cloudflared
mkdir -p %{buildroot}/usr/share/cloudflared-fips

# Fleet binaries
install -m 0755 %{_sourcedir}/cloudflared-fips-selftest %{buildroot}/usr/local/bin/cloudflared-fips-selftest
install -m 0755 %{_sourcedir}/cloudflared-fips-dashboard %{buildroot}/usr/local/bin/cloudflared-fips-dashboard
install -m 0755 %{_sourcedir}/cloudflared-fips-tui %{buildroot}/usr/local/bin/cloudflared-fips-tui
install -m 0755 %{_sourcedir}/cloudflared-fips-proxy %{buildroot}/usr/local/bin/cloudflared-fips-proxy
install -m 0755 %{_sourcedir}/cloudflared-fips-agent %{buildroot}/usr/local/bin/cloudflared-fips-agent

# Provision script
install -m 0755 %{_sourcedir}/cloudflared-fips-provision %{buildroot}/usr/local/bin/cloudflared-fips-provision

# Config and manifest
install -m 0644 %{_sourcedir}/cloudflared-fips.yaml %{buildroot}/etc/cloudflared/config.yaml.sample
install -m 0644 %{_sourcedir}/build-manifest.json %{buildroot}/usr/share/cloudflared-fips/build-manifest.json

%post
echo "Running FIPS self-test..."
/usr/local/bin/cloudflared-fips-selftest || {
    echo "WARNING: FIPS self-test failed. Check /usr/local/bin/cloudflared-fips-selftest output."
    echo "The binary may not be FIPS-compliant on this system."
}

echo ""
echo "cloudflared-fips installed successfully."
echo ""
echo "  Binaries installed to /usr/local/bin/cloudflared-fips-*"
echo "  Config sample:    /etc/cloudflared/config.yaml.sample"
echo "  Build manifest:   /usr/share/cloudflared-fips/build-manifest.json"
echo ""
echo "  Next steps — choose one:"
echo "    cloudflared-fips-tui setup                              # interactive wizard"
echo "    cloudflared-fips-provision --role controller             # fleet controller"
echo "    cloudflared-fips-provision --role server --enrollment-token <T> --controller-url <URL>"
echo "    cloudflared-fips-provision --role proxy  --enrollment-token <T> --controller-url <URL>"
echo "    cloudflared-fips-provision --role client --enrollment-token <T> --controller-url <URL>"

%preun
if [ "$1" = "0" ]; then
    # Full uninstall (not upgrade) — stop all fleet services
    systemctl stop cloudflared-fips.target 2>/dev/null || true
    systemctl disable cloudflared-fips.target 2>/dev/null || true
    for svc in dashboard tunnel proxy agent; do
        systemctl stop "cloudflared-fips-${svc}.service" 2>/dev/null || true
        systemctl disable "cloudflared-fips-${svc}.service" 2>/dev/null || true
    done
fi

%postun
systemctl daemon-reload 2>/dev/null || true

%files
/usr/local/bin/cloudflared-fips-selftest
/usr/local/bin/cloudflared-fips-dashboard
/usr/local/bin/cloudflared-fips-tui
/usr/local/bin/cloudflared-fips-proxy
/usr/local/bin/cloudflared-fips-agent
/usr/local/bin/cloudflared-fips-provision
%config(noreplace) /etc/cloudflared/config.yaml.sample
/usr/share/cloudflared-fips/build-manifest.json

%changelog
* Sat Mar 01 2026 cloudflared-fips maintainers
- Ship all fleet binaries (dashboard, tui, proxy, agent) in single RPM
- Add provision script for role-based deployment
- Remove hardcoded systemd unit (provisioning creates role-specific units)

* Fri Feb 21 2026 cloudflared-fips maintainers
- Initial FIPS 140-2 compliant package

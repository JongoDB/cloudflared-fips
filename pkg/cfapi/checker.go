package cfapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// ComplianceChecker uses the Cloudflare API to produce the "Cloudflare Edge"
// compliance section with real data.
type ComplianceChecker struct {
	client    *Client
	zoneID    string
	accountID string
	tunnelID  string
}

// NewComplianceChecker creates a checker for a specific zone and tunnel.
func NewComplianceChecker(client *Client, zoneID, accountID, tunnelID string) *ComplianceChecker {
	return &ComplianceChecker{
		client:    client,
		zoneID:    zoneID,
		accountID: accountID,
		tunnelID:  tunnelID,
	}
}

// RunEdgeChecks produces the "Cloudflare Edge" compliance section from live API data.
func (cc *ComplianceChecker) RunEdgeChecks() compliance.Section {
	section := compliance.Section{
		ID:          "edge",
		Name:        "Cloudflare Edge",
		Description: "Cloudflare Access, Gateway, and edge TLS configuration",
	}

	section.Items = append(section.Items, cc.checkAccessPolicy())
	section.Items = append(section.Items, cc.checkIdentityProvider())
	section.Items = append(section.Items, cc.checkAuthMethod())
	section.Items = append(section.Items, cc.checkMFAEnforced())
	section.Items = append(section.Items, cc.checkCipherRestriction())
	section.Items = append(section.Items, cc.checkMinTLSVersion())
	section.Items = append(section.Items, cc.checkEdgeCertificate())
	section.Items = append(section.Items, cc.checkHSTS())
	section.Items = append(section.Items, cc.checkTunnelHealth())

	return section
}

func (cc *ComplianceChecker) checkAccessPolicy() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-1",
		Name:               "Cloudflare Access Policy",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyAPI,
		What:               "Queries Cloudflare Access API to verify at least one Access application is configured",
		Why:                "Access policies enforce authentication before requests reach the tunnel. Without Access, anyone can reach the origin.",
		Remediation:        "Configure a Cloudflare Access application at dash.cloudflare.com > Zero Trust > Access > Applications.",
		NISTRef:            "AC-3, AC-17",
	}

	apps, err := cc.client.GetAccessApps(cc.zoneID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		item.What = fmt.Sprintf("Cannot query Access API: %s", err)
		return item
	}

	if len(apps) > 0 {
		item.Status = compliance.StatusPass
		item.What = fmt.Sprintf("%d Access application(s) configured", len(apps))
	} else {
		item.Status = compliance.StatusFail
		item.What = "No Access applications found"
	}
	return item
}

func (cc *ComplianceChecker) checkIdentityProvider() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-2",
		Name:               "Identity Provider Connected",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyAPI,
		What:               "Checks if an identity provider is connected to Cloudflare Access",
		Why:                "An IdP is required for user authentication. Without it, Access cannot verify identity.",
		Remediation:        "Configure an IdP (Okta, Azure AD, etc.) in Zero Trust > Settings > Authentication.",
		NISTRef:            "IA-2, IA-8",
	}

	apps, err := cc.client.GetAccessApps(cc.zoneID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		return item
	}

	// If any app exists, an IdP must be configured
	if len(apps) > 0 {
		item.Status = compliance.StatusPass
		item.What = "Identity provider configured (Access apps require IdP)"
	} else {
		item.Status = compliance.StatusUnknown
		item.What = "No Access apps configured; cannot determine IdP status"
	}
	return item
}

func (cc *ComplianceChecker) checkAuthMethod() compliance.ChecklistItem {
	return compliance.ChecklistItem{
		ID:                 "ce-3",
		Name:               "Authentication Method",
		Severity:           "high",
		VerificationMethod: compliance.VerifyAPI,
		Status:             compliance.StatusUnknown,
		What:               "Authentication method (SAML/OIDC) determined by IdP configuration",
		Why:                "The authentication protocol affects the strength of identity verification.",
		Remediation:        "Configure SAML 2.0 or OIDC in the Access IdP settings.",
		NISTRef:            "IA-2, IA-8",
	}
}

func (cc *ComplianceChecker) checkMFAEnforced() compliance.ChecklistItem {
	return compliance.ChecklistItem{
		ID:                 "ce-4",
		Name:               "MFA Enforced",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyAPI,
		Status:             compliance.StatusUnknown,
		What:               "MFA enforcement is configured at the IdP level, not directly queryable via Cloudflare API",
		Why:                "Multi-factor authentication is required by NIST SP 800-63B for government systems.",
		Remediation:        "Enforce MFA in your IdP (Okta, Azure AD). Add an Access policy requiring MFA.",
		NISTRef:            "IA-2(1), IA-2(2)",
	}
}

func (cc *ComplianceChecker) checkCipherRestriction() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-5",
		Name:               "Cipher Suite Restriction",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyInherited,
		What:               "Queries Cloudflare API for the zone's configured cipher suite restrictions",
		Why:                "Edge cipher suites must be restricted to FIPS-approved algorithms. Cloudflare's edge uses BoringSSL but the FIPS cert belongs to Google, not Cloudflare.",
		Remediation:        "Set cipher suites via API: PATCH /zones/{id}/settings/ciphers with FIPS-approved list.",
		NISTRef:            "SC-13, SC-8",
	}

	ciphers, err := cc.client.GetCiphers(cc.zoneID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		item.What = fmt.Sprintf("Cannot query cipher settings: %s", err)
		return item
	}

	if len(ciphers) > 0 {
		// Check if all ciphers are FIPS-approved names
		allFIPS := true
		for _, c := range ciphers {
			if !isFIPSCipherName(c) {
				allFIPS = false
				break
			}
		}
		if allFIPS {
			item.Status = compliance.StatusPass
			item.What = fmt.Sprintf("%d cipher(s) configured, all FIPS-approved", len(ciphers))
		} else {
			item.Status = compliance.StatusWarning
			item.What = fmt.Sprintf("%d cipher(s) configured, some may not be FIPS-approved", len(ciphers))
		}
	} else {
		// Empty means Cloudflare defaults (includes non-FIPS ciphers)
		item.Status = compliance.StatusWarning
		item.What = "No custom cipher restriction; Cloudflare defaults include non-FIPS ciphers"
	}
	return item
}

func (cc *ComplianceChecker) checkMinTLSVersion() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-6",
		Name:               "Minimum TLS Version Enforced",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyAPI,
		What:               "Queries Cloudflare API for the zone's minimum TLS version setting",
		Why:                "FIPS requires TLS 1.2 minimum (NIST SP 800-52 Rev 2). TLS 1.0/1.1 must be disabled.",
		Remediation:        "Set minimum TLS version to 1.2 via API or dashboard: SSL/TLS > Edge Certificates > Minimum TLS Version.",
		NISTRef:            "SC-8, SC-13",
	}

	version, err := cc.client.GetMinTLSVersion(cc.zoneID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		item.What = fmt.Sprintf("Cannot query min TLS version: %s", err)
		return item
	}

	switch version {
	case "1.2", "1.3":
		item.Status = compliance.StatusPass
		item.What = fmt.Sprintf("Minimum TLS version: %s", version)
	case "1.1":
		item.Status = compliance.StatusFail
		item.What = "Minimum TLS version 1.1 — must be 1.2 or higher for FIPS"
	case "1.0":
		item.Status = compliance.StatusFail
		item.What = "Minimum TLS version 1.0 — must be 1.2 or higher for FIPS"
	default:
		item.Status = compliance.StatusWarning
		item.What = fmt.Sprintf("Unknown TLS version: %s", version)
	}
	return item
}

func (cc *ComplianceChecker) checkEdgeCertificate() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-7",
		Name:               "Edge Certificate Valid",
		Severity:           "high",
		VerificationMethod: compliance.VerifyAPI,
		What:               "Queries Cloudflare API for SSL certificate pack status and expiry",
		Why:                "An expired or missing edge certificate will cause TLS failures for all clients.",
		Remediation:        "Check SSL/TLS > Edge Certificates in the Cloudflare dashboard. Renew or re-order if expired.",
		NISTRef:            "SC-12, IA-5",
	}

	packs, err := cc.client.GetCertificatePacks(cc.zoneID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		item.What = fmt.Sprintf("Cannot query certificates: %s", err)
		return item
	}

	if len(packs) == 0 {
		item.Status = compliance.StatusFail
		item.What = "No certificate packs found"
		return item
	}

	for _, pack := range packs {
		if pack.Status == "active" {
			if pack.CertificateExpiry != "" {
				expiry, err := time.Parse(time.RFC3339, pack.CertificateExpiry)
				if err == nil && time.Now().After(expiry) {
					item.Status = compliance.StatusFail
					item.What = fmt.Sprintf("Certificate expired: %s", pack.CertificateExpiry)
					return item
				}
				daysLeft := int(time.Until(expiry).Hours() / 24)
				if daysLeft < 30 {
					item.Status = compliance.StatusWarning
					item.What = fmt.Sprintf("Certificate expires in %d days", daysLeft)
					return item
				}
			}
			item.Status = compliance.StatusPass
			item.What = fmt.Sprintf("Active certificate (%s)", pack.Type)
			return item
		}
	}

	item.Status = compliance.StatusWarning
	item.What = "No active certificate pack found"
	return item
}

func (cc *ComplianceChecker) checkHSTS() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-8",
		Name:               "HSTS Enforced",
		Severity:           "high",
		VerificationMethod: compliance.VerifyAPI,
		What:               "Queries Cloudflare API for HTTP Strict Transport Security settings",
		Why:                "HSTS prevents protocol downgrade attacks. Without it, clients may be tricked into using HTTP.",
		Remediation:        "Enable HSTS in SSL/TLS > Edge Certificates > HTTP Strict Transport Security.",
		NISTRef:            "SC-8, SC-23",
	}

	header, err := cc.client.GetSecurityHeader(cc.zoneID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		item.What = fmt.Sprintf("Cannot query HSTS settings: %s", err)
		return item
	}

	if header.StrictTransportSecurity.Enabled {
		item.Status = compliance.StatusPass
		maxAge := header.StrictTransportSecurity.MaxAge
		item.What = fmt.Sprintf("HSTS enabled (max-age=%d, includeSubdomains=%v, preload=%v)",
			maxAge, header.StrictTransportSecurity.IncludeSubdomains, header.StrictTransportSecurity.Preload)
	} else {
		item.Status = compliance.StatusFail
		item.What = "HSTS is not enabled"
	}
	return item
}

func (cc *ComplianceChecker) checkTunnelHealth() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ce-9",
		Name:               "Tunnel Health",
		Severity:           "high",
		VerificationMethod: compliance.VerifyAPI,
		What:               "Queries Cloudflare API for tunnel connection status and health",
		Why:                "An unhealthy tunnel means traffic cannot reach the origin, regardless of crypto compliance.",
		Remediation:        "Check cloudflared logs. Restart the tunnel: cloudflared tunnel run. Verify network connectivity.",
		NISTRef:            "SC-8, CP-8",
	}

	if cc.accountID == "" || cc.tunnelID == "" {
		item.Status = compliance.StatusUnknown
		item.What = "Account ID or Tunnel ID not configured"
		return item
	}

	tunnel, err := cc.client.GetTunnel(cc.accountID, cc.tunnelID)
	if err != nil {
		item.Status = compliance.StatusUnknown
		item.What = fmt.Sprintf("Cannot query tunnel: %s", err)
		return item
	}

	switch tunnel.Status {
	case "healthy":
		activeConns := 0
		for _, conn := range tunnel.Connections {
			if conn.IsActive {
				activeConns++
			}
		}
		item.Status = compliance.StatusPass
		item.What = fmt.Sprintf("Tunnel healthy: %d active connection(s)", activeConns)
	case "degraded":
		item.Status = compliance.StatusWarning
		item.What = "Tunnel degraded — some connections may be down"
	default:
		item.Status = compliance.StatusFail
		item.What = fmt.Sprintf("Tunnel status: %s", tunnel.Status)
	}
	return item
}

// isFIPSCipherName checks if a cipher suite name matches known FIPS-approved suites.
func isFIPSCipherName(name string) bool {
	name = strings.ToUpper(name)
	fipsCiphers := []string{
		"ECDHE-RSA-AES128-GCM-SHA256",
		"ECDHE-RSA-AES256-GCM-SHA384",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
		"ECDHE-ECDSA-AES256-GCM-SHA384",
		"AES128-GCM-SHA256",
		"AES256-GCM-SHA384",
		"ECDHE-RSA-AES128-SHA256",
		"ECDHE-ECDSA-AES128-SHA256",
		// TLS 1.3 (always FIPS-approved in Cloudflare)
		"TLS_AES_128_GCM_SHA256",
		"TLS_AES_256_GCM_SHA384",
	}
	for _, c := range fipsCiphers {
		if strings.ToUpper(c) == name {
			return true
		}
	}
	return false
}

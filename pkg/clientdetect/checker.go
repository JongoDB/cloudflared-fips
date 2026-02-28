package clientdetect

import (
	"fmt"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// ComplianceChecker produces the "Client Posture" section from TLS inspection
// and device posture data.
type ComplianceChecker struct {
	inspector *Inspector
	posture   *PostureCollector
}

// NewComplianceChecker creates a client posture compliance checker.
func NewComplianceChecker(inspector *Inspector, posture *PostureCollector) *ComplianceChecker {
	return &ComplianceChecker{
		inspector: inspector,
		posture:   posture,
	}
}

// RunClientPostureChecks produces the "Client Posture" compliance section.
func (cc *ComplianceChecker) RunClientPostureChecks() compliance.Section {
	section := compliance.Section{
		ID:          "client",
		Name:        "Client Posture",
		Description: "Segment 1 \u2014 Client OS/Browser to Cloudflare Edge (HTTPS)",
	}

	section.Items = append(section.Items, cc.checkClientOSFIPS())
	section.Items = append(section.Items, cc.checkClientOSType())
	section.Items = append(section.Items, cc.checkBrowserTLS())
	section.Items = append(section.Items, cc.checkNegotiatedCipher())
	section.Items = append(section.Items, cc.checkTLSVersion())
	section.Items = append(section.Items, cc.checkDevicePosture())
	section.Items = append(section.Items, cc.checkMDMEnrolled())
	section.Items = append(section.Items, cc.checkClientCertificate())

	return section
}

func (cc *ComplianceChecker) checkClientOSFIPS() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "cp-1",
		Name:               "Client OS FIPS Mode",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyReported,
		What:               "Checks if connecting client devices have OS-level FIPS mode enabled",
		Why:                "Client-side FIPS mode ensures the browser/TLS stack only uses validated crypto.",
		Remediation:        "Enable FIPS mode on client OS. Deploy via MDM policy.",
		NISTRef:            "SC-13, CM-6",
	}

	devices := cc.posture.AllDevices()
	if len(devices) == 0 {
		item.Status = compliance.StatusWarning
		item.What = "No device posture reports — deploy WARP agent or configure posture API endpoint"
		return item
	}

	fips, nonFIPS := cc.posture.FIPSDeviceStats()
	if nonFIPS == 0 {
		item.Status = compliance.StatusPass
		item.What = fmt.Sprintf("All %d reporting device(s) have FIPS enabled", fips)
	} else {
		item.Status = compliance.StatusWarning
		item.What = fmt.Sprintf("%d FIPS, %d non-FIPS device(s) reporting", fips, nonFIPS)
	}
	return item
}

func (cc *ComplianceChecker) checkClientOSType() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "cp-2",
		Name:               "Client OS Type and Version",
		Severity:           "medium",
		VerificationMethod: compliance.VerifyReported,
		What:               "Reports client operating system types from device posture data",
		Why:                "OS type determines available FIPS crypto modules.",
		Remediation:        "Ensure endpoints run FIPS-capable OS versions.",
		NISTRef:            "CM-8, SI-2",
	}

	devices := cc.posture.AllDevices()
	if len(devices) == 0 {
		item.Status = compliance.StatusWarning
		item.What = "No device posture reports — deploy WARP agent to collect OS information"
		return item
	}

	item.Status = compliance.StatusPass
	item.What = fmt.Sprintf("%d device(s) reporting OS information", len(devices))
	return item
}

func (cc *ComplianceChecker) checkBrowserTLS() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "cp-3",
		Name:               "Browser TLS Capabilities",
		Severity:           "high",
		VerificationMethod: compliance.VerifyProbe,
		What:               "Analyzes TLS ClientHello to determine if clients offer only FIPS-approved ciphers",
		Why:                "If a browser offers ChaCha20-Poly1305 or RC4, it is not in FIPS mode.",
		Remediation:        "Enable FIPS mode on client browsers. For Windows: enable FIPS GPO.",
		NISTRef:            "SC-13, SC-8",
	}

	stats := cc.inspector.FIPSStats()
	if stats.Total == 0 {
		item.Status = compliance.StatusWarning
		item.What = "No TLS clients inspected — enable Tier 3 FIPS proxy or WARP for ClientHello analysis"
		return item
	}

	if stats.NonFIPS == 0 {
		item.Status = compliance.StatusPass
		item.What = fmt.Sprintf("All %d inspected client(s) are FIPS-capable", stats.FIPSCapable)
	} else {
		item.Status = compliance.StatusWarning
		item.What = fmt.Sprintf("%d FIPS-capable, %d non-FIPS client(s) detected", stats.FIPSCapable, stats.NonFIPS)
	}
	return item
}

func (cc *ComplianceChecker) checkNegotiatedCipher() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "cp-4",
		Name:               "Negotiated Cipher Suite (Client \u2192 Edge)",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyProbe,
		What:               "Reports the cipher suites offered by recently connected clients",
		Why:                "The negotiated cipher must be FIPS-approved for the client-to-edge segment.",
		Remediation:        "Restrict edge cipher suites via Cloudflare API to force FIPS-approved negotiation.",
		NISTRef:            "SC-13, SC-8",
	}

	recent := cc.inspector.RecentClients(10)
	if len(recent) == 0 {
		item.Status = compliance.StatusWarning
		item.What = "No recent client connections inspected — requires Tier 3 FIPS proxy"
		return item
	}

	allFIPS := true
	for _, c := range recent {
		if !c.FIPSCapable {
			allFIPS = false
			break
		}
	}

	if allFIPS {
		item.Status = compliance.StatusPass
	} else {
		item.Status = compliance.StatusWarning
	}
	item.What = fmt.Sprintf("%d recent connection(s) inspected", len(recent))
	return item
}

func (cc *ComplianceChecker) checkTLSVersion() compliance.ChecklistItem {
	return compliance.ChecklistItem{
		ID:                 "cp-5",
		Name:               "TLS Version (Client \u2192 Edge)",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyProbe,
		Status:             compliance.StatusPass,
		What:               "Cloudflare edge enforces minimum TLS version; clients below threshold are rejected",
		Why:                "TLS 1.0/1.1 are prohibited under FIPS. Edge enforcement ensures compliance.",
		Remediation:        "Set minimum TLS version to 1.2 in Cloudflare zone settings.",
		NISTRef:            "SC-8, SC-13",
	}
}

func (cc *ComplianceChecker) checkDevicePosture() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "cp-6",
		Name:               "Cloudflare Access Device Posture",
		Severity:           "high",
		VerificationMethod: compliance.VerifyReported,
		What:               "Checks if Cloudflare Access device posture checks are receiving reports",
		Why:                "Device posture validates endpoint security before granting access.",
		Remediation:        "Configure device posture checks in Zero Trust > Devices > Device Posture.",
		NISTRef:            "AC-17, CM-8",
	}

	devices := cc.posture.AllDevices()
	if len(devices) == 0 {
		item.Status = compliance.StatusWarning
		item.What = "No posture reports — configure Cloudflare Access device posture checks with WARP agent"
		return item
	}

	item.Status = compliance.StatusPass
	item.What = fmt.Sprintf("%d device(s) reporting posture", len(devices))
	return item
}

func (cc *ComplianceChecker) checkMDMEnrolled() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "cp-7",
		Name:               "MDM Enrollment Verified",
		Severity:           "medium",
		VerificationMethod: compliance.VerifyReported,
		What:               "Checks if connecting devices are enrolled in MDM",
		Why:                "MDM enrollment enables enforcement of FIPS policies on endpoints.",
		Remediation:        "Enroll devices in your MDM solution. Add MDM check to Access device posture.",
		NISTRef:            "CM-8, CM-2",
	}

	devices := cc.posture.AllDevices()
	if len(devices) == 0 {
		item.Status = compliance.StatusWarning
		item.What = "No device reports — configure MDM integration (Intune/Jamf) via --mdm-provider flag"
		return item
	}

	mdmCount := 0
	for _, d := range devices {
		if d.MDMEnrolled {
			mdmCount++
		}
	}

	if mdmCount == len(devices) {
		item.Status = compliance.StatusPass
		item.What = fmt.Sprintf("All %d device(s) MDM-enrolled", mdmCount)
	} else {
		item.Status = compliance.StatusWarning
		item.What = fmt.Sprintf("%d/%d device(s) MDM-enrolled", mdmCount, len(devices))
	}
	return item
}

func (cc *ComplianceChecker) checkClientCertificate() compliance.ChecklistItem {
	return compliance.ChecklistItem{
		ID:                 "cp-8",
		Name:               "Client Certificate (mTLS)",
		Severity:           "medium",
		VerificationMethod: compliance.VerifyProbe,
		Status:             compliance.StatusWarning,
		What:               "mTLS not configured — optional for enhanced device identity via Cloudflare Access",
		Why:                "Client certificates provide strong device identity for zero-trust access.",
		Remediation:        "Configure mTLS in Cloudflare Access with a trusted CA.",
		NISTRef:            "IA-2, IA-5",
	}
}

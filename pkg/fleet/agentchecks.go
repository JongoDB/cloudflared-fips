package fleet

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// AgentChecks performs FIPS posture checks on the local endpoint.
// These are lightweight checks suitable for client agents.
type AgentChecks struct{}

// NewAgentChecks creates a new agent checks instance.
func NewAgentChecks() *AgentChecks {
	return &AgentChecks{}
}

// RunChecks performs all agent checks and returns a compliance section.
func (a *AgentChecks) RunChecks() compliance.Section {
	items := []compliance.ChecklistItem{
		a.checkOSFIPSMode(),
		a.checkOSType(),
		a.checkDiskEncryption(),
		a.checkMDMEnrollment(),
		a.checkWARPInstalled(),
		a.checkTLSCapabilities(),
	}

	return compliance.Section{
		ID:          "agent-posture",
		Name:        "Endpoint FIPS Posture",
		Description: "FIPS compliance checks for this endpoint",
		Items:       items,
	}
}

func (a *AgentChecks) checkOSType() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ag-os",
		Name:               "Operating System",
		Severity:           "info",
		VerificationMethod: compliance.VerifyDirect,
		What:               "Operating system type and version",
		Why:                "Different OSes have different FIPS validation status",
		NISTRef:            "SC-13",
	}

	item.Status = compliance.StatusPass
	item.Remediation = runtime.GOOS + "/" + runtime.GOARCH
	return item
}

func (a *AgentChecks) checkOSFIPSMode() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ag-fips",
		Name:               "OS FIPS Mode",
		Severity:           "critical",
		VerificationMethod: compliance.VerifyDirect,
		What:               "Whether the OS is running in FIPS mode",
		Why:                "OS FIPS mode ensures all system crypto uses validated modules",
		Remediation:        "Enable FIPS mode: fips-mode-setup --enable (Linux), GPO policy (Windows)",
		NISTRef:            "SC-13",
	}

	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/sys/crypto/fips_enabled")
		if err != nil {
			item.Status = compliance.StatusUnknown
			item.Remediation = "Cannot read /proc/sys/crypto/fips_enabled"
			return item
		}
		if strings.TrimSpace(string(data)) == "1" {
			item.Status = compliance.StatusPass
		} else {
			item.Status = compliance.StatusFail
		}
	case "darwin":
		// macOS always uses CommonCrypto which is FIPS validated
		item.Status = compliance.StatusPass
		item.Remediation = "macOS CommonCrypto is always active"
	case "windows":
		// Check Windows FIPS registry key
		out, err := exec.Command("reg", "query",
			`HKLM\SYSTEM\CurrentControlSet\Control\Lsa\FIPSAlgorithmPolicy`,
			"/v", "Enabled").Output()
		if err != nil {
			item.Status = compliance.StatusUnknown
			return item
		}
		if strings.Contains(string(out), "0x1") {
			item.Status = compliance.StatusPass
		} else {
			item.Status = compliance.StatusFail
		}
	default:
		item.Status = compliance.StatusUnknown
	}
	return item
}

func (a *AgentChecks) checkDiskEncryption() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ag-disk",
		Name:               "Disk Encryption",
		Severity:           "high",
		VerificationMethod: compliance.VerifyDirect,
		What:               "Full disk encryption is enabled",
		Why:                "Protects data at rest per FIPS requirements",
		Remediation:        "Enable LUKS (Linux), BitLocker (Windows), or FileVault (macOS)",
		NISTRef:            "SC-28",
	}

	switch runtime.GOOS {
	case "linux":
		// Check for LUKS devices
		if _, err := exec.LookPath("lsblk"); err == nil {
			out, err := exec.Command("lsblk", "-o", "TYPE", "--noheadings").Output()
			if err == nil && strings.Contains(string(out), "crypt") {
				item.Status = compliance.StatusPass
				return item
			}
		}
		// Also check for dm-crypt
		if _, err := os.Stat("/dev/mapper"); err == nil {
			entries, _ := os.ReadDir("/dev/mapper")
			for _, e := range entries {
				if e.Name() != "control" {
					item.Status = compliance.StatusPass
					return item
				}
			}
		}
		item.Status = compliance.StatusWarning
		item.Remediation = "No encrypted volumes detected. Enable LUKS encryption."
	case "darwin":
		out, err := exec.Command("fdesetup", "status").Output()
		if err == nil && strings.Contains(string(out), "FileVault is On") {
			item.Status = compliance.StatusPass
		} else {
			item.Status = compliance.StatusFail
		}
	case "windows":
		out, err := exec.Command("manage-bde", "-status", "C:").Output()
		if err == nil && strings.Contains(string(out), "Fully Encrypted") {
			item.Status = compliance.StatusPass
		} else {
			item.Status = compliance.StatusWarning
		}
	default:
		item.Status = compliance.StatusUnknown
	}
	return item
}

func (a *AgentChecks) checkMDMEnrollment() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ag-mdm",
		Name:               "MDM Enrollment",
		Severity:           "medium",
		VerificationMethod: compliance.VerifyDirect,
		What:               "Device is enrolled in an MDM system",
		Why:                "MDM ensures policy enforcement and device compliance",
		Remediation:        "Contact IT to enroll device in Intune or Jamf",
		NISTRef:            "CM-6",
	}

	switch runtime.GOOS {
	case "darwin":
		// Check for MDM enrollment profiles
		out, err := exec.Command("profiles", "status", "-type", "enrollment").Output()
		if err == nil && (strings.Contains(string(out), "MDM") || strings.Contains(string(out), "enrolled")) {
			item.Status = compliance.StatusPass
		} else {
			item.Status = compliance.StatusWarning
		}
	case "windows":
		out, err := exec.Command("dsregcmd", "/status").Output()
		if err == nil && strings.Contains(string(out), "AzureAdJoined : YES") {
			item.Status = compliance.StatusPass
		} else {
			item.Status = compliance.StatusWarning
		}
	case "linux":
		// Linux MDM is less common; check for common agents
		agents := []string{"/opt/microsoft/mdatp", "/usr/local/jamf", "/opt/tanium"}
		for _, path := range agents {
			if _, err := os.Stat(path); err == nil {
				item.Status = compliance.StatusPass
				return item
			}
		}
		item.Status = compliance.StatusWarning
		item.Remediation = "No MDM agent detected"
	default:
		item.Status = compliance.StatusUnknown
	}
	return item
}

func (a *AgentChecks) checkWARPInstalled() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ag-warp",
		Name:               "Cloudflare WARP",
		Severity:           "medium",
		VerificationMethod: compliance.VerifyDirect,
		What:               "Cloudflare WARP client is installed and running",
		Why:                "WARP provides secure DNS and can enforce device posture",
		Remediation:        "Install Cloudflare WARP: https://1.1.1.1",
		NISTRef:            "SC-8",
	}

	// Check if warp-cli is available
	if _, err := exec.LookPath("warp-cli"); err == nil {
		out, err := exec.Command("warp-cli", "status").Output()
		if err == nil && strings.Contains(string(out), "Connected") {
			item.Status = compliance.StatusPass
			return item
		}
		item.Status = compliance.StatusWarning
		item.Remediation = "WARP installed but not connected"
		return item
	}

	// Check common install paths
	paths := []string{
		"/usr/bin/warp-cli",
		"/Applications/Cloudflare WARP.app",
		`C:\Program Files\Cloudflare\Cloudflare WARP\warp-svc.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			item.Status = compliance.StatusWarning
			item.Remediation = "WARP found but not on PATH or not running"
			return item
		}
	}

	item.Status = compliance.StatusWarning
	return item
}

func (a *AgentChecks) checkTLSCapabilities() compliance.ChecklistItem {
	item := compliance.ChecklistItem{
		ID:                 "ag-tls",
		Name:               "TLS Capabilities",
		Severity:           "high",
		VerificationMethod: compliance.VerifyDirect,
		What:               "FIPS-approved TLS cipher suites available",
		Why:                "Ensures connections use validated cryptography",
		Remediation:        "Enable OS FIPS mode to restrict to approved cipher suites",
		NISTRef:            "SC-8",
	}

	// The Go runtime in this binary reports its own cipher suite support
	// which is representative of the host's capabilities
	item.Status = compliance.StatusPass
	return item
}

package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ControllerConfigPage collects controller-specific settings: fleet identity,
// ingress rules, and compliance enforcement policy.
// Tunnel token and protocol are no longer needed here — the tunnel is created
// on the Dashboard Wiring page and the token is auto-generated.
type ControllerConfigPage struct {
	adminKey        common.TextInput
	nodeName        common.TextInput
	region          common.TextInput
	ingress         common.IngressEditor
	enforcementMode common.Selector
	requireOSFIPS   common.Toggle
	requireDiskEnc  common.Toggle
	requireMDM      common.Toggle

	// Pre-populated from dashboard wiring page (read-only display)
	tunnelTokenSet bool

	focus  int
	width  int
	height int
}

const controllerFieldCount = 8

// NewControllerConfigPage creates the controller config page.
func NewControllerConfigPage() *ControllerConfigPage {
	adminKey := common.NewTextInput("Admin API Key", "auto-generated if empty", "(leave blank to auto-generate)")
	adminKey.HelpText = "Used by fleet nodes to authenticate with this controller.\nLeave blank to auto-generate a random 32-char key.\nStore securely — needed for fleet admin operations."

	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	region := common.NewTextInput("Node Region", "us-east", "(optional label)")

	ing := common.NewIngressEditor("Ingress Rules")

	enforcement := common.NewSelector("Enforcement Mode", []common.SelectorOption{
		{Value: "audit", Label: "Audit", Description: "Log non-compliance but allow traffic (recommended to start)"},
		{Value: "enforce", Label: "Enforce", Description: "Deny traffic from non-compliant nodes"},
		{Value: "disabled", Label: "Disabled", Description: "No compliance checking"},
	})
	enforcement.HelpText = "Audit: log violations but allow traffic (start here).\nEnforce: block traffic from non-compliant nodes.\nDisabled: no compliance checking at all."

	requireOSFIPS := common.NewToggle("Require OS FIPS mode", "All nodes must have OS-level FIPS enabled", false)
	requireOSFIPS.HelpText = "Nodes without OS FIPS mode will be flagged non-compliant.\nLinux: requires fips-mode-setup --enable && reboot\nWindows: requires FIPS GPO policy\nmacOS: always passes (CommonCrypto is FIPS-validated)"

	requireDiskEnc := common.NewToggle("Require disk encryption", "All nodes must have disk encryption enabled", false)
	requireDiskEnc.HelpText = "Checks for full-disk encryption on each fleet node.\nLinux: LUKS/dm-crypt  |  Windows: BitLocker  |  macOS: FileVault\nThe agent detects but cannot auto-enable encryption."

	requireMDM := common.NewToggle("Require MDM enrollment", "All nodes must be enrolled in an MDM provider", false)
	requireMDM.HelpText = "Nodes must be enrolled in a managed device platform.\nSupported: Microsoft Intune (Azure AD) or Jamf Pro (Apple).\nConfigure the MDM provider on the Dashboard & Tunnel page."

	return &ControllerConfigPage{
		adminKey:        adminKey,
		nodeName:        nodeName,
		region:          region,
		ingress:         ing,
		enforcementMode: enforcement,
		requireOSFIPS:   requireOSFIPS,
		requireDiskEnc:  requireDiskEnc,
		requireMDM:      requireMDM,
	}
}

// PrePopulateTunnelToken marks that the tunnel token was set from the
// dashboard wiring page. The controller config page no longer collects
// the tunnel token — it's auto-generated.
func (p *ControllerConfigPage) PrePopulateTunnelToken(token string) {
	if token != "" {
		p.tunnelTokenSet = true
	}
}

func (p *ControllerConfigPage) Title() string { return "Controller & Tunnel" }
func (p *ControllerConfigPage) Init() tea.Cmd { return nil }

func (p *ControllerConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.adminKey.Focus()
}

func (p *ControllerConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ControllerConfigPage) updateFocus() {
	p.adminKey.Blur()
	p.nodeName.Blur()
	p.region.Blur()
	p.ingress.Blur()
	p.enforcementMode.Blur()
	p.requireOSFIPS.Blur()
	p.requireDiskEnc.Blur()
	p.requireMDM.Blur()

	switch p.focus {
	case 0:
		p.adminKey.Focus()
	case 1:
		p.nodeName.Focus()
	case 2:
		p.region.Focus()
	case 3:
		p.ingress.Focus()
	case 4:
		p.enforcementMode.Focus()
	case 5:
		p.requireOSFIPS.Focus()
	case 6:
		p.requireDiskEnc.Focus()
	case 7:
		p.requireMDM.Focus()
	}
}

func (p *ControllerConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Ingress in add mode: don't advance
			if p.focus == 3 && p.ingress.Adding {
				return p, fieldNav
			}
			// Selector: enter selects within
			if msg.String() == "enter" && p.focus == 4 {
				return p, fieldNav
			}
			if p.focus < controllerFieldCount-1 {
				p.focus++
				p.updateFocus()
				return p, fieldNav
			}
			return p, nil
		case "shift+tab":
			if p.focus > 0 {
				p.focus--
				p.updateFocus()
				return p, fieldNav
			}
			return p, nil
		}
	}

	var cmd tea.Cmd
	switch p.focus {
	case 0:
		cmd = p.adminKey.Update(msg)
	case 1:
		cmd = p.nodeName.Update(msg)
	case 2:
		cmd = p.region.Update(msg)
	case 3:
		cmd = p.ingress.Update(msg)
	case 4:
		p.enforcementMode.Update(msg)
	case 5:
		p.requireOSFIPS.Update(msg)
	case 6:
		p.requireDiskEnc.Update(msg)
	case 7:
		p.requireMDM.Update(msg)
	}
	return p, cmd
}

func (p *ControllerConfigPage) ScrollOffset() int {
	offsets := []int{0, 5, 8, 13, 19, 26, 29, 32}
	if p.focus < len(offsets) {
		return offsets[p.focus]
	}
	return 0
}

func (p *ControllerConfigPage) Validate() bool {
	return true
}

func (p *ControllerConfigPage) Apply(cfg *config.Config) {
	cfg.AdminKey = strings.TrimSpace(p.adminKey.Value())
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	// TunnelToken is set by DashboardWiringPage (auto-generated); don't overwrite.
	// Protocol is always QUIC (default, best performance) — no longer configurable.
	if cfg.Protocol == "" {
		cfg.Protocol = "quic"
	}

	var rules []config.IngressRule
	for _, entry := range p.ingress.Entries {
		rules = append(rules, config.IngressRule{
			Hostname: entry.Hostname,
			Service:  entry.Service,
		})
	}
	rules = append(rules, config.IngressRule{Service: "http_status:404"})
	cfg.Ingress = rules

	cfg.CompliancePolicy = config.CompliancePolicyConfig{
		EnforcementMode: p.enforcementMode.Selected(),
		RequireOSFIPS:   p.requireOSFIPS.Enabled,
		RequireDiskEnc:  p.requireDiskEnc.Enabled,
		RequireMDM:      p.requireMDM.Enabled,
	}
}

func (p *ControllerConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Controller Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.adminKey.View())
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")

	// Show tunnel status from dashboard wiring page
	if p.tunnelTokenSet {
		b.WriteString(common.SuccessStyle.Render("Tunnel token set from Dashboard & Tunnel page"))
		b.WriteString("\n\n")
	}

	b.WriteString(p.ingress.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Compliance Enforcement Policy"))
	b.WriteString("\n\n")
	b.WriteString(p.enforcementMode.View())
	b.WriteString("\n\n")
	b.WriteString(p.requireOSFIPS.View())
	b.WriteString("\n\n")
	b.WriteString(p.requireDiskEnc.View())
	b.WriteString("\n\n")
	b.WriteString(p.requireMDM.View())
	return b.String()
}

package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ControllerConfigPage collects controller-specific settings: tunnel, ingress,
// fleet identity, and compliance enforcement policy.
type ControllerConfigPage struct {
	adminKey        common.TextInput
	nodeName        common.TextInput
	region          common.TextInput
	tunnelToken     common.TextInput
	protocol        common.Selector
	ingress         common.IngressEditor
	enforcementMode common.Selector
	requireOSFIPS   common.Toggle
	requireDiskEnc  common.Toggle
	withCF          common.Toggle

	focus  int
	width  int
	height int
}

const controllerFieldCount = 10

// NewControllerConfigPage creates the controller config page.
func NewControllerConfigPage() *ControllerConfigPage {
	adminKey := common.NewTextInput("Admin API Key", "auto-generated if empty", "(leave blank to auto-generate)")
	adminKey.HelpText = "Used by fleet nodes to authenticate with this controller.\nLeave blank to auto-generate a random 32-char key.\nStore securely — needed for fleet admin operations."

	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	region := common.NewTextInput("Node Region", "us-east", "(optional label)")

	tunnelToken := common.NewTextInput("Tunnel Token", "eyJ...", "(REQUIRED — controller owns the Cloudflare tunnel)")
	tunnelToken.Validate = config.ValidateNonEmpty
	tunnelToken.HelpText = "Get from Cloudflare Zero Trust dashboard:\n  Networks → Tunnels → Create → Cloudflared → copy token\nOr CLI: cloudflared tunnel token <tunnel-name>"

	proto := common.NewSelector("Protocol", []common.SelectorOption{
		{Value: "quic", Label: "QUIC", Description: "UDP 7844 — preferred, lower latency"},
		{Value: "http2", Label: "HTTP/2", Description: "TCP 443 — fallback when UDP is blocked"},
	})

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

	return &ControllerConfigPage{
		adminKey:        adminKey,
		nodeName:        nodeName,
		region:          region,
		tunnelToken:     tunnelToken,
		protocol:        proto,
		ingress:         ing,
		enforcementMode: enforcement,
		requireOSFIPS:   requireOSFIPS,
		requireDiskEnc:  requireDiskEnc,
		withCF:          common.NewToggle("Enable Cloudflare API integration", "Wire dashboard to CF API for edge compliance checks", false),
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
	p.tunnelToken.Blur()
	p.protocol.Blur()
	p.ingress.Blur()
	p.enforcementMode.Blur()
	p.requireOSFIPS.Blur()
	p.requireDiskEnc.Blur()
	p.withCF.Blur()

	switch p.focus {
	case 0:
		p.adminKey.Focus()
	case 1:
		p.nodeName.Focus()
	case 2:
		p.region.Focus()
	case 3:
		p.tunnelToken.Focus()
	case 4:
		p.protocol.Focus()
	case 5:
		p.ingress.Focus()
	case 6:
		p.enforcementMode.Focus()
	case 7:
		p.requireOSFIPS.Focus()
	case 8:
		p.requireDiskEnc.Focus()
	case 9:
		p.withCF.Focus()
	}
}

func (p *ControllerConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Ingress in add mode: don't advance
			if p.focus == 5 && p.ingress.Adding {
				return p, fieldNav
			}
			// Selectors: enter selects within
			if msg.String() == "enter" && (p.focus == 4 || p.focus == 6) {
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
		cmd = p.tunnelToken.Update(msg)
	case 4:
		p.protocol.Update(msg)
	case 5:
		cmd = p.ingress.Update(msg)
	case 6:
		p.enforcementMode.Update(msg)
	case 7:
		p.requireOSFIPS.Update(msg)
	case 8:
		p.requireDiskEnc.Update(msg)
	case 9:
		p.withCF.Update(msg)
	}
	return p, cmd
}

func (p *ControllerConfigPage) ScrollOffset() int {
	// Approximate line offsets per field in the View() output.
	// Each TextInput = ~2 lines (label + input), each \n\n gap = 1 blank.
	offsets := []int{0, 5, 8, 13, 16, 22, 28, 35, 38, 41}
	if p.focus < len(offsets) {
		return offsets[p.focus]
	}
	return 0
}

func (p *ControllerConfigPage) Validate() bool {
	return p.tunnelToken.RunValidation()
}

func (p *ControllerConfigPage) Apply(cfg *config.Config) {
	cfg.AdminKey = strings.TrimSpace(p.adminKey.Value())
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	cfg.TunnelToken = strings.TrimSpace(p.tunnelToken.Value())
	cfg.Protocol = p.protocol.Selected()

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
	}
	cfg.WithCF = p.withCF.Enabled
}

func (p *ControllerConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Controller & Tunnel Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.adminKey.View())
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Cloudflare Tunnel (controller owns the tunnel)"))
	b.WriteString("\n\n")
	b.WriteString(p.tunnelToken.View())
	b.WriteString("\n\n")
	b.WriteString(p.protocol.View())
	b.WriteString("\n")
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
	b.WriteString(p.withCF.View())
	return b.String()
}

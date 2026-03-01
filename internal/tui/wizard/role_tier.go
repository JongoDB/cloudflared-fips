package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/deployment"
)

// RoleTierPage is the first wizard page: select role, tier, and skip-FIPS toggle.
type RoleTierPage struct {
	roleSelector common.Selector
	tierSelector common.Selector
	skipFIPS     common.Toggle

	focus  int
	width  int
	height int
}

// NewRoleTierPage creates page 1.
func NewRoleTierPage() *RoleTierPage {
	role := common.NewSelector("Node Role", []common.SelectorOption{
		{Value: "controller", Label: "Controller", Description: "Central hub — Cloudflare tunnel, fleet management, compliance enforcement, traffic routing"},
		{Value: "server", Label: "Server", Description: "Origin server — registers service endpoint with controller, runs mandatory posture agent"},
		{Value: "proxy", Label: "Proxy", Description: "Client-side forward proxy — own Cloudflare tunnel, FIPS TLS termination, posture agent"},
		{Value: "client", Label: "Client", Description: "Endpoint device — mandatory FIPS posture agent, non-compliant devices denied access"},
	})
	role.Cursor = 0 // default: controller (provisioned first)

	t1 := deployment.GetTierInfo(deployment.TierStandard)
	t2 := deployment.GetTierInfo(deployment.TierRegionalKeyless)
	t3 := deployment.GetTierInfo(deployment.TierSelfHosted)
	tier := common.NewSelector("Deployment Tier", []common.SelectorOption{
		{Value: string(deployment.TierStandard), Label: "Tier 1: " + t1.Name, Description: t1.Description},
		{Value: string(deployment.TierRegionalKeyless), Label: "Tier 2: " + t2.Name, Description: t2.Description},
		{Value: string(deployment.TierSelfHosted), Label: "Tier 3: " + t3.Name, Description: t3.Description},
	})

	return &RoleTierPage{
		roleSelector: role,
		tierSelector: tier,
		skipFIPS:     common.NewToggle("Skip FIPS mode", "Dev/test only — skip OS FIPS enablement", false),
	}
}

func (p *RoleTierPage) Title() string { return "Role & Tier" }
func (p *RoleTierPage) Init() tea.Cmd { return nil }

func (p *RoleTierPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return fieldNav
}

func (p *RoleTierPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// SelectedRole returns the currently selected role value.
func (p *RoleTierPage) SelectedRole() string {
	return p.roleSelector.Selected()
}

// SelectedTier returns the currently selected tier value.
func (p *RoleTierPage) SelectedTier() string {
	return p.tierSelector.Selected()
}

func (p *RoleTierPage) fieldCount() int {
	role := p.SelectedRole()
	// Client and proxy don't get a tier selector (client=tier 1, proxy=tier 3 implicit)
	if role == "client" || role == "proxy" {
		return 2 // role + skipFIPS
	}
	return 3 // role + tier + skipFIPS
}

func (p *RoleTierPage) showsTier() bool {
	return p.SelectedRole() == "controller"
}

func (p *RoleTierPage) updateFocus() {
	p.roleSelector.Blur()
	p.tierSelector.Blur()
	p.skipFIPS.Blur()

	switch p.focus {
	case 0:
		p.roleSelector.Focus()
	case 1:
		if p.showsTier() {
			p.tierSelector.Focus()
		} else {
			p.skipFIPS.Focus()
		}
	case 2:
		p.skipFIPS.Focus()
	}
}

func (p *RoleTierPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Selectors: enter selects within, tab advances
			if p.focus < p.fieldCount()-1 {
				if msg.String() == "enter" && p.isSelectorFocused() {
					return p, fieldNav
				}
				p.focus++
				p.updateFocus()
				return p, fieldNav
			}
			// Last field — let wizard handle page advance
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

	switch p.focus {
	case 0:
		p.roleSelector.Update(msg)
	case 1:
		if p.showsTier() {
			p.tierSelector.Update(msg)
		} else {
			p.skipFIPS.Update(msg)
		}
	case 2:
		p.skipFIPS.Update(msg)
	}
	return p, nil
}

func (p *RoleTierPage) isSelectorFocused() bool {
	if p.focus == 0 {
		return true
	}
	if p.focus == 1 && p.showsTier() {
		return true
	}
	return false
}

func (p *RoleTierPage) ScrollOffset() int {
	// Page is short enough — no scrolling needed.
	return 0
}

func (p *RoleTierPage) Validate() bool { return true }

func (p *RoleTierPage) Apply(cfg *config.Config) {
	cfg.Role = p.SelectedRole()
	switch cfg.Role {
	case "controller":
		cfg.DeploymentTier = p.SelectedTier()
	case "proxy":
		cfg.DeploymentTier = string(deployment.TierSelfHosted)
	default: // server, client
		cfg.DeploymentTier = string(deployment.TierStandard)
	}
	cfg.SkipFIPS = p.skipFIPS.Enabled
}

func (p *RoleTierPage) View() string {
	var b strings.Builder

	b.WriteString(p.roleSelector.View())

	if p.showsTier() {
		b.WriteString("\n")
		b.WriteString(p.tierSelector.View())
	}

	b.WriteString("\n")
	b.WriteString(p.skipFIPS.View())

	// Dynamic description panel
	b.WriteString("\n\n")
	b.WriteString(p.roleDescription())

	return b.String()
}

func (p *RoleTierPage) roleDescription() string {
	role := p.SelectedRole()

	var desc string
	switch role {
	case "controller":
		desc = "Installs: dashboard, selftest, agent, cloudflared"
		desc += "\nServices: cloudflared tunnel, dashboard (fleet mode), agent"
		desc += "\nOwns the Cloudflare tunnel and routes traffic to compliant servers"
	case "server":
		desc = "Installs: selftest, agent"
		desc += "\nServices: agent (mandatory posture reporting)"
		desc += "\nRegisters origin service with controller — no tunnel needed"
	case "proxy":
		desc = "Installs: selftest, fips-proxy, agent, cloudflared"
		desc += "\nServices: fips-proxy (FIPS TLS termination), cloudflared tunnel, agent"
		desc += "\nClients connect through this proxy for FIPS-compliant egress"
	case "client":
		desc = "Installs: selftest, agent (~11 MB)"
		desc += "\nServices: agent (mandatory FIPS posture reporting)"
		desc += "\nNon-compliant devices will be denied access to protected services"
	}

	return common.HintStyle.Render(desc)
}

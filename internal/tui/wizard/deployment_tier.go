package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/deployment"
)

// DeploymentTierPage is page 3: tier picker with conditional fields.
type DeploymentTierPage struct {
	tierSelector common.Selector

	// Tier 2 fields
	keylessSSLHost   common.TextInput
	keylessSSLPort   common.TextInput
	regionalServices common.Toggle

	// Tier 3 fields
	proxyListenAddr common.TextInput
	proxyCertFile   common.TextInput
	proxyKeyFile    common.TextInput
	proxyUpstream   common.TextInput

	focus  int
	width  int
	height int
}

// NewDeploymentTierPage creates page 3.
func NewDeploymentTierPage() *DeploymentTierPage {
	t1 := deployment.GetTierInfo(deployment.TierStandard)
	t2 := deployment.GetTierInfo(deployment.TierRegionalKeyless)
	t3 := deployment.GetTierInfo(deployment.TierSelfHosted)

	sel := common.NewSelector("Deployment Tier", []common.SelectorOption{
		{Value: string(deployment.TierStandard), Label: "Tier 1: " + t1.Name, Description: t1.Description},
		{Value: string(deployment.TierRegionalKeyless), Label: "Tier 2: " + t2.Name, Description: t2.Description},
		{Value: string(deployment.TierSelfHosted), Label: "Tier 3: " + t3.Name, Description: t3.Description},
	})

	kslHost := common.NewTextInput("Keyless SSL Host", "keyless.example.com", "")
	kslPort := common.NewTextInput("Keyless SSL Port", "2407", "(default: 2407)")
	kslPort.Input.SetValue("2407")
	regional := common.NewToggle("Regional Services Enabled", "Restrict to FedRAMP US data centers", true)

	proxyAddr := common.NewTextInput("Proxy Listen Address", "0.0.0.0:443", "")
	proxyAddr.Input.SetValue("0.0.0.0:443")
	proxyCert := common.NewTextInput("TLS Certificate File", "/etc/pki/tls/certs/proxy.pem", "")
	proxyKey := common.NewTextInput("TLS Private Key File", "/etc/pki/tls/private/proxy-key.pem", "")
	proxyUp := common.NewTextInput("Upstream URL", "https://your-app.example.com", "")

	return &DeploymentTierPage{
		tierSelector:     sel,
		keylessSSLHost:   kslHost,
		keylessSSLPort:   kslPort,
		regionalServices: regional,
		proxyListenAddr:  proxyAddr,
		proxyCertFile:    proxyCert,
		proxyKeyFile:     proxyKey,
		proxyUpstream:    proxyUp,
	}
}

func (p *DeploymentTierPage) Title() string { return "Deployment Tier" }

func (p *DeploymentTierPage) Init() tea.Cmd { return nil }

func (p *DeploymentTierPage) Focus() tea.Cmd {
	p.focus = 0
	p.tierSelector.Focus()
	return nil
}

func (p *DeploymentTierPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *DeploymentTierPage) selectedTier() string {
	return p.tierSelector.Selected()
}

func (p *DeploymentTierPage) fieldCount() int {
	switch p.selectedTier() {
	case string(deployment.TierRegionalKeyless):
		return 1 + 3 // selector + host, port, regional
	case string(deployment.TierSelfHosted):
		return 1 + 4 // selector + addr, cert, key, upstream
	default:
		return 1 // just selector
	}
}

func (p *DeploymentTierPage) updateFocus() {
	p.tierSelector.Blur()
	p.keylessSSLHost.Blur()
	p.keylessSSLPort.Blur()
	p.regionalServices.Blur()
	p.proxyListenAddr.Blur()
	p.proxyCertFile.Blur()
	p.proxyKeyFile.Blur()
	p.proxyUpstream.Blur()

	switch p.focus {
	case 0:
		p.tierSelector.Focus()
	default:
		idx := p.focus - 1
		switch p.selectedTier() {
		case string(deployment.TierRegionalKeyless):
			switch idx {
			case 0:
				p.keylessSSLHost.Input.Focus()
			case 1:
				p.keylessSSLPort.Input.Focus()
			case 2:
				p.regionalServices.Focus()
			}
		case string(deployment.TierSelfHosted):
			switch idx {
			case 0:
				p.proxyListenAddr.Input.Focus()
			case 1:
				p.proxyCertFile.Input.Focus()
			case 2:
				p.proxyKeyFile.Input.Focus()
			case 3:
				p.proxyUpstream.Input.Focus()
			}
		}
	}
}

func (p *DeploymentTierPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Tier selector: enter selects within, tab advances
			if p.focus == 0 && msg.String() == "enter" {
				return p, fieldNav
			}
			if p.focus < p.fieldCount()-1 {
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
		p.tierSelector.Update(msg)
		// Reset focus if tier changed and conditional fields no longer exist
		if p.focus >= p.fieldCount() {
			p.focus = 0
			p.updateFocus()
		}
	default:
		idx := p.focus - 1
		switch p.selectedTier() {
		case string(deployment.TierRegionalKeyless):
			switch idx {
			case 0:
				cmd = p.keylessSSLHost.Update(msg)
			case 1:
				cmd = p.keylessSSLPort.Update(msg)
			case 2:
				p.regionalServices.Update(msg)
			}
		case string(deployment.TierSelfHosted):
			switch idx {
			case 0:
				cmd = p.proxyListenAddr.Update(msg)
			case 1:
				cmd = p.proxyCertFile.Update(msg)
			case 2:
				cmd = p.proxyKeyFile.Update(msg)
			case 3:
				cmd = p.proxyUpstream.Update(msg)
			}
		}
	}
	return p, cmd
}

func (p *DeploymentTierPage) Validate() bool {
	// Tier selector always valid; validate conditional fields
	switch p.selectedTier() {
	case string(deployment.TierSelfHosted):
		valid := true
		p.proxyListenAddr.Validate = config.ValidateNonEmpty
		if !p.proxyListenAddr.RunValidation() {
			valid = false
		}
		p.proxyCertFile.Validate = config.ValidateNonEmpty
		if !p.proxyCertFile.RunValidation() {
			valid = false
		}
		p.proxyKeyFile.Validate = config.ValidateNonEmpty
		if !p.proxyKeyFile.RunValidation() {
			valid = false
		}
		p.proxyUpstream.Validate = config.ValidateNonEmpty
		if !p.proxyUpstream.RunValidation() {
			valid = false
		}
		return valid
	}
	return true
}

func (p *DeploymentTierPage) Apply(cfg *config.Config) {
	cfg.DeploymentTier = p.selectedTier()

	switch p.selectedTier() {
	case string(deployment.TierRegionalKeyless):
		cfg.KeylessSSLHost = strings.TrimSpace(p.keylessSSLHost.Value())
		portStr := strings.TrimSpace(p.keylessSSLPort.Value())
		port := 2407
		if portStr != "" {
			if v := parseInt(portStr); v > 0 {
				port = v
			}
		}
		cfg.KeylessSSLPort = port
		cfg.RegionalServices = p.regionalServices.Enabled
	case string(deployment.TierSelfHosted):
		cfg.ProxyListenAddr = strings.TrimSpace(p.proxyListenAddr.Value())
		cfg.ProxyCertFile = strings.TrimSpace(p.proxyCertFile.Value())
		cfg.ProxyKeyFile = strings.TrimSpace(p.proxyKeyFile.Value())
		cfg.ProxyUpstream = strings.TrimSpace(p.proxyUpstream.Value())
	}
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}

func (p *DeploymentTierPage) View() string {
	var b strings.Builder
	b.WriteString(p.tierSelector.View())

	switch p.selectedTier() {
	case string(deployment.TierRegionalKeyless):
		b.WriteString("\n")
		b.WriteString(common.LabelStyle.Render("Tier 2 — Keyless SSL + Regional Services"))
		b.WriteString("\n\n")
		b.WriteString(p.keylessSSLHost.View())
		b.WriteString("\n\n")
		b.WriteString(p.keylessSSLPort.View())
		b.WriteString("\n\n")
		b.WriteString(p.regionalServices.View())
	case string(deployment.TierSelfHosted):
		b.WriteString("\n")
		b.WriteString(common.LabelStyle.Render("Tier 3 — Self-Hosted FIPS Proxy"))
		b.WriteString("\n\n")
		b.WriteString(p.proxyListenAddr.View())
		b.WriteString("\n\n")
		b.WriteString(p.proxyCertFile.View())
		b.WriteString("\n\n")
		b.WriteString(p.proxyKeyFile.View())
		b.WriteString("\n\n")
		b.WriteString(p.proxyUpstream.View())
	}

	return b.String()
}

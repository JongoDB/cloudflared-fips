package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// TierSpecificPage collects tier 2 or tier 3 settings for controller/server roles.
// It is only included in the wizard when tier != "standard".
type TierSpecificPage struct {
	tier string // "regional_keyless" or "self_hosted"

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

// NewTierSpecificPage creates the tier-specific page. Tier must be
// "regional_keyless" or "self_hosted".
func NewTierSpecificPage(tier string) *TierSpecificPage {
	kslHost := common.NewTextInput("Keyless SSL Host", "keyless.example.com", "")
	kslPort := common.NewTextInput("Keyless SSL Port", "2407", "(default: 2407)")
	kslPort.Input.SetValue("2407")
	regional := common.NewToggle("Regional Services Enabled", "Restrict to FedRAMP US data centers", true)

	proxyAddr := common.NewTextInput("Proxy Listen Address", "0.0.0.0:443", "")
	proxyAddr.Input.SetValue("0.0.0.0:443")
	proxyCert := common.NewTextInput("TLS Certificate File", "/etc/pki/tls/certs/proxy.pem", "")
	proxyKey := common.NewTextInput("TLS Private Key File", "/etc/pki/tls/private/proxy-key.pem", "")
	proxyUp := common.NewTextInput("Upstream URL", "https://your-app.example.com", "")

	return &TierSpecificPage{
		tier:             tier,
		keylessSSLHost:   kslHost,
		keylessSSLPort:   kslPort,
		regionalServices: regional,
		proxyListenAddr:  proxyAddr,
		proxyCertFile:    proxyCert,
		proxyKeyFile:     proxyKey,
		proxyUpstream:    proxyUp,
	}
}

func (p *TierSpecificPage) Title() string {
	if p.tier == "regional_keyless" {
		return "Keyless SSL + Regional"
	}
	return "FIPS Proxy Settings"
}

func (p *TierSpecificPage) Init() tea.Cmd { return nil }

func (p *TierSpecificPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	// Tier 2 starts with a TextInput — return its focus command for cursor blink.
	// Tier 3 also starts with a TextInput.
	if p.tier == "regional_keyless" {
		return p.keylessSSLHost.Focus()
	}
	return p.proxyListenAddr.Focus()
}

func (p *TierSpecificPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *TierSpecificPage) fieldCount() int {
	if p.tier == "regional_keyless" {
		return 3 // host, port, regional
	}
	return 4 // addr, cert, key, upstream
}

func (p *TierSpecificPage) updateFocus() {
	p.keylessSSLHost.Blur()
	p.keylessSSLPort.Blur()
	p.regionalServices.Blur()
	p.proxyListenAddr.Blur()
	p.proxyCertFile.Blur()
	p.proxyKeyFile.Blur()
	p.proxyUpstream.Blur()

	if p.tier == "regional_keyless" {
		switch p.focus {
		case 0:
			p.keylessSSLHost.Focus()
		case 1:
			p.keylessSSLPort.Focus()
		case 2:
			p.regionalServices.Focus()
		}
	} else {
		switch p.focus {
		case 0:
			p.proxyListenAddr.Focus()
		case 1:
			p.proxyCertFile.Focus()
		case 2:
			p.proxyKeyFile.Focus()
		case 3:
			p.proxyUpstream.Focus()
		}
	}
}

func (p *TierSpecificPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
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
	if p.tier == "regional_keyless" {
		switch p.focus {
		case 0:
			cmd = p.keylessSSLHost.Update(msg)
		case 1:
			cmd = p.keylessSSLPort.Update(msg)
		case 2:
			p.regionalServices.Update(msg)
		}
	} else {
		switch p.focus {
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
	return p, cmd
}

func (p *TierSpecificPage) Validate() bool {
	if p.tier == "self_hosted" {
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

func (p *TierSpecificPage) Apply(cfg *config.Config) {
	if p.tier == "regional_keyless" {
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
	} else {
		cfg.ProxyListenAddr = strings.TrimSpace(p.proxyListenAddr.Value())
		cfg.ProxyCertFile = strings.TrimSpace(p.proxyCertFile.Value())
		cfg.ProxyKeyFile = strings.TrimSpace(p.proxyKeyFile.Value())
	}
}

func (p *TierSpecificPage) View() string {
	var b strings.Builder
	if p.tier == "regional_keyless" {
		b.WriteString(common.LabelStyle.Render("Tier 2 — Keyless SSL + Regional Services"))
		b.WriteString("\n\n")
		b.WriteString(p.keylessSSLHost.View())
		b.WriteString("\n\n")
		b.WriteString(p.keylessSSLPort.View())
		b.WriteString("\n\n")
		b.WriteString(p.regionalServices.View())
	} else {
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

package wizard

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// DashboardWiringPage is page 2: CF API token, zone/account/tunnel IDs, MDM.
type DashboardWiringPage struct {
	apiToken   common.TextInput
	zoneID     common.TextInput
	accountID  common.TextInput
	tunnelID   common.TextInput
	metricsAddr common.TextInput
	mdmProvider common.Selector

	// Conditional MDM fields
	intuneTenantID     common.TextInput
	intuneClientID     common.TextInput
	intuneClientSecret common.TextInput
	jamfBaseURL        common.TextInput
	jamfAPIToken       common.TextInput

	focus  int
	width  int
	height int
}

func (p *DashboardWiringPage) fieldCount() int {
	base := 6 // apiToken, zoneID, accountID, tunnelID, metricsAddr, mdmProvider
	switch p.mdmProvider.Selected() {
	case "intune":
		return base + 3
	case "jamf":
		return base + 2
	default:
		return base
	}
}

// NewDashboardWiringPage creates page 2.
func NewDashboardWiringPage() *DashboardWiringPage {
	apiToken := common.NewPasswordInput("Cloudflare API Token", "Bearer token", "(or set CF_API_TOKEN env var)")
	// Pre-fill from env
	if env := os.Getenv("CF_API_TOKEN"); env != "" {
		apiToken.SetValue(env)
	}

	zoneID := common.NewTextInput("Zone ID", "32-character hex", "")
	zoneID.Validate = config.ValidateOptionalHexID

	accountID := common.NewTextInput("Account ID", "32-character hex", "")
	accountID.Validate = config.ValidateOptionalHexID

	tunnelID := common.NewTextInput("Tunnel ID", "(auto-populated from page 1)", "")

	metricsAddr := common.NewTextInput("Metrics Address", "localhost:2000", "")
	metricsAddr.Input.SetValue("localhost:2000")
	metricsAddr.Validate = config.ValidateOptionalHostPort

	mdm := common.NewSelector("MDM Provider", []common.SelectorOption{
		{Value: "none", Label: "None", Description: "No MDM integration"},
		{Value: "intune", Label: "Microsoft Intune", Description: "Azure AD device posture via Graph API"},
		{Value: "jamf", Label: "Jamf Pro", Description: "Apple device management posture"},
	})

	intuneTenantID := common.NewTextInput("Intune Tenant ID", "Azure AD tenant UUID", "")
	intuneClientID := common.NewTextInput("Intune Client ID", "App registration client ID", "")
	intuneClientSecret := common.NewPasswordInput("Intune Client Secret", "App registration secret", "")

	jamfBaseURL := common.NewTextInput("Jamf Pro Base URL", "https://your-instance.jamfcloud.com", "")
	jamfAPIToken := common.NewPasswordInput("Jamf API Token", "Bearer token", "")

	return &DashboardWiringPage{
		apiToken:           apiToken,
		zoneID:             zoneID,
		accountID:          accountID,
		tunnelID:           tunnelID,
		metricsAddr:        metricsAddr,
		mdmProvider:        mdm,
		intuneTenantID:     intuneTenantID,
		intuneClientID:     intuneClientID,
		intuneClientSecret: intuneClientSecret,
		jamfBaseURL:        jamfBaseURL,
		jamfAPIToken:       jamfAPIToken,
	}
}

func (p *DashboardWiringPage) Title() string { return "Dashboard Wiring" }

func (p *DashboardWiringPage) Init() tea.Cmd { return nil }

func (p *DashboardWiringPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.apiToken.Focus()
}

func (p *DashboardWiringPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// PrePopulateTunnelID sets the tunnel ID from page 1.
func (p *DashboardWiringPage) PrePopulateTunnelID(id string) {
	p.tunnelID.SetValue(id)
}

func (p *DashboardWiringPage) updateFocus() {
	// Blur all
	p.apiToken.Blur()
	p.zoneID.Blur()
	p.accountID.Blur()
	p.tunnelID.Blur()
	p.metricsAddr.Blur()
	p.mdmProvider.Blur()
	p.intuneTenantID.Blur()
	p.intuneClientID.Blur()
	p.intuneClientSecret.Blur()
	p.jamfBaseURL.Blur()
	p.jamfAPIToken.Blur()

	switch p.focus {
	case 0:
		p.apiToken.Input.Focus()
	case 1:
		p.zoneID.Input.Focus()
	case 2:
		p.accountID.Input.Focus()
	case 3:
		p.tunnelID.Input.Focus()
	case 4:
		p.metricsAddr.Input.Focus()
	case 5:
		p.mdmProvider.Focus()
	default:
		// MDM conditional fields
		idx := p.focus - 6
		switch p.mdmProvider.Selected() {
		case "intune":
			switch idx {
			case 0:
				p.intuneTenantID.Input.Focus()
			case 1:
				p.intuneClientID.Input.Focus()
			case 2:
				p.intuneClientSecret.Input.Focus()
			}
		case "jamf":
			switch idx {
			case 0:
				p.jamfBaseURL.Input.Focus()
			case 1:
				p.jamfAPIToken.Input.Focus()
			}
		}
	}
}

func (p *DashboardWiringPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			if p.focus == 5 {
				// On selector, enter shouldn't advance — only tab
				break
			}
			if p.focus < p.fieldCount()-1 {
				p.focus++
				p.updateFocus()
				return p, nil
			}
			return p, nil
		case "shift+tab":
			if p.focus > 0 {
				p.focus--
				p.updateFocus()
				return p, nil
			}
			return p, nil
		}
	}

	var cmd tea.Cmd
	switch p.focus {
	case 0:
		cmd = p.apiToken.Update(msg)
	case 1:
		cmd = p.zoneID.Update(msg)
	case 2:
		cmd = p.accountID.Update(msg)
	case 3:
		cmd = p.tunnelID.Update(msg)
	case 4:
		cmd = p.metricsAddr.Update(msg)
	case 5:
		p.mdmProvider.Update(msg)
	default:
		idx := p.focus - 6
		switch p.mdmProvider.Selected() {
		case "intune":
			switch idx {
			case 0:
				cmd = p.intuneTenantID.Update(msg)
			case 1:
				cmd = p.intuneClientID.Update(msg)
			case 2:
				cmd = p.intuneClientSecret.Update(msg)
			}
		case "jamf":
			switch idx {
			case 0:
				cmd = p.jamfBaseURL.Update(msg)
			case 1:
				cmd = p.jamfAPIToken.Update(msg)
			}
		}
	}
	return p, cmd
}

func (p *DashboardWiringPage) Validate() bool {
	valid := true
	if !p.zoneID.RunValidation() {
		valid = false
	}
	if !p.accountID.RunValidation() {
		valid = false
	}
	if !p.metricsAddr.RunValidation() {
		valid = false
	}
	return valid
}

func (p *DashboardWiringPage) Apply(cfg *config.Config) {
	cfg.Dashboard.CFAPIToken = strings.TrimSpace(p.apiToken.Value())
	cfg.Dashboard.ZoneID = strings.TrimSpace(p.zoneID.Value())
	cfg.Dashboard.AccountID = strings.TrimSpace(p.accountID.Value())
	cfg.Dashboard.TunnelID = strings.TrimSpace(p.tunnelID.Value())
	cfg.Dashboard.MetricsAddress = strings.TrimSpace(p.metricsAddr.Value())

	provider := p.mdmProvider.Selected()
	cfg.Dashboard.MDM.Provider = provider

	switch provider {
	case "intune":
		cfg.Dashboard.MDM.TenantID = strings.TrimSpace(p.intuneTenantID.Value())
		cfg.Dashboard.MDM.ClientID = strings.TrimSpace(p.intuneClientID.Value())
		cfg.Dashboard.MDM.ClientSecret = strings.TrimSpace(p.intuneClientSecret.Value())
	case "jamf":
		cfg.Dashboard.MDM.BaseURL = strings.TrimSpace(p.jamfBaseURL.Value())
		cfg.Dashboard.MDM.APIToken = strings.TrimSpace(p.jamfAPIToken.Value())
	}
}

func (p *DashboardWiringPage) View() string {
	var b strings.Builder
	b.WriteString(p.apiToken.View())
	b.WriteString("\n\n")
	b.WriteString(p.zoneID.View())
	b.WriteString("\n\n")
	b.WriteString(p.accountID.View())
	b.WriteString("\n\n")
	b.WriteString(p.tunnelID.View())
	b.WriteString("\n\n")
	b.WriteString(p.metricsAddr.View())
	b.WriteString("\n\n")
	b.WriteString(p.mdmProvider.View())

	switch p.mdmProvider.Selected() {
	case "intune":
		b.WriteString("\n")
		b.WriteString(p.intuneTenantID.View())
		b.WriteString("\n\n")
		b.WriteString(p.intuneClientID.View())
		b.WriteString("\n\n")
		b.WriteString(p.intuneClientSecret.View())
	case "jamf":
		b.WriteString("\n")
		b.WriteString(p.jamfBaseURL.View())
		b.WriteString("\n\n")
		b.WriteString(p.jamfAPIToken.View())
	}

	return b.String()
}

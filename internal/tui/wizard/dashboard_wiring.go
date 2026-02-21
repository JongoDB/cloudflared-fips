package wizard

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/cfapi"
)

// Async messages for Cloudflare API discovery.
type tokenVerifiedMsg struct {
	accounts []cfapi.Account
	err      error
}

type zonesLoadedMsg struct {
	zones []cfapi.Zone
	err   error
}

type tunnelsLoadedMsg struct {
	tunnels []cfapi.TunnelInfo
	err     error
}

// DashboardWiringPage is page 2: CF API token, zone/account/tunnel IDs, MDM.
type DashboardWiringPage struct {
	apiToken    common.TextInput
	zoneID      common.TextInput
	accountID   common.TextInput
	tunnelID    common.TextInput
	metricsAddr common.TextInput
	mdmProvider common.Selector

	// API-discovered selectors (replace text inputs when data available)
	accountPicker common.Selector
	zonePicker    common.Selector
	tunnelPicker  common.Selector

	// Discovery state
	accounts       []cfapi.Account
	zones          []cfapi.Zone
	apiTunnels     []cfapi.TunnelInfo
	accountsLoaded bool
	zonesLoaded    bool
	tunnelsLoaded  bool
	discoveryErr   string
	fetching       bool

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

// hasAccounts returns true if API accounts were successfully loaded.
func (p *DashboardWiringPage) hasAccounts() bool {
	return p.accountsLoaded && len(p.accounts) > 0
}

// hasZones returns true if API zones were successfully loaded.
func (p *DashboardWiringPage) hasZones() bool {
	return p.zonesLoaded && len(p.zones) > 0
}

func (p *DashboardWiringPage) fieldCount() int {
	base := 6 // apiToken, zone, account, tunnelID, metricsAddr, mdmProvider
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
		accountPicker:      common.NewSelector("Account", nil),
		zonePicker:         common.NewSelector("Zone", nil),
		tunnelPicker:       common.NewSelector("Tunnel (API)", nil),
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
	p.apiToken.Blur()
	p.zoneID.Blur()
	p.accountID.Blur()
	p.tunnelID.Blur()
	p.metricsAddr.Blur()
	p.mdmProvider.Blur()
	p.accountPicker.Blur()
	p.zonePicker.Blur()
	p.tunnelPicker.Blur()
	p.intuneTenantID.Blur()
	p.intuneClientID.Blur()
	p.intuneClientSecret.Blur()
	p.jamfBaseURL.Blur()
	p.jamfAPIToken.Blur()

	switch p.focus {
	case 0:
		p.apiToken.Input.Focus()
	case 1:
		if p.hasZones() {
			p.zonePicker.Focus()
		} else {
			p.zoneID.Input.Focus()
		}
	case 2:
		if p.hasAccounts() {
			p.accountPicker.Focus()
		} else {
			p.accountID.Input.Focus()
		}
	case 3:
		p.tunnelID.Input.Focus()
	case 4:
		p.metricsAddr.Input.Focus()
	case 5:
		p.mdmProvider.Focus()
	default:
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
	// Handle async API discovery messages
	switch msg := msg.(type) {
	case tokenVerifiedMsg:
		p.fetching = false
		if msg.err != nil {
			p.discoveryErr = msg.err.Error()
			return p, nil
		}
		p.accounts = msg.accounts
		p.accountsLoaded = true
		p.discoveryErr = ""

		// Build account picker
		opts := make([]common.SelectorOption, len(msg.accounts))
		for i, a := range msg.accounts {
			opts[i] = common.SelectorOption{
				Value:       a.ID,
				Label:       a.Name,
				Description: a.ID,
			}
		}
		p.accountPicker = common.NewSelector("Account", opts)

		// Auto-fill account ID if only one
		if len(msg.accounts) == 1 {
			p.accountID.SetValue(msg.accounts[0].ID)
			// Fetch zones for first account
			return p, p.fetchZones(msg.accounts[0].ID)
		}
		return p, nil

	case zonesLoadedMsg:
		if msg.err != nil {
			// Don't overwrite a more serious error
			if p.discoveryErr == "" {
				p.discoveryErr = "zones: " + msg.err.Error()
			}
			return p, nil
		}
		p.zones = msg.zones
		p.zonesLoaded = true

		opts := make([]common.SelectorOption, len(msg.zones))
		for i, z := range msg.zones {
			opts[i] = common.SelectorOption{
				Value:       z.ID,
				Label:       z.Name,
				Description: z.ID + " (" + z.Status + ")",
			}
		}
		p.zonePicker = common.NewSelector("Zone", opts)

		// Auto-fill zone if only one
		if len(msg.zones) == 1 {
			p.zoneID.SetValue(msg.zones[0].ID)
		}

		// Update focus if currently on zone field
		if p.focus == 1 {
			p.updateFocus()
		}
		return p, nil

	case tunnelsLoadedMsg:
		if msg.err != nil {
			return p, nil
		}
		p.apiTunnels = msg.tunnels
		p.tunnelsLoaded = true

		opts := make([]common.SelectorOption, len(msg.tunnels))
		for i, t := range msg.tunnels {
			opts[i] = common.SelectorOption{
				Value:       t.ID,
				Label:       t.Name,
				Description: t.ID,
			}
		}
		p.tunnelPicker = common.NewSelector("Tunnel (API)", opts)
		return p, nil
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// When leaving the API token field, trigger discovery
			if p.focus == 0 && strings.TrimSpace(p.apiToken.Value()) != "" && !p.accountsLoaded && !p.fetching {
				p.focus = 1
				p.updateFocus()
				return p, p.startDiscovery()
			}

			// When selecting an account from picker, fetch zones
			if p.focus == 2 && p.hasAccounts() {
				selectedID := p.accountPicker.Selected()
				if selectedID != "" {
					p.accountID.SetValue(selectedID)
					if !p.zonesLoaded {
						p.focus = 1
						p.updateFocus()
						return p, p.fetchZones(selectedID)
					}
				}
			}

			// When selecting a zone from picker, fill zone ID
			if p.focus == 1 && p.hasZones() {
				selectedID := p.zonePicker.Selected()
				if selectedID != "" {
					p.zoneID.SetValue(selectedID)
					// Also fill account ID from zones data
					for _, z := range p.zones {
						if z.ID == selectedID {
							break
						}
					}
				}
			}

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
		if p.hasZones() {
			p.zonePicker.Update(msg)
		} else {
			cmd = p.zoneID.Update(msg)
		}
	case 2:
		if p.hasAccounts() {
			p.accountPicker.Update(msg)
		} else {
			cmd = p.accountID.Update(msg)
		}
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

// startDiscovery verifies the token and fetches accounts.
func (p *DashboardWiringPage) startDiscovery() tea.Cmd {
	token := strings.TrimSpace(p.apiToken.Value())
	p.fetching = true
	p.discoveryErr = ""
	return func() tea.Msg {
		client := cfapi.NewClient(token)
		if err := client.VerifyToken(); err != nil {
			return tokenVerifiedMsg{err: err}
		}
		accounts, err := client.ListAccounts()
		return tokenVerifiedMsg{accounts: accounts, err: err}
	}
}

// fetchZones fetches zones for the given account.
func (p *DashboardWiringPage) fetchZones(accountID string) tea.Cmd {
	token := strings.TrimSpace(p.apiToken.Value())
	return func() tea.Msg {
		client := cfapi.NewClient(token)
		zones, err := client.ListZones(accountID)
		return zonesLoadedMsg{zones: zones, err: err}
	}
}

func (p *DashboardWiringPage) Validate() bool {
	valid := true
	// If using pickers, values come from selected items — skip hex validation
	if !p.hasZones() {
		if !p.zoneID.RunValidation() {
			valid = false
		}
	}
	if !p.hasAccounts() {
		if !p.accountID.RunValidation() {
			valid = false
		}
	}
	if !p.metricsAddr.RunValidation() {
		valid = false
	}
	return valid
}

func (p *DashboardWiringPage) Apply(cfg *config.Config) {
	cfg.Dashboard.CFAPIToken = strings.TrimSpace(p.apiToken.Value())

	// Zone: prefer picker selection
	if p.hasZones() {
		cfg.Dashboard.ZoneID = p.zonePicker.Selected()
	} else {
		cfg.Dashboard.ZoneID = strings.TrimSpace(p.zoneID.Value())
	}

	// Account: prefer picker selection
	if p.hasAccounts() {
		cfg.Dashboard.AccountID = p.accountPicker.Selected()
	} else {
		cfg.Dashboard.AccountID = strings.TrimSpace(p.accountID.Value())
	}

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
	b.WriteString("\n")

	// Show discovery status
	if p.fetching {
		b.WriteString(common.HintStyle.Render("  Verifying token and loading accounts..."))
		b.WriteString("\n")
	}
	if p.discoveryErr != "" {
		b.WriteString(common.ErrorStyle.Render("  ! "+p.discoveryErr))
		b.WriteString("\n")
	}
	if p.accountsLoaded && len(p.accounts) > 0 {
		b.WriteString(common.SuccessStyle.Render("  Token verified"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Zone: picker or text input
	if p.hasZones() {
		b.WriteString(p.zonePicker.View())
	} else {
		b.WriteString(p.zoneID.View())
	}
	b.WriteString("\n\n")

	// Account: picker or text input
	if p.hasAccounts() && len(p.accounts) > 1 {
		b.WriteString(p.accountPicker.View())
	} else if p.hasAccounts() {
		// Single account — show as read-only info
		b.WriteString(common.LabelStyle.Render("Account"))
		b.WriteString("\n")
		b.WriteString(common.HintStyle.Render("  " + p.accounts[0].Name + " (" + p.accounts[0].ID + ")"))
	} else {
		b.WriteString(p.accountID.View())
	}
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

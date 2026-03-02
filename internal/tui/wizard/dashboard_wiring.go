package wizard

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/cfapi"
)

// Async messages for Cloudflare API discovery.
type tokenVerifiedMsg struct {
	accounts []cfapi.Account
	zones    []cfapi.Zone // populated when accounts discovered via zone fallback
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

type tunnelCreatedMsg struct {
	tunnel *cfapi.TunnelWithToken
	err    error
}

// DashboardWiringPage collects CF API token, zone/account/tunnel selection,
// tunnel creation, public hostname, and MDM config.
type DashboardWiringPage struct {
	apiToken    common.TextInput
	zoneID      common.TextInput
	accountID   common.TextInput
	metricsAddr common.TextInput
	mdmProvider common.Selector

	// API-discovered selectors (replace text inputs when data available)
	accountPicker common.Selector
	zonePicker    common.Selector
	tunnelPicker  common.Selector

	// Discovery state
	accounts         []cfapi.Account
	zones            []cfapi.Zone
	apiTunnels       []cfapi.TunnelInfo
	accountsLoaded   bool
	zonesLoaded      bool
	tunnelsLoaded    bool
	discoveryErr     string
	fetching         bool
	tokenAtLastVerify string // tracks token value at last discovery attempt

	// Tunnel creation
	tunnelNameInput    common.TextInput
	creatingTunnel     bool
	tunnelCreated      bool
	createdTunnelToken string
	createdTunnelID    string
	createdTunnelName  string
	tunnelCreateErr    string

	// Public hostname configuration
	hostnameSubdomain common.TextInput
	hostnameService   common.TextInput

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

// Field indices:
//
//	0: apiToken
//	1: zone (picker or text)
//	2: account (picker or text)
//	3: tunnel (picker with "Create New" or text input for name if creating)
//	4: hostnameSubdomain
//	5: hostnameService
//	6: metricsAddr
//	7: mdmProvider
//	8+: MDM-specific fields

const dashWiringBaseFields = 8

// hasAccounts returns true if API accounts were successfully loaded.
func (p *DashboardWiringPage) hasAccounts() bool {
	return p.accountsLoaded && len(p.accounts) > 0
}

// hasZones returns true if API zones were successfully loaded.
func (p *DashboardWiringPage) hasZones() bool {
	return p.zonesLoaded && len(p.zones) > 0
}

// hasTunnels returns true if API tunnels were successfully loaded.
func (p *DashboardWiringPage) hasTunnels() bool {
	return p.tunnelsLoaded
}

// isCreatingNewTunnel returns true if user selected "Create New Tunnel".
func (p *DashboardWiringPage) isCreatingNewTunnel() bool {
	return p.hasTunnels() && p.tunnelPicker.Selected() == "__create_new__"
}

func (p *DashboardWiringPage) fieldCount() int {
	base := dashWiringBaseFields
	switch p.mdmProvider.Selected() {
	case "intune":
		return base + 3
	case "jamf":
		return base + 2
	default:
		return base
	}
}

// selectedZoneName returns the domain name for the selected zone.
func (p *DashboardWiringPage) selectedZoneName() string {
	if !p.hasZones() {
		return ""
	}
	selectedID := p.zonePicker.Selected()
	for _, z := range p.zones {
		if z.ID == selectedID {
			return z.Name
		}
	}
	return ""
}

// NewDashboardWiringPage creates the dashboard wiring page.
func NewDashboardWiringPage() *DashboardWiringPage {
	apiToken := common.NewPasswordInput("Cloudflare API Token", "Bearer token", "(or set CF_API_TOKEN env var)")
	apiToken.HelpText = "Create at: https://dash.cloudflare.com/profile/api-tokens\nRequired permissions (Resource | Permission | Access):\n  Account | Cloudflare Tunnel | Edit\n  Zone    | Zone              | Read\n  Zone    | DNS               | Edit\n  Zone    | Access: Apps and Policies | Read\n  Zone    | SSL and Certificates     | Read\nThe token is validated automatically when you leave this field."
	if env := os.Getenv("CF_API_TOKEN"); env != "" {
		apiToken.SetValue(env)
	}

	zoneID := common.NewTextInput("Zone ID", "32-character hex", "")
	zoneID.Validate = config.ValidateOptionalHexID
	zoneID.HelpText = "Find in Cloudflare dashboard: select your domain → Overview →\nright sidebar under 'API'. Will auto-populate if your token has\naccess to zones."

	accountID := common.NewTextInput("Account ID", "32-character hex", "")
	accountID.Validate = config.ValidateOptionalHexID
	accountID.HelpText = "Find at: https://dash.cloudflare.com → select account →\nOverview sidebar. Auto-discovered from your API token."

	tunnelName := common.NewTextInput("New Tunnel Name", "my-fips-tunnel", "")
	tunnelName.Validate = config.ValidateNonEmpty
	tunnelName.HelpText = "Name for the new Cloudflare Tunnel.\nThe tunnel will be created via the API — no dashboard visit required."

	hostnameSubdomain := common.NewTextInput("Subdomain", "dashboard", "(combined with your zone domain)")
	hostnameSubdomain.HelpText = "Type just the subdomain — the domain comes from your zone selection above.\nExample: type 'dashboard' → becomes dashboard.yourdomain.com\nA CNAME record and tunnel ingress rule will be created automatically."

	hostnameService := common.NewTextInput("Backend Service URL", "http://localhost:8080", "")
	hostnameService.Input.SetValue("http://localhost:8080")
	hostnameService.HelpText = "Local service URL that the tunnel routes traffic to.\nDefaults to the dashboard server at http://localhost:8080."

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
		metricsAddr:        metricsAddr,
		mdmProvider:        mdm,
		accountPicker:      common.NewSelector("Account", nil),
		zonePicker:         common.NewSelector("Zone", nil),
		tunnelPicker:       common.NewSelector("Tunnel", nil),
		tunnelNameInput:    tunnelName,
		hostnameSubdomain:  hostnameSubdomain,
		hostnameService:    hostnameService,
		intuneTenantID:     intuneTenantID,
		intuneClientID:     intuneClientID,
		intuneClientSecret: intuneClientSecret,
		jamfBaseURL:        jamfBaseURL,
		jamfAPIToken:       jamfAPIToken,
	}
}

func (p *DashboardWiringPage) Title() string { return "Dashboard & Tunnel" }

func (p *DashboardWiringPage) Init() tea.Cmd { return nil }

func (p *DashboardWiringPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	cmds := []tea.Cmd{p.apiToken.Focus()}
	// Auto-trigger discovery if token is pre-populated (e.g. from CF_API_TOKEN env var)
	if strings.TrimSpace(p.apiToken.Value()) != "" && !p.accountsLoaded && !p.fetching {
		cmds = append(cmds, p.startDiscovery())
	}
	return tea.Batch(cmds...)
}

func (p *DashboardWiringPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// PrePopulateTunnelID is a no-op now; tunnel ID comes from tunnel picker/creation.
func (p *DashboardWiringPage) PrePopulateTunnelID(_ string) {}

func (p *DashboardWiringPage) updateFocus() {
	p.apiToken.Blur()
	p.zoneID.Blur()
	p.accountID.Blur()
	p.metricsAddr.Blur()
	p.mdmProvider.Blur()
	p.accountPicker.Blur()
	p.zonePicker.Blur()
	p.tunnelPicker.Blur()
	p.tunnelNameInput.Blur()
	p.hostnameSubdomain.Blur()
	p.hostnameService.Blur()
	p.intuneTenantID.Blur()
	p.intuneClientID.Blur()
	p.intuneClientSecret.Blur()
	p.jamfBaseURL.Blur()
	p.jamfAPIToken.Blur()

	switch p.focus {
	case 0:
		p.apiToken.Focus()
	case 1:
		if p.hasZones() {
			p.zonePicker.Focus()
		} else {
			p.zoneID.Focus()
		}
	case 2:
		if p.hasAccounts() {
			p.accountPicker.Focus()
		} else {
			p.accountID.Focus()
		}
	case 3:
		if p.isCreatingNewTunnel() {
			p.tunnelNameInput.Focus()
		} else if p.hasTunnels() {
			p.tunnelPicker.Focus()
		} else {
			p.tunnelNameInput.Focus()
		}
	case 4:
		p.hostnameSubdomain.Focus()
	case 5:
		p.hostnameService.Focus()
	case 6:
		p.metricsAddr.Focus()
	case 7:
		p.mdmProvider.Focus()
	default:
		idx := p.focus - dashWiringBaseFields
		switch p.mdmProvider.Selected() {
		case "intune":
			switch idx {
			case 0:
				p.intuneTenantID.Focus()
			case 1:
				p.intuneClientID.Focus()
			case 2:
				p.intuneClientSecret.Focus()
			}
		case "jamf":
			switch idx {
			case 0:
				p.jamfBaseURL.Focus()
			case 1:
				p.jamfAPIToken.Focus()
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

		// If zones were pre-fetched (fallback discovery path), use them directly
		if len(msg.zones) > 0 {
			// Auto-fill account if only one
			if len(msg.accounts) == 1 {
				p.accountID.SetValue(msg.accounts[0].ID)
			}
			// Inject pre-fetched zones as if zonesLoadedMsg arrived
			return p, func() tea.Msg {
				return zonesLoadedMsg{zones: msg.zones}
			}
		}

		// Normal path: auto-fill account ID if only one, then fetch zones
		if len(msg.accounts) == 1 {
			p.accountID.SetValue(msg.accounts[0].ID)
			return p, p.fetchZones(msg.accounts[0].ID)
		}
		return p, nil

	case zonesLoadedMsg:
		if msg.err != nil {
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

		// Auto-fetch tunnels for the account
		accountID := strings.TrimSpace(p.accountID.Value())
		if accountID != "" && !p.tunnelsLoaded {
			return p, p.fetchTunnels(accountID)
		}
		return p, nil

	case tunnelsLoadedMsg:
		if msg.err != nil {
			return p, nil
		}
		p.apiTunnels = msg.tunnels
		p.tunnelsLoaded = true

		// Build tunnel picker: existing tunnels + "Create New"
		opts := make([]common.SelectorOption, 0, len(msg.tunnels)+1)
		for _, t := range msg.tunnels {
			status := t.Status
			if status == "" {
				status = "unknown"
			}
			opts = append(opts, common.SelectorOption{
				Value:       t.ID,
				Label:       t.Name,
				Description: t.ID[:minLen(t.ID, 12)] + "... (" + status + ")",
			})
		}
		opts = append(opts, common.SelectorOption{
			Value:       "__create_new__",
			Label:       "+ Create New Tunnel",
			Description: "Create a new tunnel via the API",
		})
		p.tunnelPicker = common.NewSelector("Tunnel", opts)

		// If focus is on tunnel field, update display
		if p.focus == 3 {
			p.updateFocus()
		}
		return p, nil

	case tunnelCreatedMsg:
		p.creatingTunnel = false
		if msg.err != nil {
			p.tunnelCreateErr = msg.err.Error()
			return p, nil
		}
		p.tunnelCreated = true
		p.createdTunnelToken = msg.tunnel.Token
		p.createdTunnelID = msg.tunnel.ID
		p.createdTunnelName = msg.tunnel.Name
		p.tunnelCreateErr = ""
		return p, nil
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// When leaving the API token field, trigger discovery if:
			// - token is non-empty, and
			// - either never verified, or token changed since last verify
			if p.focus == 0 && strings.TrimSpace(p.apiToken.Value()) != "" && !p.fetching {
				currentToken := strings.TrimSpace(p.apiToken.Value())
				needsVerify := !p.accountsLoaded || currentToken != p.tokenAtLastVerify
				if needsVerify {
					p.resetDiscoveryState()
					p.focus = 1
					p.updateFocus()
					return p, p.startDiscovery()
				}
				// Token already verified and unchanged — just advance
				p.focus = 1
				p.updateFocus()
				return p, fieldNav
			}

			// When selecting an account from picker, fetch zones + tunnels
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

			// When selecting a zone from picker, fill zone ID and update hostname hint
			if p.focus == 1 && p.hasZones() {
				selectedID := p.zonePicker.Selected()
				if selectedID != "" {
					p.zoneID.SetValue(selectedID)
					// Update hostname subdomain hint with zone name
					if zoneName := p.selectedZoneName(); zoneName != "" {
						p.hostnameSubdomain.Hint = fmt.Sprintf("(.%s)", zoneName)
					}
				}
			}

			// Tunnel picker: when "Create New" is selected and user presses enter
			// on the tunnel name field, trigger creation
			if p.focus == 3 && p.isCreatingNewTunnel() && msg.String() == "enter" {
				name := strings.TrimSpace(p.tunnelNameInput.Value())
				accountID := strings.TrimSpace(p.accountID.Value())
				if name != "" && accountID != "" && !p.creatingTunnel && !p.tunnelCreated {
					return p, p.createTunnel(accountID, name)
				}
				// If already created or creating, just advance
				if p.tunnelCreated {
					p.focus++
					p.updateFocus()
					return p, fieldNav
				}
				return p, fieldNav
			}

			// Tunnel picker: enter selects within the selector
			if p.focus == 3 && p.hasTunnels() && !p.isCreatingNewTunnel() && msg.String() == "enter" {
				return p, fieldNav
			}

			// mdmProvider selector: enter selects within, tab advances
			if p.focus == 7 && msg.String() == "enter" {
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
		oldVal := p.apiToken.Value()
		cmd = p.apiToken.Update(msg)
		newVal := p.apiToken.Value()
		// If user edited the token after a previous verify attempt, reset discovery state
		if oldVal != newVal && p.tokenAtLastVerify != "" {
			p.resetDiscoveryState()
		}
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
		if p.isCreatingNewTunnel() {
			cmd = p.tunnelNameInput.Update(msg)
		} else if p.hasTunnels() {
			p.tunnelPicker.Update(msg)
		} else {
			cmd = p.tunnelNameInput.Update(msg)
		}
	case 4:
		cmd = p.hostnameSubdomain.Update(msg)
	case 5:
		cmd = p.hostnameService.Update(msg)
	case 6:
		cmd = p.metricsAddr.Update(msg)
	case 7:
		p.mdmProvider.Update(msg)
	default:
		idx := p.focus - dashWiringBaseFields
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
// If the token lacks Account-level permission (GET /accounts returns empty),
// it falls back to listing all zones and extracting accounts from the zone data.
func (p *DashboardWiringPage) startDiscovery() tea.Cmd {
	token := strings.TrimSpace(p.apiToken.Value())
	p.fetching = true
	p.discoveryErr = ""
	p.tokenAtLastVerify = token
	return func() tea.Msg {
		client := cfapi.NewClient(token)
		if err := client.VerifyToken(); err != nil {
			return tokenVerifiedMsg{err: err}
		}
		accounts, err := client.ListAccounts()
		if err != nil {
			return tokenVerifiedMsg{err: err}
		}
		// If accounts are empty, the token likely lacks Account resource permission.
		// Fall back: list all zones (which include embedded account info).
		if len(accounts) == 0 {
			zones, zErr := client.ListAllZones()
			if zErr != nil {
				return tokenVerifiedMsg{err: zErr}
			}
			accounts = cfapi.DiscoverAccountsFromZones(zones)
			return tokenVerifiedMsg{accounts: accounts, zones: zones}
		}
		return tokenVerifiedMsg{accounts: accounts}
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

// fetchTunnels fetches tunnels for the given account.
func (p *DashboardWiringPage) fetchTunnels(accountID string) tea.Cmd {
	token := strings.TrimSpace(p.apiToken.Value())
	return func() tea.Msg {
		client := cfapi.NewClient(token)
		tunnels, err := client.ListTunnels(accountID)
		return tunnelsLoadedMsg{tunnels: tunnels, err: err}
	}
}

// createTunnel creates a new tunnel via the API.
func (p *DashboardWiringPage) createTunnel(accountID, name string) tea.Cmd {
	token := strings.TrimSpace(p.apiToken.Value())
	p.creatingTunnel = true
	p.tunnelCreateErr = ""
	return func() tea.Msg {
		client := cfapi.NewClient(token)
		tunnel, err := client.CreateTunnel(accountID, name)
		return tunnelCreatedMsg{tunnel: tunnel, err: err}
	}
}

func (p *DashboardWiringPage) ScrollOffset() int {
	// Approximate line offsets per field in the View() output.
	offsets := []int{0, 5, 10, 15, 22, 27, 32, 37}
	if p.focus < len(offsets) {
		return offsets[p.focus]
	}
	// MDM-specific fields start after the selector.
	return 42 + (p.focus-dashWiringBaseFields)*5
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

	// Tunnel: from created tunnel or picker selection
	if p.tunnelCreated {
		cfg.TunnelToken = p.createdTunnelToken
		cfg.Dashboard.TunnelID = p.createdTunnelID
	} else if p.hasTunnels() {
		selected := p.tunnelPicker.Selected()
		if selected != "__create_new__" {
			cfg.Dashboard.TunnelID = selected
		}
	}

	// Public hostname
	subdomain := strings.TrimSpace(p.hostnameSubdomain.Value())
	if subdomain != "" {
		zoneName := p.selectedZoneName()
		if zoneName != "" {
			cfg.PublicHostname = subdomain + "." + zoneName
		} else {
			cfg.PublicHostname = subdomain
		}
	}
	cfg.HostnameService = strings.TrimSpace(p.hostnameService.Value())

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

	// Enable CF API integration for controller if token is set
	if cfg.Dashboard.CFAPIToken != "" {
		cfg.WithCF = true
	}
}

// friendlyTokenError translates raw API errors into user-friendly messages.
func friendlyTokenError(rawErr string) string {
	switch {
	case strings.Contains(rawErr, "Invalid API Token") || strings.Contains(rawErr, "code 1000"):
		return "Invalid API token — check that you copied it correctly"
	case strings.Contains(rawErr, "code 6003"):
		return "Missing required permissions — token needs Zone:Read, Tunnel:Edit, etc."
	case strings.Contains(rawErr, "API request failed:"):
		// Network-level errors (dial, timeout, TLS, etc.)
		return "Cannot reach Cloudflare API — check network connectivity"
	case strings.Contains(rawErr, "rate limited"):
		return "Rate limited by Cloudflare — wait a moment and try again"
	default:
		return "Token verification failed: " + rawErr
	}
}

// resetDiscoveryState clears all API discovery state so re-verification can proceed.
func (p *DashboardWiringPage) resetDiscoveryState() {
	p.discoveryErr = ""
	p.fetching = false
	p.accounts = nil
	p.zones = nil
	p.apiTunnels = nil
	p.accountsLoaded = false
	p.zonesLoaded = false
	p.tunnelsLoaded = false
	p.tunnelCreated = false
	p.creatingTunnel = false
	p.tunnelCreateErr = ""
	p.createdTunnelToken = ""
	p.createdTunnelID = ""
	p.createdTunnelName = ""
}

// discoveryStatusView renders the progressive API discovery status block.
func (p *DashboardWiringPage) discoveryStatusView() string {
	token := strings.TrimSpace(p.apiToken.Value())
	if token == "" && !p.fetching && p.discoveryErr == "" && !p.accountsLoaded {
		return "" // nothing to show before user enters a token
	}

	var b strings.Builder

	// Error state — show friendly message with tip
	if p.discoveryErr != "" {
		b.WriteString(common.ErrorStyle.Render("  \u2717 " + friendlyTokenError(p.discoveryErr)))
		b.WriteString("\n")
		b.WriteString(common.HintStyle.Render("    Tip: Create a token at https://dash.cloudflare.com/profile/api-tokens"))
		b.WriteString("\n")
		return b.String()
	}

	// Currently verifying (no accounts yet)
	if p.fetching && !p.accountsLoaded {
		b.WriteString(common.HintStyle.Render("  \u27f3 Verifying token..."))
		b.WriteString("\n")
		return b.String()
	}

	// Accounts loaded — show progressive results
	if p.accountsLoaded && len(p.accounts) > 0 {
		// Line 1: token + accounts
		acctNames := make([]string, 0, len(p.accounts))
		for _, a := range p.accounts {
			acctNames = append(acctNames, a.Name)
		}
		acctSuffix := fmt.Sprintf("%d account", len(p.accounts))
		if len(p.accounts) != 1 {
			acctSuffix += "s"
		}
		acctSuffix += " (" + strings.Join(acctNames, ", ") + ")"
		b.WriteString(common.SuccessStyle.Render("  \u2713 Token verified — "+acctSuffix))
		b.WriteString("\n")

		// Line 2: zones status
		if p.zonesLoaded {
			zoneCount := len(p.zones)
			zoneStr := fmt.Sprintf("%d zone", zoneCount)
			if zoneCount != 1 {
				zoneStr += "s"
			}
			if zoneCount > 0 {
				zoneNames := make([]string, 0, len(p.zones))
				for _, z := range p.zones {
					zoneNames = append(zoneNames, z.Name)
				}
				zoneStr += " (" + strings.Join(zoneNames, ", ") + ")"
			}
			b.WriteString(common.SuccessStyle.Render("  \u2713 " + zoneStr + " loaded"))
			b.WriteString("\n")
		} else {
			b.WriteString(common.HintStyle.Render("  \u27f3 Loading zones..."))
			b.WriteString("\n")
		}

		// Line 3: tunnels status
		if p.tunnelsLoaded {
			tunnelCount := len(p.apiTunnels)
			tunnelStr := fmt.Sprintf("%d tunnel", tunnelCount)
			if tunnelCount != 1 {
				tunnelStr += "s"
			}
			tunnelStr += " found"
			b.WriteString(common.SuccessStyle.Render("  \u2713 " + tunnelStr))
			b.WriteString("\n")
		} else if p.zonesLoaded {
			b.WriteString(common.HintStyle.Render("  \u27f3 Loading tunnels..."))
			b.WriteString("\n")
		}
	} else if p.accountsLoaded && len(p.accounts) == 0 {
		b.WriteString(common.WarningStyle.Render("  \u2717 No accounts found — token may lack Account permissions"))
		b.WriteString("\n")
	}

	return b.String()
}

func (p *DashboardWiringPage) View() string {
	var b strings.Builder
	b.WriteString(p.apiToken.View())
	b.WriteString("\n")

	// Show progressive discovery status
	statusView := p.discoveryStatusView()
	if statusView != "" {
		b.WriteString(statusView)
	}
	b.WriteString("\n")

	// Domain (Zone): picker, loading state, or manual input
	b.WriteString(common.LabelStyle.Render("Domain (Zone)"))
	b.WriteString("\n")
	if p.hasZones() {
		// Zone picker loaded — show radio selector (without duplicate label)
		for i, opt := range p.zonePicker.Options {
			selected := i == p.zonePicker.Cursor
			cursor := "  "
			radio := common.MutedStyle.Render("\u25cb")
			if selected {
				if p.zonePicker.Focused {
					cursor = common.FocusedPrompt
					radio = lipgloss.NewStyle().Bold(true).Foreground(common.ColorPrimary).Render("\u25cf")
				} else {
					radio = lipgloss.NewStyle().Foreground(common.ColorPrimary).Render("\u25cf")
				}
			}
			label := opt.Label
			if selected && p.zonePicker.Focused {
				label = lipgloss.NewStyle().Bold(true).Foreground(common.ColorWhite).Render(label)
			} else if selected {
				label = lipgloss.NewStyle().Foreground(common.ColorWhite).Render(label)
			} else {
				label = lipgloss.NewStyle().Foreground(common.ColorDim).Render(label)
			}
			b.WriteString(cursor + radio + " " + label + "\n")
			if opt.Description != "" {
				descStyle := common.HintStyle
				if selected && p.zonePicker.Focused {
					descStyle = lipgloss.NewStyle().Foreground(common.ColorDim)
				}
				b.WriteString(descStyle.Render("    "+opt.Description) + "\n")
			}
		}
	} else if p.accountsLoaded && !p.zonesLoaded {
		b.WriteString(common.HintStyle.Render("  \u27f3 Loading your domains..."))
		b.WriteString("\n")
	} else if p.zonesLoaded && len(p.zones) == 0 {
		b.WriteString(common.ErrorStyle.Render("  \u2717 No zones found — ensure your token has Zone:Read permission"))
		b.WriteString("\n")
		b.WriteString(p.zoneID.View())
	} else if !p.accountsLoaded && !p.fetching && p.discoveryErr == "" {
		b.WriteString(common.HintStyle.Render("  \u25cb Verify your API token first"))
		b.WriteString("\n")
	} else if p.discoveryErr != "" {
		b.WriteString(common.HintStyle.Render("  \u25cb Fix your API token to load domains"))
		b.WriteString("\n")
	} else {
		// Fallback: manual zone ID entry
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
		b.WriteString(common.SuccessStyle.Render("  \u2713 " + p.accounts[0].Name))
		b.WriteString(common.HintStyle.Render("  " + p.accounts[0].ID))
	} else if p.fetching && !p.accountsLoaded {
		b.WriteString(common.LabelStyle.Render("Account"))
		b.WriteString("\n")
		b.WriteString(common.HintStyle.Render("  \u27f3 Discovering..."))
	} else if !p.accountsLoaded && p.discoveryErr == "" && !p.fetching {
		b.WriteString(common.LabelStyle.Render("Account"))
		b.WriteString("\n")
		b.WriteString(common.HintStyle.Render("  \u25cb Verify your API token first"))
	} else {
		b.WriteString(p.accountID.View())
	}
	b.WriteString("\n\n")

	// Tunnel section
	b.WriteString(common.LabelStyle.Render("Cloudflare Tunnel"))
	b.WriteString("\n")
	if p.hasTunnels() {
		b.WriteString(p.tunnelPicker.View())
		if p.isCreatingNewTunnel() {
			b.WriteString("\n")
			b.WriteString(p.tunnelNameInput.View())
			if p.creatingTunnel {
				b.WriteString("\n")
				b.WriteString(common.HintStyle.Render("  Creating tunnel..."))
			}
			if p.tunnelCreateErr != "" {
				b.WriteString("\n")
				b.WriteString(common.ErrorStyle.Render("  ! "+p.tunnelCreateErr))
			}
			if p.tunnelCreated {
				b.WriteString("\n")
				b.WriteString(common.SuccessStyle.Render(fmt.Sprintf("  Tunnel created: %s (%s)", p.createdTunnelName, p.createdTunnelID[:minLen(p.createdTunnelID, 12)]+"...")))
				b.WriteString("\n")
				b.WriteString(common.HintStyle.Render("  Token generated automatically"))
			}
		}
	} else if p.accountsLoaded && !p.tunnelsLoaded {
		b.WriteString(common.HintStyle.Render("  \u27f3 Loading tunnels..."))
	} else if !p.accountsLoaded && !p.fetching {
		b.WriteString(common.HintStyle.Render("  \u25cb Verify your API token first"))
	} else {
		// No tunnels loaded — show name input for creation
		b.WriteString(p.tunnelNameInput.View())
		b.WriteString("\n")
		b.WriteString(common.HintStyle.Render("  Enter a name to create a new tunnel via API"))
	}
	b.WriteString("\n\n")

	// Public hostname
	b.WriteString(common.LabelStyle.Render("Public Hostname"))
	if zoneName := p.selectedZoneName(); zoneName != "" {
		subdomain := strings.TrimSpace(p.hostnameSubdomain.Value())
		if subdomain != "" {
			b.WriteString(common.SuccessStyle.Render(fmt.Sprintf("  → %s.%s", subdomain, zoneName)))
		} else {
			b.WriteString(common.HintStyle.Render(fmt.Sprintf("  (type subdomain for .%s)", zoneName)))
		}
		// Update the input hint to show the domain suffix
		p.hostnameSubdomain.Hint = fmt.Sprintf("(.%s)", zoneName)
	}
	b.WriteString("\n")
	b.WriteString(p.hostnameSubdomain.View())
	b.WriteString("\n\n")
	b.WriteString(p.hostnameService.View())
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

// minLen returns the minimum of len(s) and n.
func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}

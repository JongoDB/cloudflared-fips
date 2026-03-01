package wizard

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// loginCompleteMsg is sent when the interactive cloudflared login finishes.
type loginCompleteMsg struct{ err error }

// Async messages for tunnel CLI operations.
type tunnelsListedMsg struct {
	tunnels []common.CLITunnel
	err     error
}

type tunnelCreatedMsg struct {
	uuid      string
	credsPath string
	err       error
}

// tunnelSource selects how the tunnel is configured.
type tunnelSource int

const (
	tunnelSourceSelect tunnelSource = iota
	tunnelSourceCreate
	tunnelSourceManual
)

// TunnelPage is page 1: tunnel UUID, credentials, protocol, ingress rules.
type TunnelPage struct {
	source      common.Selector
	tunnelID    common.TextInput
	credentials common.TextInput
	protocol    common.Selector
	ingress     common.IngressEditor

	// "Select existing" state
	tunnelList    []common.CLITunnel
	tunnelPicker  common.Selector
	tunnelsLoaded bool
	tunnelErr     string

	// "Create new" state
	tunnelName common.TextInput
	creating   bool
	createErr  string

	// Whether cloudflared is on PATH
	hasCLI bool
	// Whether the error is due to missing cert.pem (needs cloudflared login)
	needsLogin bool
	// Whether cloudflared login is currently running (TUI is suspended)
	loggingIn bool

	focus  int // active field index (depends on source mode)
	width  int
	height int
}

// NewTunnelPage creates page 1.
func NewTunnelPage() *TunnelPage {
	_, cliErr := common.DetectCloudflared()
	hasCLI := cliErr == nil

	defaultSource := tunnelSourceManual
	if hasCLI {
		defaultSource = tunnelSourceSelect
	}

	sourceOpts := []common.SelectorOption{
		{Value: "select", Label: "Select existing tunnel", Description: "Pick from cloudflared tunnel list"},
		{Value: "create", Label: "Create new tunnel", Description: "Run cloudflared tunnel create"},
		{Value: "manual", Label: "Enter manually", Description: "Paste tunnel UUID and credentials path"},
	}
	src := common.NewSelector("How would you like to configure the tunnel?", sourceOpts)
	src.Cursor = int(defaultSource)

	tid := common.NewTextInput("Tunnel UUID", "550e8400-e29b-41d4-a716-446655440000", "(from cloudflared tunnel create)")
	tid.Validate = config.ValidateUUID

	creds := common.NewTextInput("Credentials File", "/etc/cloudflared/credentials.json", "")
	creds.Validate = config.ValidateNonEmpty

	proto := common.NewSelector("Protocol", []common.SelectorOption{
		{Value: "quic", Label: "QUIC", Description: "UDP 7844 — preferred, lower latency"},
		{Value: "http2", Label: "HTTP/2", Description: "TCP 443 — fallback when UDP is blocked"},
	})

	ing := common.NewIngressEditor("Ingress Rules")

	tunnelName := common.NewTextInput("Tunnel Name", "my-fips-tunnel", "")
	tunnelName.Validate = config.ValidateNonEmpty

	tunnelPicker := common.NewSelector("Select Tunnel", nil)

	return &TunnelPage{
		source:       src,
		tunnelID:     tid,
		credentials:  creds,
		protocol:     proto,
		ingress:      ing,
		tunnelName:   tunnelName,
		tunnelPicker: tunnelPicker,
		hasCLI:       hasCLI,
	}
}

func (p *TunnelPage) Title() string { return "Tunnel Configuration" }

func (p *TunnelPage) Init() tea.Cmd { return nil }

func (p *TunnelPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return nil
}

func (p *TunnelPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *TunnelPage) currentSource() tunnelSource {
	switch p.source.Selected() {
	case "select":
		return tunnelSourceSelect
	case "create":
		return tunnelSourceCreate
	default:
		return tunnelSourceManual
	}
}

// fieldCount returns total focusable fields for the current source mode.
// field 0 is always the source selector.
func (p *TunnelPage) fieldCount() int {
	switch p.currentSource() {
	case tunnelSourceSelect:
		// source, tunnelPicker, protocol, ingress
		return 4
	case tunnelSourceCreate:
		// source, tunnelName, protocol, ingress
		return 4
	default:
		// source, tunnelID, credentials, protocol, ingress
		return 5
	}
}

func (p *TunnelPage) updateFocus() {
	p.source.Blur()
	p.tunnelID.Blur()
	p.credentials.Blur()
	p.protocol.Blur()
	p.ingress.Blur()
	p.tunnelPicker.Blur()
	p.tunnelName.Blur()

	switch p.focus {
	case 0:
		p.source.Focus()
	default:
		p.focusContentField(p.focus)
	}
}

// focusContentField focuses field idx (1-based) within the current source mode.
func (p *TunnelPage) focusContentField(idx int) {
	switch p.currentSource() {
	case tunnelSourceSelect:
		switch idx {
		case 1:
			p.tunnelPicker.Focus()
		case 2:
			p.protocol.Focus()
		case 3:
			p.ingress.Focus()
		}
	case tunnelSourceCreate:
		switch idx {
		case 1:
			p.tunnelName.Input.Focus()
		case 2:
			p.protocol.Focus()
		case 3:
			p.ingress.Focus()
		}
	default: // manual
		switch idx {
		case 1:
			p.tunnelID.Input.Focus()
		case 2:
			p.credentials.Input.Focus()
		case 3:
			p.protocol.Focus()
		case 4:
			p.ingress.Focus()
		}
	}
}

func (p *TunnelPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	// Handle async messages
	switch msg := msg.(type) {
	case loginCompleteMsg:
		p.loggingIn = false
		if msg.err != nil {
			// Login failed or was cancelled — show error, let user retry
			if p.currentSource() == tunnelSourceSelect {
				p.tunnelErr = fmt.Sprintf("login failed: %v", msg.err)
			} else {
				p.createErr = fmt.Sprintf("login failed: %v", msg.err)
			}
			return p, nil
		}
		// Login succeeded — clear error state and auto-retry the operation
		p.needsLogin = false
		if p.currentSource() == tunnelSourceSelect {
			p.tunnelErr = ""
			p.tunnelsLoaded = false
			return p, p.fetchTunnelList()
		}
		// Create mode: auto-retry tunnel creation with the name already entered
		p.createErr = ""
		return p, p.triggerCreate()

	case tunnelsListedMsg:
		p.tunnelsLoaded = true
		if msg.err != nil {
			p.needsLogin = errors.Is(msg.err, common.ErrNotLoggedIn)
			p.tunnelErr = msg.err.Error()
			return p, nil
		}
		p.tunnelList = msg.tunnels
		opts := make([]common.SelectorOption, len(msg.tunnels))
		for i, t := range msg.tunnels {
			opts[i] = common.SelectorOption{
				Value:       t.ID,
				Label:       t.Name,
				Description: t.ID,
			}
		}
		p.tunnelPicker = common.NewSelector("Select Tunnel", opts)
		if p.focus == 1 {
			p.tunnelPicker.Focus()
		}
		return p, nil

	case tunnelCreatedMsg:
		p.creating = false
		if msg.err != nil {
			p.needsLogin = errors.Is(msg.err, common.ErrNotLoggedIn)
			p.createErr = msg.err.Error()
			return p, nil
		}
		p.tunnelID.SetValue(msg.uuid)
		if msg.credsPath != "" {
			p.credentials.SetValue(msg.credsPath)
		}
		// Advance past tunnelName to protocol
		p.focus = 2
		p.updateFocus()
		return p, nil
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Handle source selector: on tab out, maybe trigger async load
			if p.focus == 0 {
				cmd := p.onSourceSelected()
				p.focus = 1
				p.updateFocus()
				if cmd != nil {
					return p, cmd
				}
				return p, fieldNav
			}

			// If login is needed, Enter triggers interactive cloudflared login
			if p.needsLogin && p.focus == 1 {
				return p, p.triggerLogin()
			}

			// For create mode on the name field: trigger creation,
			// but skip if tunnel was already created successfully.
			if p.currentSource() == tunnelSourceCreate && p.focus == 1 {
				if p.tunnelID.Value() != "" {
					p.createErr = ""
					p.focus++
					p.updateFocus()
					return p, fieldNav
				}
				return p, p.triggerCreate()
			}

			// Ingress in add mode: don't advance
			lastField := p.fieldCount() - 1
			if p.focus == lastField && p.ingress.Adding {
				return p, fieldNav
			}
			// Selectors: enter selects within the component, don't advance;
			// tab advances to the next field.
			if msg.String() == "enter" && p.isCurrentFieldSelector() {
				return p, fieldNav
			}

			if p.focus < lastField {
				p.focus++
				p.updateFocus()
				return p, fieldNav
			}
			// At last field: wizard will handle page advance
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

	// Delegate to focused component
	return p, p.updateFocusedField(msg)
}

// triggerLogin suspends the TUI and runs "cloudflared login" interactively.
func (p *TunnelPage) triggerLogin() tea.Cmd {
	p.loggingIn = true
	cmd := common.LoginCmd()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return loginCompleteMsg{err: err}
	})
}

func (p *TunnelPage) isCurrentFieldSelector() bool {
	if p.focus == 0 {
		return true
	}
	src := p.currentSource()
	switch {
	case src == tunnelSourceSelect && p.focus == 1:
		return true // tunnelPicker
	case src == tunnelSourceSelect && p.focus == 2:
		return true // protocol
	case src == tunnelSourceCreate && p.focus == 2:
		return true // protocol
	case src == tunnelSourceManual && p.focus == 3:
		return true // protocol
	}
	return false
}

func (p *TunnelPage) updateFocusedField(msg tea.Msg) tea.Cmd {
	if p.focus == 0 {
		p.source.Update(msg)
		return nil
	}
	switch p.currentSource() {
	case tunnelSourceSelect:
		switch p.focus {
		case 1:
			p.tunnelPicker.Update(msg)
		case 2:
			p.protocol.Update(msg)
		case 3:
			return p.ingress.Update(msg)
		}
	case tunnelSourceCreate:
		switch p.focus {
		case 1:
			return p.tunnelName.Update(msg)
		case 2:
			p.protocol.Update(msg)
		case 3:
			return p.ingress.Update(msg)
		}
	default: // manual
		switch p.focus {
		case 1:
			return p.tunnelID.Update(msg)
		case 2:
			return p.credentials.Update(msg)
		case 3:
			p.protocol.Update(msg)
		case 4:
			return p.ingress.Update(msg)
		}
	}
	return nil
}

// onSourceSelected fires when the user tabs past the source selector.
func (p *TunnelPage) onSourceSelected() tea.Cmd {
	if p.currentSource() == tunnelSourceSelect && !p.tunnelsLoaded && p.hasCLI {
		return p.fetchTunnelList()
	}
	return nil
}

func (p *TunnelPage) fetchTunnelList() tea.Cmd {
	return func() tea.Msg {
		tunnels, err := common.ListTunnelsCLI()
		return tunnelsListedMsg{tunnels: tunnels, err: err}
	}
}

func (p *TunnelPage) triggerCreate() tea.Cmd {
	name := strings.TrimSpace(p.tunnelName.Value())
	if name == "" {
		p.createErr = "tunnel name is required"
		return nil
	}
	p.creating = true
	p.createErr = ""
	return func() tea.Msg {
		uuid, creds, err := common.CreateTunnel(name)
		return tunnelCreatedMsg{uuid: uuid, credsPath: creds, err: err}
	}
}

func (p *TunnelPage) Validate() bool {
	switch p.currentSource() {
	case tunnelSourceSelect:
		if len(p.tunnelPicker.Options) == 0 {
			p.tunnelErr = "no tunnels available — create one or enter manually"
			return false
		}
		return true
	case tunnelSourceCreate:
		if p.tunnelID.Value() == "" {
			p.createErr = "tunnel has not been created yet"
			return false
		}
		return true
	default: // manual
		valid := true
		if !p.tunnelID.RunValidation() {
			valid = false
		}
		if !p.credentials.RunValidation() {
			valid = false
		}
		return valid
	}
}

func (p *TunnelPage) Apply(cfg *config.Config) {
	switch p.currentSource() {
	case tunnelSourceSelect:
		selected := p.tunnelPicker.Selected()
		cfg.Tunnel = selected
		creds := common.FindCredentialsFile(selected)
		if creds != "" {
			cfg.CredentialsFile = creds
		}
	case tunnelSourceCreate:
		cfg.Tunnel = strings.TrimSpace(p.tunnelID.Value())
		cfg.CredentialsFile = strings.TrimSpace(p.credentials.Value())
	default:
		cfg.Tunnel = strings.TrimSpace(p.tunnelID.Value())
		cfg.CredentialsFile = strings.TrimSpace(p.credentials.Value())
	}

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

	cfg.Dashboard.TunnelID = cfg.Tunnel
}

func (p *TunnelPage) View() string {
	var b strings.Builder

	// Source selector
	b.WriteString(p.source.View())

	if !p.hasCLI && p.currentSource() != tunnelSourceManual {
		b.WriteString(common.WarningStyle.Render("  cloudflared not found on PATH — install it or enter manually"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Source-specific content
	switch p.currentSource() {
	case tunnelSourceSelect:
		b.WriteString(p.viewSelect())
	case tunnelSourceCreate:
		b.WriteString(p.viewCreate())
	default:
		b.WriteString(p.viewManual())
	}

	b.WriteString("\n")
	b.WriteString(p.protocol.View())
	b.WriteString("\n")
	b.WriteString(p.ingress.View())
	return b.String()
}

func (p *TunnelPage) viewSelect() string {
	var b strings.Builder
	if p.loggingIn {
		b.WriteString(common.HintStyle.Render("  Logging in via browser..."))
		b.WriteString("\n")
	} else if !p.tunnelsLoaded {
		b.WriteString(common.HintStyle.Render("  Loading tunnels..."))
		b.WriteString("\n")
	} else if p.tunnelErr != "" {
		b.WriteString(common.ErrorStyle.Render("  ! "+p.tunnelErr))
		b.WriteString("\n")
		if p.needsLogin {
			b.WriteString(common.HintStyle.Render("  Press Enter to log in via browser"))
			b.WriteString("\n")
		}
	} else if len(p.tunnelPicker.Options) == 0 {
		b.WriteString(common.MutedStyle.Render("  No tunnels found. Create one or enter manually."))
		b.WriteString("\n")
	} else {
		b.WriteString(p.tunnelPicker.View())
	}
	b.WriteString("\n")
	return b.String()
}

func (p *TunnelPage) viewCreate() string {
	var b strings.Builder
	b.WriteString(p.tunnelName.View())
	b.WriteString("\n")
	if p.loggingIn {
		b.WriteString(common.HintStyle.Render("  Logging in via browser..."))
		b.WriteString("\n")
	} else if p.creating {
		b.WriteString(common.HintStyle.Render("  Creating tunnel..."))
		b.WriteString("\n")
	}
	if p.createErr != "" {
		b.WriteString(common.ErrorStyle.Render("  ! "+p.createErr))
		b.WriteString("\n")
		if p.needsLogin {
			b.WriteString(common.HintStyle.Render("  Press Enter to log in via browser"))
			b.WriteString("\n")
		}
	}
	if p.tunnelID.Value() != "" {
		b.WriteString(common.SuccessStyle.Render(fmt.Sprintf("  Created: %s", p.tunnelID.Value())))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (p *TunnelPage) viewManual() string {
	var b strings.Builder
	b.WriteString(p.tunnelID.View())
	b.WriteString("\n\n")
	b.WriteString(p.credentials.View())
	b.WriteString("\n\n")
	return b.String()
}

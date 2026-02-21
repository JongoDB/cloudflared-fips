package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// TunnelPage is page 1: tunnel UUID, credentials, protocol, ingress rules.
type TunnelPage struct {
	tunnelID    common.TextInput
	credentials common.TextInput
	protocol    common.Selector
	ingress     common.IngressEditor
	focus       int // 0=tunnel, 1=creds, 2=protocol, 3=ingress
	width       int
	height      int
}

const tunnelFieldCount = 4

// NewTunnelPage creates page 1.
func NewTunnelPage() *TunnelPage {
	tid := common.NewTextInput("Tunnel UUID", "550e8400-e29b-41d4-a716-446655440000", "(from cloudflared tunnel create)")
	tid.Validate = config.ValidateUUID

	creds := common.NewTextInput("Credentials File", "/etc/cloudflared/credentials.json", "")
	// Don't validate file-exists in the TUI since it may be on a remote host
	creds.Validate = config.ValidateNonEmpty

	proto := common.NewSelector("Protocol", []common.SelectorOption{
		{Value: "quic", Label: "QUIC", Description: "UDP 7844 — preferred, lower latency"},
		{Value: "http2", Label: "HTTP/2", Description: "TCP 443 — fallback when UDP is blocked"},
	})

	ing := common.NewIngressEditor("Ingress Rules")

	return &TunnelPage{
		tunnelID:    tid,
		credentials: creds,
		protocol:    proto,
		ingress:     ing,
	}
}

func (p *TunnelPage) Title() string { return "Tunnel Configuration" }

func (p *TunnelPage) Init() tea.Cmd { return nil }

func (p *TunnelPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.tunnelID.Focus()
}

func (p *TunnelPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *TunnelPage) updateFocus() {
	p.tunnelID.Blur()
	p.credentials.Blur()
	p.protocol.Blur()
	p.ingress.Blur()

	switch p.focus {
	case 0:
		p.tunnelID.Input.Focus()
	case 1:
		p.credentials.Input.Focus()
	case 2:
		p.protocol.Focus()
	case 3:
		p.ingress.Focus()
	}
}

func (p *TunnelPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		// Handle focus navigation within the page
		switch msg.String() {
		case "tab", "enter":
			// For ingress in add mode, don't advance page focus
			if p.focus == 3 && p.ingress.Adding {
				break
			}
			// For selector, enter selects — don't advance
			if p.focus == 2 {
				// Allow normal selector navigation
				break
			}
			if p.focus < tunnelFieldCount-1 {
				p.focus++
				p.updateFocus()
				return p, nil
			}
			// At last field, tab handled by wizard (advance page)
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

	// Delegate to focused component
	var cmd tea.Cmd
	switch p.focus {
	case 0:
		cmd = p.tunnelID.Update(msg)
	case 1:
		cmd = p.credentials.Update(msg)
	case 2:
		p.protocol.Update(msg)
	case 3:
		cmd = p.ingress.Update(msg)
	}
	return p, cmd
}

func (p *TunnelPage) Validate() bool {
	valid := true
	if !p.tunnelID.RunValidation() {
		valid = false
	}
	if !p.credentials.RunValidation() {
		valid = false
	}
	return valid
}

func (p *TunnelPage) Apply(cfg *config.Config) {
	cfg.Tunnel = strings.TrimSpace(p.tunnelID.Value())
	cfg.CredentialsFile = strings.TrimSpace(p.credentials.Value())
	cfg.Protocol = p.protocol.Selected()

	// Build ingress list
	var rules []config.IngressRule
	for _, entry := range p.ingress.Entries {
		rules = append(rules, config.IngressRule{
			Hostname: entry.Hostname,
			Service:  entry.Service,
		})
	}
	// Always append catch-all
	rules = append(rules, config.IngressRule{Service: "http_status:404"})
	cfg.Ingress = rules

	// Set tunnel ID in dashboard wiring too
	cfg.Dashboard.TunnelID = cfg.Tunnel
}

func (p *TunnelPage) View() string {
	var b strings.Builder
	b.WriteString(p.tunnelID.View())
	b.WriteString("\n\n")
	b.WriteString(p.credentials.View())
	b.WriteString("\n\n")
	b.WriteString(p.protocol.View())
	b.WriteString("\n")
	b.WriteString(p.ingress.View())
	return b.String()
}

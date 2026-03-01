package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ServerConfigPage collects server-specific settings: tunnel token or UUID+creds,
// protocol, ingress, node identity, and optional fleet enrollment.
type ServerConfigPage struct {
	tunnelToken   common.TextInput
	protocol      common.Selector
	ingress       common.IngressEditor
	nodeName      common.TextInput
	region        common.TextInput
	controllerURL common.TextInput
	enrollToken   common.TextInput

	focus  int
	width  int
	height int
}

const serverFieldCount = 7

// NewServerConfigPage creates the server config page.
func NewServerConfigPage() *ServerConfigPage {
	token := common.NewTextInput("Tunnel Token", "eyJ...", "(from cloudflared tunnel token <ID>)")
	token.Validate = config.ValidateNonEmpty

	proto := common.NewSelector("Protocol", []common.SelectorOption{
		{Value: "quic", Label: "QUIC", Description: "UDP 7844 — preferred, lower latency"},
		{Value: "http2", Label: "HTTP/2", Description: "TCP 443 — fallback when UDP is blocked"},
	})

	ing := common.NewIngressEditor("Ingress Rules")

	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	nodeRegion := common.NewTextInput("Node Region", "us-east", "(optional)")

	ctrlURL := common.NewTextInput("Fleet Controller URL", "https://controller.example.com:8080", "(optional — enroll in fleet)")
	enrollToken := common.NewTextInput("Enrollment Token", "tok-...", "(from controller)")

	return &ServerConfigPage{
		tunnelToken:   token,
		protocol:      proto,
		ingress:       ing,
		nodeName:      nodeName,
		region:        nodeRegion,
		controllerURL: ctrlURL,
		enrollToken:   enrollToken,
	}
}

func (p *ServerConfigPage) Title() string { return "Server Config" }
func (p *ServerConfigPage) Init() tea.Cmd { return nil }

func (p *ServerConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.tunnelToken.Focus()
}

func (p *ServerConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ServerConfigPage) updateFocus() {
	p.tunnelToken.Blur()
	p.protocol.Blur()
	p.ingress.Blur()
	p.nodeName.Blur()
	p.region.Blur()
	p.controllerURL.Blur()
	p.enrollToken.Blur()

	switch p.focus {
	case 0:
		p.tunnelToken.Input.Focus()
	case 1:
		p.protocol.Focus()
	case 2:
		p.ingress.Focus()
	case 3:
		p.nodeName.Input.Focus()
	case 4:
		p.region.Input.Focus()
	case 5:
		p.controllerURL.Input.Focus()
	case 6:
		p.enrollToken.Input.Focus()
	}
}

func (p *ServerConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Ingress in add mode: don't advance
			if p.focus == 2 && p.ingress.Adding {
				return p, fieldNav
			}
			// Protocol selector: enter selects within
			if msg.String() == "enter" && p.focus == 1 {
				return p, fieldNav
			}
			if p.focus < serverFieldCount-1 {
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
		cmd = p.tunnelToken.Update(msg)
	case 1:
		p.protocol.Update(msg)
	case 2:
		cmd = p.ingress.Update(msg)
	case 3:
		cmd = p.nodeName.Update(msg)
	case 4:
		cmd = p.region.Update(msg)
	case 5:
		cmd = p.controllerURL.Update(msg)
	case 6:
		cmd = p.enrollToken.Update(msg)
	}
	return p, cmd
}

func (p *ServerConfigPage) Validate() bool {
	return p.tunnelToken.RunValidation()
}

func (p *ServerConfigPage) Apply(cfg *config.Config) {
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

	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	cfg.ControllerURL = strings.TrimSpace(p.controllerURL.Value())
	cfg.EnrollmentToken = strings.TrimSpace(p.enrollToken.Value())
}

func (p *ServerConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Tunnel & Fleet Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.tunnelToken.View())
	b.WriteString("\n\n")
	b.WriteString(p.protocol.View())
	b.WriteString("\n")
	b.WriteString(p.ingress.View())
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Fleet Enrollment (optional)"))
	b.WriteString("\n\n")
	b.WriteString(p.controllerURL.View())
	b.WriteString("\n\n")
	b.WriteString(p.enrollToken.View())
	return b.String()
}

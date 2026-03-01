package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ProxyConfigPage collects client-side FIPS forward proxy settings:
// TLS termination, own Cloudflare tunnel, and fleet enrollment.
type ProxyConfigPage struct {
	nodeName      common.TextInput
	region        common.TextInput
	listenAddr    common.TextInput
	certFile      common.TextInput
	keyFile       common.TextInput
	tunnelToken   common.TextInput
	protocol      common.Selector
	controllerURL common.TextInput
	enrollToken   common.TextInput

	focus  int
	width  int
	height int
}

const proxyFieldCount = 9

// NewProxyConfigPage creates the proxy config page.
func NewProxyConfigPage() *ProxyConfigPage {
	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	region := common.NewTextInput("Node Region", "us-east", "(optional)")

	listen := common.NewTextInput("Listen Address", ":443", "(client-facing)")
	listen.Input.SetValue(":443")
	listen.Validate = config.ValidateNonEmpty

	cert := common.NewTextInput("TLS Certificate File", "/etc/pki/tls/certs/proxy.pem", "")
	cert.Validate = config.ValidateNonEmpty

	key := common.NewTextInput("TLS Private Key File", "/etc/pki/tls/private/proxy-key.pem", "")
	key.Validate = config.ValidateNonEmpty

	tunnelToken := common.NewTextInput("Tunnel Token", "eyJ...", "(proxy's own tunnel for FIPS egress)")
	tunnelToken.Validate = config.ValidateNonEmpty

	proto := common.NewSelector("Protocol", []common.SelectorOption{
		{Value: "quic", Label: "QUIC", Description: "UDP 7844 — preferred, lower latency"},
		{Value: "http2", Label: "HTTP/2", Description: "TCP 443 — fallback when UDP is blocked"},
	})

	ctrlURL := common.NewTextInput("Controller URL", "https://controller.example.com:8080", "(REQUIRED)")
	ctrlURL.Validate = config.ValidateNonEmpty

	enrollToken := common.NewTextInput("Enrollment Token", "tok-...", "(REQUIRED — from controller)")
	enrollToken.Validate = config.ValidateNonEmpty

	return &ProxyConfigPage{
		nodeName:      nodeName,
		region:        region,
		listenAddr:    listen,
		certFile:      cert,
		keyFile:       key,
		tunnelToken:   tunnelToken,
		protocol:      proto,
		controllerURL: ctrlURL,
		enrollToken:   enrollToken,
	}
}

func (p *ProxyConfigPage) Title() string { return "FIPS Forward Proxy" }
func (p *ProxyConfigPage) Init() tea.Cmd { return nil }

func (p *ProxyConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.nodeName.Focus()
}

func (p *ProxyConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ProxyConfigPage) updateFocus() {
	p.nodeName.Blur()
	p.region.Blur()
	p.listenAddr.Blur()
	p.certFile.Blur()
	p.keyFile.Blur()
	p.tunnelToken.Blur()
	p.protocol.Blur()
	p.controllerURL.Blur()
	p.enrollToken.Blur()

	switch p.focus {
	case 0:
		p.nodeName.Focus()
	case 1:
		p.region.Focus()
	case 2:
		p.listenAddr.Focus()
	case 3:
		p.certFile.Focus()
	case 4:
		p.keyFile.Focus()
	case 5:
		p.tunnelToken.Focus()
	case 6:
		p.protocol.Focus()
	case 7:
		p.controllerURL.Focus()
	case 8:
		p.enrollToken.Focus()
	}
}

func (p *ProxyConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			// Protocol selector: enter selects within
			if msg.String() == "enter" && p.focus == 6 {
				return p, fieldNav
			}
			if p.focus < proxyFieldCount-1 {
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
		cmd = p.nodeName.Update(msg)
	case 1:
		cmd = p.region.Update(msg)
	case 2:
		cmd = p.listenAddr.Update(msg)
	case 3:
		cmd = p.certFile.Update(msg)
	case 4:
		cmd = p.keyFile.Update(msg)
	case 5:
		cmd = p.tunnelToken.Update(msg)
	case 6:
		p.protocol.Update(msg)
	case 7:
		cmd = p.controllerURL.Update(msg)
	case 8:
		cmd = p.enrollToken.Update(msg)
	}
	return p, cmd
}

func (p *ProxyConfigPage) ScrollOffset() int {
	offsets := []int{2, 7, 13, 18, 23, 28, 33, 39, 44}
	if p.focus < len(offsets) {
		return offsets[p.focus]
	}
	return 0
}

func (p *ProxyConfigPage) Validate() bool {
	valid := true
	if !p.listenAddr.RunValidation() {
		valid = false
	}
	if !p.certFile.RunValidation() {
		valid = false
	}
	if !p.keyFile.RunValidation() {
		valid = false
	}
	if !p.tunnelToken.RunValidation() {
		valid = false
	}
	if !p.controllerURL.RunValidation() {
		valid = false
	}
	if !p.enrollToken.RunValidation() {
		valid = false
	}
	return valid
}

func (p *ProxyConfigPage) Apply(cfg *config.Config) {
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	cfg.ProxyListenAddr = strings.TrimSpace(p.listenAddr.Value())
	cfg.ProxyCertFile = strings.TrimSpace(p.certFile.Value())
	cfg.ProxyKeyFile = strings.TrimSpace(p.keyFile.Value())
	cfg.TunnelToken = strings.TrimSpace(p.tunnelToken.Value())
	cfg.Protocol = p.protocol.Selected()
	cfg.ControllerURL = strings.TrimSpace(p.controllerURL.Value())
	cfg.EnrollmentToken = strings.TrimSpace(p.enrollToken.Value())
}

func (p *ProxyConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Client-Side FIPS Proxy Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("TLS Termination (client-facing)"))
	b.WriteString("\n\n")
	b.WriteString(p.listenAddr.View())
	b.WriteString("\n\n")
	b.WriteString(p.certFile.View())
	b.WriteString("\n\n")
	b.WriteString(p.keyFile.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Proxy Tunnel (FIPS egress)"))
	b.WriteString("\n\n")
	b.WriteString(p.tunnelToken.View())
	b.WriteString("\n\n")
	b.WriteString(p.protocol.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Fleet Enrollment (required)"))
	b.WriteString("\n\n")
	b.WriteString(p.controllerURL.View())
	b.WriteString("\n\n")
	b.WriteString(p.enrollToken.View())
	return b.String()
}

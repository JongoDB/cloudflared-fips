package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ProxyConfigPage collects FIPS edge proxy settings.
type ProxyConfigPage struct {
	listenAddr    common.TextInput
	certFile      common.TextInput
	keyFile       common.TextInput
	upstream      common.TextInput
	nodeName      common.TextInput
	region        common.TextInput
	controllerURL common.TextInput
	enrollToken   common.TextInput

	focus  int
	width  int
	height int
}

const proxyFieldCount = 8

// NewProxyConfigPage creates the proxy config page.
func NewProxyConfigPage() *ProxyConfigPage {
	listen := common.NewTextInput("Listen Address", ":443", "")
	listen.Input.SetValue(":443")
	listen.Validate = config.ValidateNonEmpty

	cert := common.NewTextInput("TLS Certificate File", "/etc/pki/tls/certs/proxy.pem", "")
	cert.Validate = config.ValidateNonEmpty

	key := common.NewTextInput("TLS Private Key File", "/etc/pki/tls/private/proxy-key.pem", "")
	key.Validate = config.ValidateNonEmpty

	upstream := common.NewTextInput("Upstream URL", "https://your-app.example.com", "")
	upstream.Validate = config.ValidateNonEmpty

	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	region := common.NewTextInput("Node Region", "us-east", "(optional)")

	ctrlURL := common.NewTextInput("Fleet Controller URL", "https://controller.example.com:8080", "(required for fleet enrollment)")
	enrollToken := common.NewTextInput("Enrollment Token", "tok-...", "(from controller)")

	return &ProxyConfigPage{
		listenAddr:    listen,
		certFile:      cert,
		keyFile:       key,
		upstream:      upstream,
		nodeName:      nodeName,
		region:        region,
		controllerURL: ctrlURL,
		enrollToken:   enrollToken,
	}
}

func (p *ProxyConfigPage) Title() string { return "Proxy Config" }
func (p *ProxyConfigPage) Init() tea.Cmd { return nil }

func (p *ProxyConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.listenAddr.Focus()
}

func (p *ProxyConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ProxyConfigPage) updateFocus() {
	p.listenAddr.Blur()
	p.certFile.Blur()
	p.keyFile.Blur()
	p.upstream.Blur()
	p.nodeName.Blur()
	p.region.Blur()
	p.controllerURL.Blur()
	p.enrollToken.Blur()

	switch p.focus {
	case 0:
		p.listenAddr.Input.Focus()
	case 1:
		p.certFile.Input.Focus()
	case 2:
		p.keyFile.Input.Focus()
	case 3:
		p.upstream.Input.Focus()
	case 4:
		p.nodeName.Input.Focus()
	case 5:
		p.region.Input.Focus()
	case 6:
		p.controllerURL.Input.Focus()
	case 7:
		p.enrollToken.Input.Focus()
	}
}

func (p *ProxyConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
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
		cmd = p.listenAddr.Update(msg)
	case 1:
		cmd = p.certFile.Update(msg)
	case 2:
		cmd = p.keyFile.Update(msg)
	case 3:
		cmd = p.upstream.Update(msg)
	case 4:
		cmd = p.nodeName.Update(msg)
	case 5:
		cmd = p.region.Update(msg)
	case 6:
		cmd = p.controllerURL.Update(msg)
	case 7:
		cmd = p.enrollToken.Update(msg)
	}
	return p, cmd
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
	if !p.upstream.RunValidation() {
		valid = false
	}
	return valid
}

func (p *ProxyConfigPage) Apply(cfg *config.Config) {
	cfg.ProxyListenAddr = strings.TrimSpace(p.listenAddr.Value())
	cfg.ProxyCertFile = strings.TrimSpace(p.certFile.Value())
	cfg.ProxyKeyFile = strings.TrimSpace(p.keyFile.Value())
	cfg.ProxyUpstream = strings.TrimSpace(p.upstream.Value())
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	cfg.ControllerURL = strings.TrimSpace(p.controllerURL.Value())
	cfg.EnrollmentToken = strings.TrimSpace(p.enrollToken.Value())
}

func (p *ProxyConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("FIPS Edge Proxy Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.listenAddr.View())
	b.WriteString("\n\n")
	b.WriteString(p.certFile.View())
	b.WriteString("\n\n")
	b.WriteString(p.keyFile.View())
	b.WriteString("\n\n")
	b.WriteString(p.upstream.View())
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Fleet Enrollment"))
	b.WriteString("\n\n")
	b.WriteString(p.controllerURL.View())
	b.WriteString("\n\n")
	b.WriteString(p.enrollToken.View())
	return b.String()
}

package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ServerConfigPage collects origin server settings: service endpoint,
// node identity, and mandatory fleet enrollment.
type ServerConfigPage struct {
	nodeName      common.TextInput
	region        common.TextInput
	serviceName   common.TextInput
	serviceHost   common.TextInput
	servicePort   common.TextInput
	serviceTLS    common.Toggle
	controllerURL common.TextInput
	enrollToken   common.TextInput

	focus  int
	width  int
	height int
}

const serverFieldCount = 8

// NewServerConfigPage creates the server config page.
func NewServerConfigPage() *ServerConfigPage {
	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	nodeRegion := common.NewTextInput("Node Region", "us-east", "(optional)")

	svcName := common.NewTextInput("Service Name", "internal-api", "(display name for this origin)")
	svcName.Validate = config.ValidateNonEmpty

	svcHost := common.NewTextInput("Service Host", "0.0.0.0", "(listen address)")
	svcHost.Input.SetValue("0.0.0.0")
	svcHost.Validate = config.ValidateNonEmpty

	svcPort := common.NewTextInput("Service Port", "8443", "")
	svcPort.Input.SetValue("8443")
	svcPort.Validate = config.ValidatePort

	ctrlURL := common.NewTextInput("Controller URL", "https://controller.example.com:8080", "(REQUIRED)")
	ctrlURL.Validate = config.ValidateNonEmpty
	ctrlURL.HelpText = "The controller's dashboard URL (includes fleet API endpoints).\nExample: https://controller.internal:8080\nMust be reachable from this node over the network."

	enrollToken := common.NewTextInput("Enrollment Token", "tok-...", "(REQUIRED — from controller)")
	enrollToken.Validate = config.ValidateNonEmpty
	enrollToken.HelpText = "One-time token from the controller admin.\nGenerate on controller:\n  curl -H 'Authorization: Bearer <admin-key>' \\\n    -X POST https://<controller>:8080/api/v1/fleet/tokens"

	return &ServerConfigPage{
		nodeName:      nodeName,
		region:        nodeRegion,
		serviceName:   svcName,
		serviceHost:   svcHost,
		servicePort:   svcPort,
		serviceTLS:    common.NewToggle("Service uses TLS", "Origin service listens on HTTPS", true),
		controllerURL: ctrlURL,
		enrollToken:   enrollToken,
	}
}

func (p *ServerConfigPage) Title() string { return "Origin Service" }
func (p *ServerConfigPage) Init() tea.Cmd { return nil }

func (p *ServerConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.nodeName.Focus()
}

func (p *ServerConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ServerConfigPage) updateFocus() {
	p.nodeName.Blur()
	p.region.Blur()
	p.serviceName.Blur()
	p.serviceHost.Blur()
	p.servicePort.Blur()
	p.serviceTLS.Blur()
	p.controllerURL.Blur()
	p.enrollToken.Blur()

	switch p.focus {
	case 0:
		p.nodeName.Focus()
	case 1:
		p.region.Focus()
	case 2:
		p.serviceName.Focus()
	case 3:
		p.serviceHost.Focus()
	case 4:
		p.servicePort.Focus()
	case 5:
		p.serviceTLS.Focus()
	case 6:
		p.controllerURL.Focus()
	case 7:
		p.enrollToken.Focus()
	}
}

func (p *ServerConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
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
		cmd = p.nodeName.Update(msg)
	case 1:
		cmd = p.region.Update(msg)
	case 2:
		cmd = p.serviceName.Update(msg)
	case 3:
		cmd = p.serviceHost.Update(msg)
	case 4:
		cmd = p.servicePort.Update(msg)
	case 5:
		p.serviceTLS.Update(msg)
	case 6:
		cmd = p.controllerURL.Update(msg)
	case 7:
		cmd = p.enrollToken.Update(msg)
	}
	return p, cmd
}

func (p *ServerConfigPage) ScrollOffset() int {
	offsets := []int{2, 7, 13, 18, 23, 28, 32, 37}
	if p.focus < len(offsets) {
		return offsets[p.focus]
	}
	return 0
}

func (p *ServerConfigPage) Validate() bool {
	valid := true
	if !p.serviceName.RunValidation() {
		valid = false
	}
	if !p.serviceHost.RunValidation() {
		valid = false
	}
	if !p.servicePort.RunValidation() {
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

func (p *ServerConfigPage) Apply(cfg *config.Config) {
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	cfg.ServiceName = strings.TrimSpace(p.serviceName.Value())
	cfg.ServiceHost = strings.TrimSpace(p.serviceHost.Value())
	cfg.ServicePort = parseInt(strings.TrimSpace(p.servicePort.Value()))
	cfg.ServiceTLS = p.serviceTLS.Enabled
	cfg.ControllerURL = strings.TrimSpace(p.controllerURL.Value())
	cfg.EnrollmentToken = strings.TrimSpace(p.enrollToken.Value())
}

func (p *ServerConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Origin Service Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Service Endpoint"))
	b.WriteString("\n\n")
	b.WriteString(p.serviceName.View())
	b.WriteString("\n\n")
	b.WriteString(p.serviceHost.View())
	b.WriteString("\n\n")
	b.WriteString(p.servicePort.View())
	b.WriteString("\n\n")
	b.WriteString(p.serviceTLS.View())
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Fleet Enrollment (required)"))
	b.WriteString("\n\n")
	b.WriteString(p.controllerURL.View())
	b.WriteString("\n\n")
	b.WriteString(p.enrollToken.View())
	return b.String()
}

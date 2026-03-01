package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// AgentConfigPage collects client agent settings.
type AgentConfigPage struct {
	controllerURL common.TextInput
	enrollToken   common.TextInput
	nodeName      common.TextInput
	region        common.TextInput

	focus  int
	width  int
	height int
}

const agentFieldCount = 4

// NewAgentConfigPage creates the client agent config page.
func NewAgentConfigPage() *AgentConfigPage {
	ctrlURL := common.NewTextInput("Controller URL", "https://controller.example.com:8080", "(required)")
	ctrlURL.Validate = config.ValidateNonEmpty

	enrollToken := common.NewTextInput("Enrollment Token", "tok-...", "(from controller)")
	enrollToken.Validate = config.ValidateNonEmpty

	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	region := common.NewTextInput("Node Region", "us-east", "(optional)")

	return &AgentConfigPage{
		controllerURL: ctrlURL,
		enrollToken:   enrollToken,
		nodeName:      nodeName,
		region:        region,
	}
}

func (p *AgentConfigPage) Title() string { return "Agent Config" }
func (p *AgentConfigPage) Init() tea.Cmd { return nil }

func (p *AgentConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.controllerURL.Focus()
}

func (p *AgentConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *AgentConfigPage) updateFocus() {
	p.controllerURL.Blur()
	p.enrollToken.Blur()
	p.nodeName.Blur()
	p.region.Blur()

	switch p.focus {
	case 0:
		p.controllerURL.Focus()
	case 1:
		p.enrollToken.Focus()
	case 2:
		p.nodeName.Focus()
	case 3:
		p.region.Focus()
	}
}

func (p *AgentConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			if p.focus < agentFieldCount-1 {
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
		cmd = p.controllerURL.Update(msg)
	case 1:
		cmd = p.enrollToken.Update(msg)
	case 2:
		cmd = p.nodeName.Update(msg)
	case 3:
		cmd = p.region.Update(msg)
	}
	return p, cmd
}

func (p *AgentConfigPage) ScrollOffset() int {
	offsets := []int{0, 9, 12, 15}
	if p.focus < len(offsets) {
		return offsets[p.focus]
	}
	return 0
}

func (p *AgentConfigPage) Validate() bool {
	valid := true
	if !p.controllerURL.RunValidation() {
		valid = false
	}
	if !p.enrollToken.RunValidation() {
		valid = false
	}
	return valid
}

func (p *AgentConfigPage) Apply(cfg *config.Config) {
	cfg.ControllerURL = strings.TrimSpace(p.controllerURL.Value())
	cfg.EnrollmentToken = strings.TrimSpace(p.enrollToken.Value())
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
}

func (p *AgentConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Endpoint Agent Settings"))
	b.WriteString("\n\n")
	b.WriteString(common.WarningStyle.Render("  This device's FIPS compliance posture will be continuously"))
	b.WriteString("\n")
	b.WriteString(common.WarningStyle.Render("  reported to the fleet controller. Non-compliant devices"))
	b.WriteString("\n")
	b.WriteString(common.WarningStyle.Render("  will be denied access to protected services."))
	b.WriteString("\n\n")
	b.WriteString(p.controllerURL.View())
	b.WriteString("\n\n")
	b.WriteString(p.enrollToken.View())
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	return b.String()
}

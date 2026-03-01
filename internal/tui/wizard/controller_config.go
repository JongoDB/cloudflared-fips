package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ControllerConfigPage collects controller-specific settings.
type ControllerConfigPage struct {
	adminKey common.TextInput
	nodeName common.TextInput
	region   common.TextInput
	withCF   common.Toggle

	focus  int
	width  int
	height int
}

const controllerFieldCount = 4

// NewControllerConfigPage creates the controller config page.
func NewControllerConfigPage() *ControllerConfigPage {
	adminKey := common.NewTextInput("Admin API Key", "auto-generated if empty", "(leave blank to auto-generate)")
	nodeName := common.NewTextInput("Node Name", defaultHostname(), "")
	nodeName.Input.SetValue(defaultHostname())
	region := common.NewTextInput("Node Region", "us-east", "(optional label)")

	return &ControllerConfigPage{
		adminKey: adminKey,
		nodeName: nodeName,
		region:   region,
		withCF:   common.NewToggle("Enable Cloudflare API integration", "Wire dashboard to CF API for edge compliance checks", false),
	}
}

func (p *ControllerConfigPage) Title() string { return "Controller Config" }
func (p *ControllerConfigPage) Init() tea.Cmd { return nil }

func (p *ControllerConfigPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return p.adminKey.Focus()
}

func (p *ControllerConfigPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ControllerConfigPage) updateFocus() {
	p.adminKey.Blur()
	p.nodeName.Blur()
	p.region.Blur()
	p.withCF.Blur()

	switch p.focus {
	case 0:
		p.adminKey.Input.Focus()
	case 1:
		p.nodeName.Input.Focus()
	case 2:
		p.region.Input.Focus()
	case 3:
		p.withCF.Focus()
	}
}

func (p *ControllerConfigPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "enter":
			if p.focus < controllerFieldCount-1 {
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
		cmd = p.adminKey.Update(msg)
	case 1:
		cmd = p.nodeName.Update(msg)
	case 2:
		cmd = p.region.Update(msg)
	case 3:
		p.withCF.Update(msg)
	}
	return p, cmd
}

func (p *ControllerConfigPage) Validate() bool { return true }

func (p *ControllerConfigPage) Apply(cfg *config.Config) {
	cfg.AdminKey = strings.TrimSpace(p.adminKey.Value())
	cfg.NodeName = strings.TrimSpace(p.nodeName.Value())
	cfg.NodeRegion = strings.TrimSpace(p.region.Value())
	cfg.WithCF = p.withCF.Enabled
}

func (p *ControllerConfigPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Fleet Controller Settings"))
	b.WriteString("\n\n")
	b.WriteString(p.adminKey.View())
	b.WriteString("\n\n")
	b.WriteString(p.nodeName.View())
	b.WriteString("\n\n")
	b.WriteString(p.region.View())
	b.WriteString("\n\n")
	b.WriteString(p.withCF.View())
	return b.String()
}

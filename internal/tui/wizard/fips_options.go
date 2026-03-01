package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// FIPSOptionsPage is page 4: self-test toggles, signature verification.
type FIPSOptionsPage struct {
	selfTestOnStart   common.Toggle
	failOnSelfTest    common.Toggle
	verifySignature   common.Toggle
	selfTestOutput    common.TextInput
	focus             int
	width             int
	height            int
}

const fipsFieldCount = 4

// NewFIPSOptionsPage creates page 4.
func NewFIPSOptionsPage() *FIPSOptionsPage {
	output := common.NewTextInput("Self-Test Output Path", "/var/log/cloudflared/selftest.json", "")
	output.Input.SetValue("/var/log/cloudflared/selftest.json")

	return &FIPSOptionsPage{
		selfTestOnStart: common.NewToggle("Run self-test on startup", "Validates FIPS crypto before accepting connections", true),
		failOnSelfTest:  common.NewToggle("Fail on self-test failure", "Refuse to start if crypto validation fails (recommended)", true),
		verifySignature: common.NewToggle("Verify binary signature", "Check GPG signature at startup (requires public key)", false),
		selfTestOutput:  output,
	}
}

func (p *FIPSOptionsPage) Title() string { return "FIPS Options" }

func (p *FIPSOptionsPage) Init() tea.Cmd { return nil }

func (p *FIPSOptionsPage) Focus() tea.Cmd {
	p.focus = 0
	p.updateFocus()
	return fieldNav
}

func (p *FIPSOptionsPage) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *FIPSOptionsPage) updateFocus() {
	p.selfTestOnStart.Blur()
	p.failOnSelfTest.Blur()
	p.verifySignature.Blur()
	p.selfTestOutput.Blur()

	switch p.focus {
	case 0:
		p.selfTestOnStart.Focus()
	case 1:
		p.failOnSelfTest.Focus()
	case 2:
		p.verifySignature.Focus()
	case 3:
		p.selfTestOutput.Focus()
	}
}

func (p *FIPSOptionsPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab":
			if p.focus < fipsFieldCount-1 {
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

	switch p.focus {
	case 0:
		p.selfTestOnStart.Update(msg)
	case 1:
		p.failOnSelfTest.Update(msg)
	case 2:
		p.verifySignature.Update(msg)
	case 3:
		cmd := p.selfTestOutput.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p *FIPSOptionsPage) Validate() bool {
	return true
}

func (p *FIPSOptionsPage) Apply(cfg *config.Config) {
	cfg.FIPS.SelfTestOnStart = p.selfTestOnStart.Enabled
	cfg.FIPS.FailOnSelfTestFailure = p.failOnSelfTest.Enabled
	cfg.FIPS.VerifySignature = p.verifySignature.Enabled
	cfg.FIPS.SelfTestOutput = strings.TrimSpace(p.selfTestOutput.Value())
}

func (p *FIPSOptionsPage) View() string {
	var b strings.Builder
	b.WriteString(common.LabelStyle.Render("Self-Test Configuration"))
	b.WriteString("\n\n")
	b.WriteString(p.selfTestOnStart.View())
	b.WriteString("\n")
	b.WriteString(p.failOnSelfTest.View())
	b.WriteString("\n")
	b.WriteString(p.verifySignature.View())
	b.WriteString("\n\n")
	b.WriteString(p.selfTestOutput.View())
	return b.String()
}

package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// configWrittenMsg is sent when the config file has been written.
type configWrittenMsg struct {
	path string
	err  error
}

// ReviewPage is page 5: read-only summary, write config on Enter.
type ReviewPage struct {
	cfg      *config.Config
	viewport viewport.Model
	width    int
	height   int
	writing  bool
	written  bool
	writePath string
	writeErr  error
}

// NewReviewPage creates page 5.
func NewReviewPage() *ReviewPage {
	vp := viewport.New(80, 20)
	return &ReviewPage{
		viewport: vp,
	}
}

func (p *ReviewPage) Title() string { return "Review & Write" }

func (p *ReviewPage) Init() tea.Cmd { return nil }

func (p *ReviewPage) Focus() tea.Cmd { return nil }

func (p *ReviewPage) SetSize(w, h int) {
	p.width = w
	p.height = h
	// Reserve space for header/footer
	contentH := h - 6
	if contentH < 5 {
		contentH = 5
	}
	p.viewport.Width = w - 4
	p.viewport.Height = contentH
}

// SetConfig updates the review page with the accumulated config.
func (p *ReviewPage) SetConfig(cfg *config.Config) {
	p.cfg = cfg
	p.viewport.SetContent(p.renderSummary())
	p.viewport.GotoTop()
}

func (p *ReviewPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case configWrittenMsg:
		p.writing = false
		if msg.err != nil {
			p.writeErr = msg.err
		} else {
			p.written = true
			p.writePath = msg.path
		}
		return p, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if !p.writing && !p.written {
				p.writing = true
				cfg := p.cfg
				return p, func() tea.Msg {
					path := "configs/cloudflared-fips.yaml"
					err := config.WriteConfig(path, cfg)
					return configWrittenMsg{path: path, err: err}
				}
			}
		}
	}

	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p *ReviewPage) Validate() bool { return true }

func (p *ReviewPage) Apply(_ *config.Config) {}

func (p *ReviewPage) View() string {
	if p.written {
		return p.renderSuccess()
	}
	if p.writeErr != nil {
		return p.viewport.View() + "\n\n" +
			common.ErrorStyle.Render(fmt.Sprintf("Error writing config: %v", p.writeErr)) + "\n" +
			common.HintStyle.Render("Press Enter to retry")
	}
	if p.writing {
		return p.viewport.View() + "\n\n" +
			common.MutedStyle.Render("Writing configuration...")
	}
	return p.viewport.View() + "\n\n" +
		common.HintStyle.Render("Scroll with arrow keys | Press Enter to write config | Ctrl+C to cancel")
}

func (p *ReviewPage) renderSuccess() string {
	var b strings.Builder
	b.WriteString(common.SuccessStyle.Render("Configuration written successfully!"))
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("File: "))
	b.WriteString(p.writePath)
	b.WriteString("\n\n")
	b.WriteString(common.LabelStyle.Render("Next steps:"))
	b.WriteString("\n")
	b.WriteString("  1. Review the generated config file\n")
	b.WriteString("  2. Start the dashboard:  go run ./cmd/dashboard\n")
	b.WriteString("  3. Run the self-test:    go run ./cmd/selftest\n")
	b.WriteString("  4. Monitor compliance:   go run ./cmd/tui status\n")
	b.WriteString("\n")
	b.WriteString(common.HintStyle.Render("Press Ctrl+C to exit"))
	return b.String()
}

func (p *ReviewPage) renderSummary() string {
	if p.cfg == nil {
		return common.MutedStyle.Render("(no configuration to review)")
	}
	cfg := p.cfg
	var b strings.Builder
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(common.ColorPrimary)
	fieldStyle := lipgloss.NewStyle().Foreground(common.ColorDim).Width(24)

	field := func(label, value string) {
		b.WriteString("  " + fieldStyle.Render(label) + " " + value + "\n")
	}

	masked := func(label, value string) {
		display := "(not set)"
		if value != "" {
			display = "****" + last4(value)
		}
		field(label, display)
	}

	// Tunnel
	b.WriteString(sectionStyle.Render("TUNNEL"))
	b.WriteString("\n")
	field("Tunnel UUID:", cfg.Tunnel)
	field("Credentials File:", cfg.CredentialsFile)
	field("Protocol:", cfg.Protocol)
	b.WriteString("  " + fieldStyle.Render("Ingress Rules:") + "\n")
	for _, rule := range cfg.Ingress {
		if rule.Hostname != "" {
			b.WriteString("    " + rule.Hostname + " → " + rule.Service + "\n")
		} else {
			b.WriteString("    * → " + rule.Service + "\n")
		}
	}
	b.WriteString("\n")

	// Dashboard Wiring
	b.WriteString(sectionStyle.Render("DASHBOARD WIRING"))
	b.WriteString("\n")
	masked("CF API Token:", cfg.Dashboard.CFAPIToken)
	field("Zone ID:", cfg.Dashboard.ZoneID)
	field("Account ID:", cfg.Dashboard.AccountID)
	field("Tunnel ID:", cfg.Dashboard.TunnelID)
	field("Metrics Address:", cfg.Dashboard.MetricsAddress)
	field("MDM Provider:", displayMDM(cfg.Dashboard.MDM.Provider))
	switch cfg.Dashboard.MDM.Provider {
	case "intune":
		field("  Tenant ID:", cfg.Dashboard.MDM.TenantID)
		field("  Client ID:", cfg.Dashboard.MDM.ClientID)
		masked("  Client Secret:", cfg.Dashboard.MDM.ClientSecret)
	case "jamf":
		field("  Base URL:", cfg.Dashboard.MDM.BaseURL)
		masked("  API Token:", cfg.Dashboard.MDM.APIToken)
	}
	b.WriteString("\n")

	// Deployment Tier
	b.WriteString(sectionStyle.Render("DEPLOYMENT TIER"))
	b.WriteString("\n")
	field("Tier:", cfg.DeploymentTier)
	switch cfg.DeploymentTier {
	case "regional_keyless":
		field("Keyless SSL Host:", cfg.KeylessSSLHost)
		field("Keyless SSL Port:", fmt.Sprintf("%d", cfg.KeylessSSLPort))
		field("Regional Services:", fmt.Sprintf("%v", cfg.RegionalServices))
	case "self_hosted":
		field("Proxy Listen:", cfg.ProxyListenAddr)
		field("Proxy Cert:", cfg.ProxyCertFile)
		field("Proxy Key:", cfg.ProxyKeyFile)
		field("Proxy Upstream:", cfg.ProxyUpstream)
	}
	b.WriteString("\n")

	// FIPS Options
	b.WriteString(sectionStyle.Render("FIPS OPTIONS"))
	b.WriteString("\n")
	field("Self-test on start:", fmt.Sprintf("%v", cfg.FIPS.SelfTestOnStart))
	field("Fail on self-test:", fmt.Sprintf("%v", cfg.FIPS.FailOnSelfTestFailure))
	field("Verify signature:", fmt.Sprintf("%v", cfg.FIPS.VerifySignature))
	field("Self-test output:", cfg.FIPS.SelfTestOutput)

	return b.String()
}

func last4(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[len(s)-4:]
}

func displayMDM(provider string) string {
	switch provider {
	case "intune":
		return "Microsoft Intune"
	case "jamf":
		return "Jamf Pro"
	case "none", "":
		return "None"
	default:
		return provider
	}
}

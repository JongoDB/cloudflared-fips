package wizard

import (
	"fmt"
	"os"
	"os/exec"
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

// execDoneMsg is sent when a subprocess launched via tea.ExecProcess finishes.
type execDoneMsg struct{ err error }

// dashStartedMsg is sent after the background dashboard process starts.
type dashStartedMsg struct {
	cmd *exec.Cmd
	err error
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

	// Post-write interactive next steps
	nextSteps    common.Selector
	running      string    // action currently exec'd: "selftest" or "dashboard"
	selftestDone bool      // self-test returned successfully
	dashRunning  bool      // dashboard background process is alive
	dashCmd      *exec.Cmd // background dashboard process handle
	execErr      error     // last error from an exec'd action
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
			p.nextSteps = common.NewSelector("What's next?", []common.SelectorOption{
				{Value: "selftest", Label: "Run self-test", Description: "Verify FIPS compliance of this build"},
				{Value: "dashboard", Label: "Launch dashboard & status monitor", Description: "Start dashboard server and open the live monitor"},
				{Value: "exit", Label: "Exit", Description: "Return to the terminal"},
			})
			p.nextSteps.Focus()
		}
		return p, nil

	case dashStartedMsg:
		if msg.err != nil {
			p.execErr = msg.err
			p.running = ""
			return p, nil
		}
		p.dashRunning = true
		p.dashCmd = msg.cmd
		return p, tea.ExecProcess(statusMonitorCmd(), func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})

	case execDoneMsg:
		if p.running == "selftest" && msg.err == nil {
			p.selftestDone = true
		}
		if msg.err != nil {
			p.execErr = msg.err
		} else {
			p.execErr = nil
		}
		p.running = ""
		p.nextSteps.Focus()
		return p, nil

	case tea.KeyMsg:
		if p.written {
			switch msg.String() {
			case "enter":
				return p.dispatchAction()
			default:
				p.nextSteps.Update(msg)
				return p, nil
			}
		}
		switch msg.String() {
		case "enter":
			if !p.writing {
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

	if !p.written {
		var cmd tea.Cmd
		p.viewport, cmd = p.viewport.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p *ReviewPage) dispatchAction() (Page, tea.Cmd) {
	p.execErr = nil
	switch p.nextSteps.Selected() {
	case "selftest":
		p.running = "selftest"
		return p, tea.ExecProcess(selftestCmd(), func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})
	case "dashboard":
		p.running = "dashboard"
		configPath := p.writePath
		return p, func() tea.Msg {
			cmd, err := startDashboard(configPath)
			return dashStartedMsg{cmd: cmd, err: err}
		}
	case "exit":
		return p, tea.Quit
	}
	return p, nil
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
	b.WriteString("\n")
	b.WriteString(common.LabelStyle.Render("File: "))
	b.WriteString(p.writePath)
	b.WriteString("\n\n")

	b.WriteString(p.nextSteps.View())

	if p.selftestDone {
		b.WriteString("\n")
		b.WriteString(common.SuccessStyle.Render("  Self-test completed successfully"))
	}
	if p.dashRunning {
		b.WriteString("\n")
		b.WriteString(common.SuccessStyle.Render("  Dashboard running on localhost:8080"))
	}
	if p.execErr != nil {
		b.WriteString("\n")
		b.WriteString(common.ErrorStyle.Render(fmt.Sprintf("  Error: %v", p.execErr)))
	}
	return b.String()
}

// selftestCmd returns an exec.Cmd for the self-test binary.
// Prefers a compiled binary on PATH, falls back to go run.
func selftestCmd() *exec.Cmd {
	if path, err := exec.LookPath("cloudflared-fips-selftest"); err == nil {
		return exec.Command(path)
	}
	return exec.Command("go", "run", "./cmd/selftest")
}

// statusMonitorCmd returns an exec.Cmd for the TUI status monitor.
// Uses the same binary (os.Args[0]) with the "status" subcommand.
func statusMonitorCmd() *exec.Cmd {
	return exec.Command(os.Args[0], "status")
}

// startDashboard launches the dashboard server as a background process.
// Output is suppressed to avoid interfering with the TUI.
func startDashboard(configPath string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	if path, err := exec.LookPath("cloudflared-fips-dashboard"); err == nil {
		cmd = exec.Command(path, "--config", configPath)
	} else {
		cmd = exec.Command("go", "run", "./cmd/dashboard", "--config", configPath)
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	if err := cmd.Start(); err != nil {
		devNull.Close()
		return nil, err
	}
	return cmd, nil
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

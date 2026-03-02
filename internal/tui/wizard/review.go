package wizard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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

// ReviewPage is the final wizard page: read-only summary → write config →
// provision or run self-test.
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
	running      string    // action currently exec'd: "selftest", "dashboard", "provision"
	selftestDone bool      // self-test has been run (pass or fail)
	selftestErr  error     // nil = all passed, non-nil = failures reported
	dashRunning  bool      // dashboard background process is alive
	dashCmd      *exec.Cmd // background dashboard process handle
	execErr      error     // last error from a non-selftest exec'd action
	provisionDone bool
	provisionErr  error
}

// NewReviewPage creates the review page.
func NewReviewPage() *ReviewPage {
	vp := viewport.New(80, 20)
	return &ReviewPage{
		viewport: vp,
	}
}

func (p *ReviewPage) Title() string { return "Review & Provision" }

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
			p.nextSteps = p.buildNextSteps()
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
		switch p.running {
		case "selftest":
			p.selftestDone = true
			p.selftestErr = msg.err
		case "provision":
			p.provisionDone = true
			p.provisionErr = msg.err
		default:
			if msg.err != nil {
				p.execErr = msg.err
			} else {
				p.execErr = nil
			}
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

func (p *ReviewPage) buildNextSteps() common.Selector {
	opts := []common.SelectorOption{
		{Value: "provision", Label: "Provision this node", Description: "Write config, build, install, and start services"},
		{Value: "selftest", Label: "Run self-test only", Description: "Verify FIPS compliance of this build"},
		{Value: "dashboard", Label: "Launch dashboard & status monitor", Description: "Start dashboard server and open the live monitor"},
		{Value: "exit", Label: "Exit", Description: "Return to the terminal"},
	}
	return common.NewSelector("What's next?", opts)
}

func (p *ReviewPage) dispatchAction() (Page, tea.Cmd) {
	p.execErr = nil
	switch p.nextSteps.Selected() {
	case "provision":
		p.running = "provision"
		cmd := p.buildProvisionExec()
		return p, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})
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

// buildProvisionExec constructs the exec.Cmd for the provision script.
func (p *ReviewPage) buildProvisionExec() *exec.Cmd {
	script, args := BuildProvisionCommand(p.cfg)

	// Wrap in a shell with a pause so the user can read output before TUI resumes.
	// If not root, prepend sudo.
	fullCmd := script + " " + strings.Join(args, " ")
	if os.Geteuid() != 0 {
		fullCmd = "sudo " + fullCmd
	}
	shellScript := fullCmd + `; rc=$?; echo ""; echo "Press Enter to return to wizard..."; read _; exit $rc`
	return exec.Command("sh", "-c", shellScript)
}

// findProvisionScript returns the path to the provision script, checking
// installed locations first (RPM/DEB install to /usr/local/bin), then
// falling back to relative paths for development.
func findProvisionScript() string {
	// Installed location (RPM, DEB, pkg all install here)
	if _, err := os.Stat("/usr/local/bin/cloudflared-fips-provision"); err == nil {
		return "/usr/local/bin/cloudflared-fips-provision"
	}
	// Development fallback
	switch runtime.GOOS {
	case "darwin":
		return "./scripts/provision-macos.sh"
	default:
		return "./scripts/provision-linux.sh"
	}
}

// BuildProvisionCommand returns the script path and arguments for provisioning
// based on the config. Exported for testing.
func BuildProvisionCommand(cfg *config.Config) (script string, args []string) {
	script = findProvisionScript()

	args = append(args, "--role", cfg.Role)
	args = append(args, "--tier", tierNumber(cfg.DeploymentTier))

	// Controller: tunnel + compliance policy
	if cfg.Role == "controller" {
		if cfg.TunnelToken != "" {
			args = append(args, "--tunnel-token", cfg.TunnelToken)
		}
		if cfg.Protocol != "" {
			args = append(args, "--protocol", cfg.Protocol)
		}
		if cfg.AdminKey != "" {
			args = append(args, "--admin-key", cfg.AdminKey)
		}
		if cfg.CompliancePolicy.EnforcementMode != "" {
			args = append(args, "--enforcement-mode", cfg.CompliancePolicy.EnforcementMode)
		}
		if cfg.CompliancePolicy.RequireOSFIPS {
			args = append(args, "--require-os-fips")
		}
		if cfg.CompliancePolicy.RequireDiskEnc {
			args = append(args, "--require-disk-enc")
		}
	}

	// Server: service endpoint + mandatory enrollment
	if cfg.Role == "server" {
		if cfg.ServiceName != "" {
			args = append(args, "--service-name", cfg.ServiceName)
		}
		if cfg.ServiceHost != "" {
			args = append(args, "--service-host", cfg.ServiceHost)
		}
		if cfg.ServicePort > 0 {
			args = append(args, "--service-port", fmt.Sprintf("%d", cfg.ServicePort))
		}
		if cfg.ServiceTLS {
			args = append(args, "--service-tls")
		}
	}

	// Proxy: tunnel + TLS termination
	if cfg.Role == "proxy" {
		if cfg.TunnelToken != "" {
			args = append(args, "--tunnel-token", cfg.TunnelToken)
		}
		if cfg.Protocol != "" {
			args = append(args, "--protocol", cfg.Protocol)
		}
		if cfg.ProxyCertFile != "" {
			args = append(args, "--cert", cfg.ProxyCertFile)
		}
		if cfg.ProxyKeyFile != "" {
			args = append(args, "--key", cfg.ProxyKeyFile)
		}
	}

	// Fleet enrollment (server, proxy, client — all mandatory)
	if cfg.Role != "controller" {
		if cfg.ControllerURL != "" {
			args = append(args, "--controller-url", cfg.ControllerURL)
		}
		if cfg.EnrollmentToken != "" {
			args = append(args, "--enrollment-token", cfg.EnrollmentToken)
		}
	}

	// Node identity
	if cfg.NodeName != "" {
		args = append(args, "--node-name", cfg.NodeName)
	}
	if cfg.NodeRegion != "" {
		args = append(args, "--node-region", cfg.NodeRegion)
	}

	// Cloudflare API credentials (controller only)
	if cfg.Role == "controller" {
		if cfg.Dashboard.CFAPIToken != "" {
			args = append(args, "--cf-api-token", cfg.Dashboard.CFAPIToken)
		}
		if cfg.Dashboard.ZoneID != "" {
			args = append(args, "--cf-zone-id", cfg.Dashboard.ZoneID)
		}
		if cfg.Dashboard.AccountID != "" {
			args = append(args, "--cf-account-id", cfg.Dashboard.AccountID)
		}
		if cfg.Dashboard.TunnelID != "" {
			args = append(args, "--cf-tunnel-id", cfg.Dashboard.TunnelID)
		}
	}

	// Skip FIPS
	if cfg.SkipFIPS {
		args = append(args, "--no-fips")
	}

	return script, args
}

func (p *ReviewPage) ScrollOffset() int { return 0 }
func (p *ReviewPage) Validate() bool    { return true }

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

	if p.provisionDone {
		b.WriteString("\n")
		if p.provisionErr != nil {
			b.WriteString(common.WarningStyle.Render(fmt.Sprintf("  Provisioning finished with errors: %v", p.provisionErr)))
		} else {
			b.WriteString(common.SuccessStyle.Render("  Provisioning completed successfully"))
		}
	}
	if p.selftestDone {
		b.WriteString("\n")
		if p.selftestErr != nil {
			b.WriteString(common.WarningStyle.Render("  Self-test completed with failures (expected in dev without BoringCrypto)"))
		} else {
			b.WriteString(common.SuccessStyle.Render("  Self-test passed"))
		}
	}
	if p.dashRunning {
		b.WriteString("\n")
		b.WriteString(common.SuccessStyle.Render("  Dashboard running on localhost:8080"))
	}
	if p.execErr != nil {
		b.WriteString("\n")
		b.WriteString(common.ErrorStyle.Render(fmt.Sprintf("  Error: %v", p.execErr)))
	}

	// FIPS reboot warning
	if p.cfg != nil && !p.cfg.SkipFIPS && runtime.GOOS == "linux" && !p.provisionDone {
		b.WriteString("\n\n")
		b.WriteString(common.WarningStyle.Render("  Note: Provisioning may reboot to enable OS FIPS mode."))
		b.WriteString("\n")
		b.WriteString(common.HintStyle.Render("  The script is idempotent — re-run after reboot."))
	}

	return b.String()
}

// selftestCmd returns an exec.Cmd for the self-test binary.
func selftestCmd() *exec.Cmd {
	var inner string
	if path, err := exec.LookPath("cloudflared-fips-selftest"); err == nil {
		inner = path
	} else {
		inner = "go run ./cmd/selftest"
	}
	script := inner + `; rc=$?; echo ""; echo "Press Enter to return to wizard..."; read _; exit $rc`
	return exec.Command("sh", "-c", script)
}

// statusMonitorCmd returns an exec.Cmd for the TUI status monitor.
func statusMonitorCmd() *exec.Cmd {
	return exec.Command(os.Args[0], "status")
}

// startDashboard launches the dashboard server as a background process.
func startDashboard(configPath string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	if path, err := exec.LookPath("cloudflared-fips-dashboard"); err == nil {
		cmd = exec.Command(path, "--config", configPath)
	} else {
		cmd = exec.Command("go", "run", "./cmd/dashboard", "--config", configPath)
	}
	env := os.Environ()
	hasFIPS := false
	for _, e := range env {
		if strings.HasPrefix(e, "GODEBUG=") || strings.HasPrefix(e, "GOEXPERIMENT=") {
			hasFIPS = true
			break
		}
	}
	if !hasFIPS {
		env = append(env, "GODEBUG=fips140=on")
	}
	cmd.Env = env

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

	// Role & Tier
	b.WriteString(sectionStyle.Render("ROLE & TIER"))
	b.WriteString("\n")
	field("Role:", cfg.Role)
	field("Deployment Tier:", cfg.DeploymentTier)
	if cfg.SkipFIPS {
		field("Skip FIPS:", "yes (dev mode)")
	}
	if cfg.NodeName != "" {
		field("Node Name:", cfg.NodeName)
	}
	if cfg.NodeRegion != "" {
		field("Node Region:", cfg.NodeRegion)
	}
	b.WriteString("\n")

	// Role-specific config
	switch cfg.Role {
	case "controller":
		b.WriteString(sectionStyle.Render("CONTROLLER"))
		b.WriteString("\n")
		masked("Admin Key:", cfg.AdminKey)
		b.WriteString("\n")

		b.WriteString(sectionStyle.Render("TUNNEL"))
		b.WriteString("\n")
		masked("Tunnel Token:", cfg.TunnelToken)
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

		b.WriteString(sectionStyle.Render("COMPLIANCE POLICY"))
		b.WriteString("\n")
		field("Enforcement Mode:", cfg.CompliancePolicy.EnforcementMode)
		field("Require OS FIPS:", fmt.Sprintf("%v", cfg.CompliancePolicy.RequireOSFIPS))
		field("Require Disk Enc:", fmt.Sprintf("%v", cfg.CompliancePolicy.RequireDiskEnc))
		if cfg.WithCF {
			field("CF API Integration:", "enabled")
		}
		b.WriteString("\n")

	case "server":
		b.WriteString(sectionStyle.Render("ORIGIN SERVICE"))
		b.WriteString("\n")
		field("Service Name:", cfg.ServiceName)
		field("Service Host:", cfg.ServiceHost)
		field("Service Port:", fmt.Sprintf("%d", cfg.ServicePort))
		field("Service TLS:", fmt.Sprintf("%v", cfg.ServiceTLS))
		b.WriteString("\n")

		b.WriteString(sectionStyle.Render("FLEET ENROLLMENT"))
		b.WriteString("\n")
		field("Controller URL:", cfg.ControllerURL)
		masked("Enrollment Token:", cfg.EnrollmentToken)
		b.WriteString("\n")

	case "proxy":
		b.WriteString(sectionStyle.Render("FIPS FORWARD PROXY"))
		b.WriteString("\n")
		field("Listen:", cfg.ProxyListenAddr)
		field("TLS Cert:", cfg.ProxyCertFile)
		field("TLS Key:", cfg.ProxyKeyFile)
		b.WriteString("\n")

		b.WriteString(sectionStyle.Render("PROXY TUNNEL"))
		b.WriteString("\n")
		masked("Tunnel Token:", cfg.TunnelToken)
		field("Protocol:", cfg.Protocol)
		b.WriteString("\n")

		b.WriteString(sectionStyle.Render("FLEET ENROLLMENT"))
		b.WriteString("\n")
		field("Controller URL:", cfg.ControllerURL)
		masked("Enrollment Token:", cfg.EnrollmentToken)
		b.WriteString("\n")

	case "client":
		b.WriteString(sectionStyle.Render("AGENT"))
		b.WriteString("\n")
		field("Controller URL:", cfg.ControllerURL)
		masked("Enrollment Token:", cfg.EnrollmentToken)
		b.WriteString("\n")
	}

	// Dashboard Wiring (controller only)
	if cfg.Role == "controller" {
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
	}

	// Tier-specific (controller only)
	if cfg.Role == "controller" {
		switch cfg.DeploymentTier {
		case "regional_keyless":
			b.WriteString(sectionStyle.Render("TIER 2 — KEYLESS SSL"))
			b.WriteString("\n")
			field("Keyless SSL Host:", cfg.KeylessSSLHost)
			field("Keyless SSL Port:", fmt.Sprintf("%d", cfg.KeylessSSLPort))
			field("Regional Services:", fmt.Sprintf("%v", cfg.RegionalServices))
			b.WriteString("\n")
		case "self_hosted":
			b.WriteString(sectionStyle.Render("TIER 3 — FIPS PROXY"))
			b.WriteString("\n")
			field("Proxy Listen:", cfg.ProxyListenAddr)
			field("Proxy Cert:", cfg.ProxyCertFile)
			field("Proxy Key:", cfg.ProxyKeyFile)
			b.WriteString("\n")
		}
	}

	// FIPS Options
	b.WriteString(sectionStyle.Render("FIPS OPTIONS"))
	b.WriteString("\n")
	field("Self-test on start:", fmt.Sprintf("%v", cfg.FIPS.SelfTestOnStart))
	field("Fail on self-test:", fmt.Sprintf("%v", cfg.FIPS.FailOnSelfTestFailure))
	field("Verify signature:", fmt.Sprintf("%v", cfg.FIPS.VerifySignature))
	field("Self-test output:", cfg.FIPS.SelfTestOutput)
	b.WriteString("\n")

	// Provision command preview (mask secrets)
	script, args := BuildProvisionCommand(cfg)
	maskedArgs := maskSecretArgs(args)
	provCmd := script + " " + strings.Join(maskedArgs, " ")
	if os.Geteuid() != 0 {
		provCmd = "sudo " + provCmd
	}
	b.WriteString(sectionStyle.Render("PROVISION COMMAND"))
	b.WriteString("\n")
	b.WriteString("  " + common.MutedStyle.Render(provCmd))
	b.WriteString("\n")

	return b.String()
}

// maskSecretArgs replaces secret values in the args list with masked forms.
func maskSecretArgs(args []string) []string {
	secretFlags := map[string]bool{
		"--tunnel-token":    true,
		"--enrollment-token": true,
		"--admin-key":       true,
		"--cf-api-token":    true,
	}
	masked := make([]string, len(args))
	for i, a := range args {
		if i > 0 && secretFlags[args[i-1]] {
			masked[i] = "****" + last4(a)
		} else {
			masked[i] = a
		}
	}
	return masked
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

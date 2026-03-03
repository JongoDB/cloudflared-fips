// Package menu implements the main menu TUI shown when cloudflared-fips
// is run with no subcommand. It lets users access all functionality from
// a single interactive interface.
package menu

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

// execDoneMsg is sent when a subprocess launched via tea.ExecProcess finishes.
type execDoneMsg struct{ err error }

// MenuModel is the main menu shown when cloudflared-fips is run with no args.
type MenuModel struct {
	selector   common.Selector
	width      int
	height     int
	lastErr    error
	lastAction string
}

// NewMenuModel creates the main menu.
func NewMenuModel() MenuModel {
	opts := []common.SelectorOption{
		{Value: "setup", Label: "Setup wizard", Description: "Configure and provision this node"},
		{Value: "status", Label: "Status monitor", Description: "Live compliance dashboard in terminal"},
		{Value: "selftest", Label: "Self-test", Description: "Verify FIPS compliance of this build"},
		{Value: "dashboard", Label: "Dashboard", Description: "Open web dashboard + status monitor"},
		{Value: "upgrade", Label: "Upgrade", Description: "Update binaries, preserve config + data"},
		{Value: "unprovision", Label: "Unprovision", Description: "Remove services, configs, and binaries"},
		{Value: "exit", Label: "Exit", Description: "Return to the terminal"},
	}
	s := common.NewSelector("", opts)
	s.Focus()
	return MenuModel{selector: s}
}

func (m MenuModel) Init() tea.Cmd { return nil }

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case execDoneMsg:
		m.lastErr = msg.err
		m.lastAction = ""
		m.selector.Focus()
		return m, nil

	case dashboardReadyMsg:
		// Dashboard is running in background — launch status monitor
		return m, tea.ExecProcess(
			exec.Command(os.Args[0], "status"),
			func(err error) tea.Msg { return execDoneMsg{err: err} },
		)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.dispatch()
		default:
			m.selector.Update(msg)
		}
	}
	return m, nil
}

func (m MenuModel) dispatch() (tea.Model, tea.Cmd) {
	m.lastErr = nil
	action := m.selector.Selected()
	m.lastAction = action

	switch action {
	case "setup":
		return m, tea.ExecProcess(
			exec.Command(os.Args[0], "setup"),
			func(err error) tea.Msg { return execDoneMsg{err: err} },
		)
	case "status":
		return m, tea.ExecProcess(
			exec.Command(os.Args[0], "status"),
			func(err error) tea.Msg { return execDoneMsg{err: err} },
		)
	case "selftest":
		return m, tea.ExecProcess(
			buildSelftestCmd(),
			func(err error) tea.Msg { return execDoneMsg{err: err} },
		)
	case "dashboard":
		return m, m.dashboardCmd()
	case "upgrade":
		return m, tea.ExecProcess(
			buildUpgradeCmd(),
			func(err error) tea.Msg { return execDoneMsg{err: err} },
		)
	case "unprovision":
		return m, tea.ExecProcess(
			buildUnprovisionCmd(),
			func(err error) tea.Msg { return execDoneMsg{err: err} },
		)
	case "exit":
		return m, tea.Quit
	}
	return m, nil
}

func (m MenuModel) View() string {
	var b strings.Builder

	// Banner
	title := fmt.Sprintf("cloudflared-fips %s", buildinfo.Version)
	subtitle := "FIPS 140-3 Compliant Tunnel Client"

	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.ColorPrimary).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(common.ColorMuted).
		Padding(0, 2).
		Align(lipgloss.Center).
		Render(title + "\n" + common.SubtitleStyle.Render(subtitle))

	b.WriteString(banner)
	b.WriteString("\n\n")

	// Menu options
	b.WriteString(m.selector.View())

	// Status line
	if m.lastErr != nil {
		b.WriteString("\n")
		b.WriteString(common.WarningStyle.Render(
			fmt.Sprintf("  Last action finished with errors: %v", m.lastErr)))
	}

	b.WriteString("\n")
	b.WriteString(common.HintStyle.Render("  arrow keys navigate | Enter select | q quit"))

	return common.PageStyle.Render(b.String())
}

// --- Command builders ---

func buildSelftestCmd() *exec.Cmd {
	path := common.FindBinary("cloudflared-fips-selftest")
	var inner string
	if path != "" {
		inner = path
	} else {
		inner = "go run ./cmd/selftest"
	}
	script := inner + `; rc=$?; echo ""; echo "Press Enter to return to menu..."; read _; exit $rc`
	return exec.Command("sh", "-c", script)
}

// dashboardRunning checks if the dashboard server is already listening.
func dashboardRunning() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/api/v1/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// startDashboardBackground launches the dashboard server as a background process.
// Returns nil if already running.
func startDashboardBackground() error {
	if dashboardRunning() {
		return nil
	}

	path := common.FindBinary("cloudflared-fips-dashboard")
	var cmd *exec.Cmd
	if path != "" {
		cmd = exec.Command(path)
	} else {
		cmd = exec.Command("go", "run", "./cmd/dashboard")
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
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	if err := cmd.Start(); err != nil {
		devNull.Close()
		return fmt.Errorf("start dashboard: %w", err)
	}

	// Wait briefly for it to become ready
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if dashboardRunning() {
			return nil
		}
	}
	return nil // started but may still be initializing
}

// dashboardCmd ensures the dashboard is running in the background, then
// launches the TUI status monitor so the user gets live feedback.
func (m MenuModel) dashboardCmd() tea.Cmd {
	return func() tea.Msg {
		if err := startDashboardBackground(); err != nil {
			return execDoneMsg{err: fmt.Errorf("dashboard: %w", err)}
		}
		// Now hand off to the status monitor (non-blocking return
		// triggers ExecProcess in the next Update cycle).
		return dashboardReadyMsg{}
	}
}

// dashboardReadyMsg signals that the dashboard is running and we should
// launch the status monitor.
type dashboardReadyMsg struct{}

func buildUpgradeCmd() *exec.Cmd {
	script := common.FindUpgradeScript()
	fullCmd := script
	if os.Geteuid() != 0 {
		fullCmd = "sudo " + fullCmd
	}
	shellScript := fullCmd + `; rc=$?; echo ""; echo "Press Enter to return to menu..."; read _; exit $rc`
	return exec.Command("sh", "-c", shellScript)
}

func buildUnprovisionCmd() *exec.Cmd {
	script := common.FindUnprovisionScript()
	fullCmd := script
	if os.Geteuid() != 0 {
		fullCmd = "sudo " + fullCmd
	}
	shellScript := fullCmd + `; rc=$?; echo ""; echo "Press Enter to return to menu..."; read _; exit $rc`
	return exec.Command("sh", "-c", shellScript)
}

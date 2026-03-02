// Package menu implements the main menu TUI shown when cloudflared-fips
// is run with no subcommand. It lets users access all functionality from
// a single interactive interface.
package menu

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
		{Value: "dashboard", Label: "Dashboard", Description: "Start the web dashboard server"},
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
		return m, tea.ExecProcess(
			buildDashboardCmd(),
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

func buildDashboardCmd() *exec.Cmd {
	path := common.FindBinary("cloudflared-fips-dashboard")
	if path != "" {
		// Dashboard is a long-running server. Ctrl+C stops it and returns to menu.
		script := path + `; rc=$?; echo ""; echo "Press Enter to return to menu..."; read _; exit $rc`
		return exec.Command("sh", "-c", script)
	}
	// Development fallback
	script := `go run ./cmd/dashboard; rc=$?; echo ""; echo "Press Enter to return to menu..."; read _; exit $rc`
	return exec.Command("sh", "-c", script)
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

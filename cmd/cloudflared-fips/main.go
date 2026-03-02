// Command cloudflared-fips is the unified entry point for all cloudflared-fips
// functionality. Run with no arguments for an interactive main menu, or use a
// subcommand for direct access.
//
// Usage:
//
//	cloudflared-fips                    Interactive main menu
//	cloudflared-fips setup              Setup wizard
//	cloudflared-fips status             Live compliance status monitor
//	cloudflared-fips selftest           FIPS self-test suite
//	cloudflared-fips dashboard [flags]  Start dashboard server
//	cloudflared-fips proxy [flags]      Start FIPS edge proxy
//	cloudflared-fips agent [flags]      Start endpoint agent
//	cloudflared-fips provision [flags]  Run provisioning script
//	cloudflared-fips unprovision [flags] Run unprovisioning script
//	cloudflared-fips version            Show version info
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/common"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/menu"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/status"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/wizard"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

func main() {
	if len(os.Args) < 2 {
		runMenu()
		return
	}

	switch os.Args[1] {
	case "menu":
		runMenu()
	case "setup":
		runSetup()
	case "status":
		runStatus(os.Args[2:])
	case "selftest":
		execBinary("cloudflared-fips-selftest", "cmd/selftest", os.Args[2:])
	case "dashboard":
		execBinary("cloudflared-fips-dashboard", "cmd/dashboard", os.Args[2:])
	case "proxy":
		execBinary("cloudflared-fips-proxy", "cmd/fips-proxy", os.Args[2:])
	case "agent":
		execBinary("cloudflared-fips-agent", "cmd/agent", os.Args[2:])
	case "provision":
		execScript(common.FindProvisionScript(), os.Args[2:])
	case "unprovision":
		execScript(common.FindUnprovisionScript(), os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println(buildinfo.String())
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runMenu() {
	m := menu.NewMenuModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSetup() {
	m := wizard.NewWizardModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	apiAddr := fs.String("api", "127.0.0.1:8080", "Dashboard API address (host:port)")
	interval := fs.Duration("interval", 5*time.Second, "Poll interval")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	m := status.NewStatusModel(*apiAddr, *interval)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// execBinary finds and executes a companion binary, passing through
// stdin/stdout/stderr and propagating the exit code.
func execBinary(name, devPkg string, args []string) {
	path := common.FindBinary(name)

	var cmd *exec.Cmd
	if path != "" {
		cmd = exec.Command(path, args...)
	} else {
		// Development fallback: go run
		goArgs := append([]string{"run", "./" + devPkg}, args...)
		cmd = exec.Command("go", goArgs...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error running %s: %v\n", name, err)
		os.Exit(1)
	}
}

// execScript finds and runs a shell script with the given args.
func execScript(script string, args []string) {
	cmd := exec.Command(script, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(buildinfo.String())
	fmt.Println()
	fmt.Println("Usage: cloudflared-fips [command] [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  (no command)     Interactive main menu")
	fmt.Println("  setup            Interactive setup wizard")
	fmt.Println("  status           Live compliance status monitor")
	fmt.Println("  selftest         Run FIPS self-test suite")
	fmt.Println("  dashboard        Start compliance dashboard server")
	fmt.Println("  proxy            Start FIPS edge proxy (Tier 3)")
	fmt.Println("  agent            Start endpoint FIPS posture agent")
	fmt.Println("  provision        Run provisioning script")
	fmt.Println("  unprovision      Run unprovisioning script")
	fmt.Println("  version          Show version information")
	fmt.Println("  help             Show this help message")
	fmt.Println()
	fmt.Println("Run 'cloudflared-fips [command] --help' for command-specific flags.")
}

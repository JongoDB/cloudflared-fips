// Command tui provides an interactive terminal UI for cloudflared-fips.
//
// Subcommands:
//
//	setup   — guided wizard that walks through configuration and writes
//	          configs/cloudflared-fips.yaml
//	status  — live compliance status monitor that polls the dashboard API
//
// Usage:
//
//	go run ./cmd/tui setup
//	go run ./cmd/tui status [--api localhost:8080] [--interval 5s]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/status"
	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/wizard"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		runSetup()
	case "status":
		runStatus(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("cloudflared-fips TUI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  tui setup                          Interactive configuration wizard")
	fmt.Println("  tui status [--api addr] [--interval duration]")
	fmt.Println("                                     Live compliance status monitor")
	fmt.Println()
	fmt.Println("Status flags:")
	fmt.Println("  --api       Dashboard API address (default: localhost:8080)")
	fmt.Println("  --interval  Poll interval (default: 5s)")
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

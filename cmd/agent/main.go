// Command agent is a lightweight FIPS posture reporting agent for endpoints.
//
// It performs OS-level FIPS compliance checks and reports results to a
// fleet controller. Designed to be small (~5MB), with no embedded frontend.
//
// Usage:
//
//	cloudflared-fips-agent --controller-url https://ctrl:8080 --node-id ID --api-key KEY
//	cloudflared-fips-agent --check    # run checks once and print results
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fipsbackend"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet"
)

func main() {
	controllerURL := flag.String("controller-url", "", "URL of fleet controller (required for reporting mode)")
	nodeID := flag.String("node-id", "", "node ID from enrollment (or set NODE_ID env)")
	apiKey := flag.String("api-key", "", "API key from enrollment (or set NODE_API_KEY env)")
	interval := flag.Duration("interval", 60*time.Second, "report interval")
	checkOnly := flag.Bool("check", false, "run checks once and print results (no reporting)")
	jsonOutput := flag.Bool("json", false, "output checks as JSON (with --check)")
	version := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *version {
		fmt.Println(buildinfo.String())
		os.Exit(0)
	}

	logger := log.New(os.Stderr, "[agent] ", log.LstdFlags)

	agentChecks := fleet.NewAgentChecks()

	// Check-only mode: run checks and exit
	if *checkOnly {
		section := agentChecks.RunChecks()
		if *jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(section)
		} else {
			printSection(section)
		}
		os.Exit(0)
	}

	// Reporting mode: push results to controller
	nID := envOrFlag(*nodeID, "NODE_ID")
	nKey := envOrFlag(*apiKey, "NODE_API_KEY")
	ctrlURL := envOrFlag(*controllerURL, "CONTROLLER_URL")

	if ctrlURL == "" || nID == "" || nKey == "" {
		fmt.Fprintln(os.Stderr, "Usage: cloudflared-fips-agent --controller-url URL --node-id ID --api-key KEY")
		fmt.Fprintln(os.Stderr, "       cloudflared-fips-agent --check  (run checks locally)")
		os.Exit(1)
	}

	logger.Printf("%s", buildinfo.String())
	logger.Printf("Controller: %s", ctrlURL)
	logger.Printf("Node ID: %s", nID)
	logger.Printf("Report interval: %s", *interval)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Build a checker with agent sections
	checker := compliance.NewChecker()
	checker.AddSection(agentChecks.RunChecks())

	reporter := fleet.NewReporter(fleet.ReporterConfig{
		ControllerURL: ctrlURL,
		NodeID:        nID,
		APIKey:        nKey,
		Checker:       checker,
		Interval:      *interval,
		Logger:        logger,
	})

	logger.Printf("Agent started, reporting every %s", *interval)
	reporter.Run(ctx)
	logger.Printf("Agent stopped")
}

func printSection(section compliance.Section) {
	fmt.Printf("=== %s ===\n", section.Name)
	for _, item := range section.Items {
		icon := "?"
		switch item.Status {
		case compliance.StatusPass:
			icon = "✓"
		case compliance.StatusFail:
			icon = "✗"
		case compliance.StatusWarning:
			icon = "!"
		}
		fmt.Printf("  [%s] %s: %s\n", icon, item.Name, item.Status)
		if item.Remediation != "" && item.Status != compliance.StatusPass {
			fmt.Printf("      → %s\n", item.Remediation)
		}
	}

	// Summary
	var pass, fail, warn, unknown int
	for _, item := range section.Items {
		switch item.Status {
		case compliance.StatusPass:
			pass++
		case compliance.StatusFail:
			fail++
		case compliance.StatusWarning:
			warn++
		default:
			unknown++
		}
	}
	fmt.Printf("\nSummary: %d passed, %d failed, %d warnings, %d unknown\n", pass, fail, warn, unknown)

	// FIPS backend
	info := fipsbackend.DetectInfo()
	fmt.Printf("FIPS Backend: %s (validated: %v)\n", info.Name, info.Validated)
}

func envOrFlag(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}

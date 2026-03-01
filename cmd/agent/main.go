// Command agent is a lightweight FIPS posture reporting agent for endpoints.
//
// It performs OS-level FIPS compliance checks and reports results to a
// fleet controller. Designed to be small (~5MB), with no embedded frontend.
//
// Usage:
//
//	cloudflared-fips-agent --controller-url https://ctrl:8080 --node-id ID --api-key KEY
//	cloudflared-fips-agent --check              # run checks once and print results
//	cloudflared-fips-agent --remediate          # run checks and fix what's possible
//	cloudflared-fips-agent --enable-remediation  # accept controller-driven remediation
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fipsbackend"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet/remediate"
)

func main() {
	controllerURL := flag.String("controller-url", "", "URL of fleet controller (required for reporting mode)")
	nodeID := flag.String("node-id", "", "node ID from enrollment (or set NODE_ID env)")
	apiKey := flag.String("api-key", "", "API key from enrollment (or set NODE_API_KEY env)")
	interval := flag.Duration("interval", 60*time.Second, "report interval")
	checkOnly := flag.Bool("check", false, "run checks once and print results (no reporting)")
	jsonOutput := flag.Bool("json", false, "output checks as JSON (with --check)")
	version := flag.Bool("version", false, "print version and exit")
	remediateFlag := flag.Bool("remediate", false, "run checks, fix auto-remediable issues, and exit")
	enableRemediation := flag.Bool("enable-remediation", false, "accept controller-driven remediation requests")

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

	// Remediate mode: run checks, fix what's possible, exit
	if *remediateFlag {
		section := agentChecks.RunChecks()
		executor := remediate.NewExecutor(logger)
		plan := executor.Plan(section)

		if len(plan) == 0 {
			fmt.Println("All checks passed — no remediation needed.")
			os.Exit(0)
		}

		fmt.Printf("Found %d remediation actions:\n\n", len(plan))
		for i, action := range plan {
			autoLabel := "AUTO"
			if !action.AutoExec {
				autoLabel = "MANUAL"
			}
			fmt.Printf("  %d. [%s] %s\n", i+1, autoLabel, action.Description)
			fmt.Printf("     %s\n\n", action.Instructions)
		}

		// Collect auto-remediable action IDs
		var autoActions []remediate.ActionID
		for _, action := range plan {
			if action.AutoExec {
				autoActions = append(autoActions, action.ID)
			}
		}

		if len(autoActions) == 0 {
			fmt.Println("No auto-remediable actions available. Follow the manual instructions above.")
			os.Exit(1)
		}

		fmt.Printf("Executing %d auto-remediable actions...\n\n", len(autoActions))
		req := remediate.RemediationRequest{
			ID:      "local-remediate",
			NodeID:  "local",
			Actions: autoActions,
		}
		result := executor.Execute(req, plan)

		exitCode := 0
		for _, action := range result.Actions {
			icon := "?"
			switch action.Status {
			case remediate.StatusSuccess:
				icon = "✓"
			case remediate.StatusNeedsReboot:
				icon = "↻"
			case remediate.StatusFailed:
				icon = "✗"
				exitCode = 1
			case remediate.StatusManualOnly:
				icon = "→"
			case remediate.StatusSkipped:
				icon = "-"
			}
			fmt.Printf("  [%s] %s: %s\n", icon, action.ID, action.Status)
			if action.Output != "" {
				fmt.Printf("      %s\n", action.Output)
			}
		}

		if *jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(result)
		}

		os.Exit(exitCode)
	}

	// Reporting mode: push results to controller
	nID := envOrFlag(*nodeID, "NODE_ID")
	nKey := envOrFlag(*apiKey, "NODE_API_KEY")
	ctrlURL := envOrFlag(*controllerURL, "CONTROLLER_URL")

	if ctrlURL == "" || nID == "" || nKey == "" {
		fmt.Fprintln(os.Stderr, "Usage: cloudflared-fips-agent --controller-url URL --node-id ID --api-key KEY")
		fmt.Fprintln(os.Stderr, "       cloudflared-fips-agent --check              (run checks locally)")
		fmt.Fprintln(os.Stderr, "       cloudflared-fips-agent --remediate          (fix auto-remediable issues)")
		fmt.Fprintln(os.Stderr, "       cloudflared-fips-agent --enable-remediation (accept controller remediation)")
		os.Exit(1)
	}

	logger.Printf("%s", buildinfo.String())
	logger.Printf("Controller: %s", ctrlURL)
	logger.Printf("Node ID: %s", nID)
	logger.Printf("Report interval: %s", *interval)
	if *enableRemediation {
		logger.Printf("Remediation: enabled (accepting controller requests)")
	}

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

	// Start reporter in background
	go func() {
		logger.Printf("Agent started, reporting every %s", *interval)
		reporter.Run(ctx)
	}()

	// If remediation enabled, also poll for controller-driven requests
	if *enableRemediation {
		go pollRemediations(ctx, logger, ctrlURL, nID, nKey, agentChecks, *interval)
	}

	<-ctx.Done()
	logger.Printf("Agent stopped")
}

// pollRemediations periodically checks the controller for pending remediation requests.
func pollRemediations(ctx context.Context, logger *log.Logger, ctrlURL, nodeID, apiKey string, checks *fleet.AgentChecks, interval time.Duration) {
	client := &http.Client{Timeout: 10 * time.Second}
	executor := remediate.NewExecutor(logger)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Poll for pending requests
			url := fmt.Sprintf("%s/api/v1/fleet/nodes/%s/remediate", ctrlURL, nodeID)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := client.Do(req)
			if err != nil {
				logger.Printf("remediation poll failed: %v", err)
				continue
			}

			var pending []struct {
				ID      string   `json:"id"`
				Actions []string `json:"actions"`
				DryRun  bool     `json:"dry_run"`
			}
			json.NewDecoder(resp.Body).Decode(&pending)
			resp.Body.Close()

			if len(pending) == 0 {
				continue
			}

			// Process each pending request
			for _, p := range pending {
				logger.Printf("processing remediation request %s (%d actions)", p.ID, len(p.Actions))

				section := checks.RunChecks()
				plan := executor.Plan(section)

				// Convert string actions to ActionIDs
				var actionIDs []remediate.ActionID
				for _, a := range p.Actions {
					actionIDs = append(actionIDs, remediate.ActionID(a))
				}

				remReq := remediate.RemediationRequest{
					ID:      p.ID,
					NodeID:  nodeID,
					Actions: actionIDs,
					DryRun:  p.DryRun,
				}
				result := executor.Execute(remReq, plan)

				// Post result back to controller
				resultJSON, _ := json.Marshal(result)
				postURL := fmt.Sprintf("%s/api/v1/fleet/nodes/%s/remediate/result", ctrlURL, nodeID)
				body := map[string]interface{}{
					"request_id": p.ID,
					"result":     json.RawMessage(resultJSON),
				}
				bodyJSON, _ := json.Marshal(body)
				postReq, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewReader(bodyJSON))
				if err != nil {
					continue
				}
				postReq.Header.Set("Authorization", "Bearer "+apiKey)
				postReq.Header.Set("Content-Type", "application/json")

				postResp, err := client.Do(postReq)
				if err != nil {
					logger.Printf("failed to post remediation result: %v", err)
					continue
				}
				postResp.Body.Close()
				logger.Printf("remediation request %s completed", p.ID)
			}
		}
	}
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

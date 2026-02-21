// Command dashboard starts the FIPS compliance dashboard HTTP server.
// It serves the React frontend (embedded) and the compliance API.
//
// By default, the server binds to localhost only (127.0.0.1:8080) and is
// not exposed to the network. All frontend assets are bundled — no CDN
// dependencies at runtime (air-gap friendly).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/dashboard"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/cfapi"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/clientdetect"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost-only by default)")
	manifestPath := flag.String("manifest", "configs/build-manifest.json", "path to build manifest")
	staticDir := flag.String("static", "dashboard/dist", "path to static frontend files")
	configPath := flag.String("config", "", "path to cloudflared config file (for drift detection)")
	metricsAddr := flag.String("metrics-addr", "localhost:2000", "cloudflared metrics endpoint")
	ingressTargets := flag.String("ingress-targets", "", "comma-separated local service endpoints to probe (host:port)")

	// Cloudflare API settings (optional — enables live edge checks)
	cfToken := flag.String("cf-api-token", "", "Cloudflare API token (or set CF_API_TOKEN env)")
	cfZoneID := flag.String("cf-zone-id", "", "Cloudflare zone ID (or set CF_ZONE_ID env)")
	cfAccountID := flag.String("cf-account-id", "", "Cloudflare account ID (or set CF_ACCOUNT_ID env)")
	cfTunnelID := flag.String("cf-tunnel-id", "", "Cloudflare tunnel ID (or set CF_TUNNEL_ID env)")

	flag.Parse()

	fmt.Fprintf(os.Stderr, "%s\n", buildinfo.String())
	fmt.Fprintf(os.Stderr, "Starting dashboard server on %s\n", *addr)
	fmt.Fprintf(os.Stderr, "Dashboard is localhost-only by default. Use --addr 0.0.0.0:8080 to expose.\n")

	// Configure live compliance checker with real system queries
	var targets []string
	if *ingressTargets != "" {
		targets = strings.Split(*ingressTargets, ",")
	}

	liveChecker := compliance.NewLiveChecker(
		compliance.WithManifestPath(*manifestPath),
		compliance.WithConfigPath(*configPath),
		compliance.WithMetricsAddr(*metricsAddr),
		compliance.WithIngressTargets(targets),
	)

	// Build compliance sections from live checks
	checker := compliance.NewChecker()
	checker.AddSection(liveChecker.RunTunnelChecks())
	checker.AddSection(liveChecker.RunLocalServiceChecks())
	checker.AddSection(liveChecker.RunBuildSupplyChainChecks())

	// Cloudflare API integration (if token provided)
	token := envOrFlag(*cfToken, "CF_API_TOKEN")
	zoneID := envOrFlag(*cfZoneID, "CF_ZONE_ID")
	accountID := envOrFlag(*cfAccountID, "CF_ACCOUNT_ID")
	tunnelID := envOrFlag(*cfTunnelID, "CF_TUNNEL_ID")

	if token != "" && zoneID != "" {
		fmt.Fprintf(os.Stderr, "Cloudflare API integration enabled (zone: %s)\n", zoneID)
		cfClient := cfapi.NewClient(token)
		cfChecker := cfapi.NewComplianceChecker(cfClient, zoneID, accountID, tunnelID)
		checker.AddSection(cfChecker.RunEdgeChecks())
	} else {
		fmt.Fprintf(os.Stderr, "Cloudflare API integration disabled (set --cf-api-token and --cf-zone-id to enable)\n")
	}

	// Client FIPS detection
	inspector := clientdetect.NewInspector(1000)
	postureCollector := clientdetect.NewPostureCollector()

	// Add client posture section from TLS inspection + device reports
	clientChecker := clientdetect.NewComplianceChecker(inspector, postureCollector)
	checker.AddSection(clientChecker.RunClientPostureChecks())

	handler := dashboard.NewHandler(*manifestPath, checker)

	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, handler)

	mux.HandleFunc("GET /api/v1/clients", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"recent":  inspector.RecentClients(50),
			"summary": inspector.FIPSStats(),
		})
	})
	mux.HandleFunc("POST /api/v1/posture", postureCollector.HandlePostureReport)
	mux.HandleFunc("GET /api/v1/posture", postureCollector.HandlePostureList)

	// Serve the React frontend (all assets bundled, air-gap friendly)
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("/", fs)

	fmt.Fprintf(os.Stderr, "Client detection: TLS inspector active, posture API at /api/v1/posture\n")

	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

// envOrFlag returns the flag value if non-empty, otherwise the environment variable.
func envOrFlag(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}

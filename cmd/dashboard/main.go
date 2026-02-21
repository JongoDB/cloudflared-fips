// Command dashboard starts the FIPS compliance dashboard HTTP server.
// It serves the React frontend (embedded) and the compliance API.
//
// By default, the server binds to localhost only (127.0.0.1:8080) and is
// not exposed to the network. All frontend assets are bundled — no CDN
// dependencies at runtime (air-gap friendly).
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/dashboard"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost-only by default)")
	manifestPath := flag.String("manifest", "configs/build-manifest.json", "path to build manifest")
	staticDir := flag.String("static", "dashboard/dist", "path to static frontend files")
	configPath := flag.String("config", "", "path to cloudflared config file (for drift detection)")
	metricsAddr := flag.String("metrics-addr", "localhost:2000", "cloudflared metrics endpoint")
	ingressTargets := flag.String("ingress-targets", "", "comma-separated local service endpoints to probe (host:port)")
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

	handler := dashboard.NewHandler(*manifestPath, checker)

	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, handler)

	// Serve the React frontend (all assets bundled, air-gap friendly)
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("/", fs)

	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

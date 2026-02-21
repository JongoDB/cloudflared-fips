// Command dashboard starts the FIPS compliance dashboard HTTP server.
// It serves the React frontend (embedded) and the compliance API.
//
// By default, the server binds to localhost only (127.0.0.1:8080) and is
// not exposed to the network. All frontend assets are bundled — no CDN
// dependencies at runtime (air-gap friendly).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/dashboard"
	"github.com/cloudflared-fips/cloudflared-fips/internal/ipc"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/cfapi"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/clientdetect"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/deployment"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fipsbackend"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/signing"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost-only by default)")
	manifestPath := flag.String("manifest", "configs/build-manifest.json", "path to build manifest")
	staticDir := flag.String("static", "dashboard/dist", "path to static frontend files")
	configPath := flag.String("config", "", "path to cloudflared config file (for drift detection)")
	metricsAddr := flag.String("metrics-addr", "localhost:2000", "cloudflared metrics endpoint")
	ingressTargets := flag.String("ingress-targets", "", "comma-separated local service endpoints to probe (host:port)")

	// Deployment tier
	deployTier := flag.String("deployment-tier", "standard", "deployment tier: standard, regional_keyless, self_hosted")

	// Cloudflare API settings (optional — enables live edge checks)
	cfToken := flag.String("cf-api-token", "", "Cloudflare API token (or set CF_API_TOKEN env)")
	cfZoneID := flag.String("cf-zone-id", "", "Cloudflare zone ID (or set CF_ZONE_ID env)")
	cfAccountID := flag.String("cf-account-id", "", "Cloudflare account ID (or set CF_ACCOUNT_ID env)")
	cfTunnelID := flag.String("cf-tunnel-id", "", "Cloudflare tunnel ID (or set CF_TUNNEL_ID env)")

	// MDM integration (Intune/Jamf)
	mdmProvider := flag.String("mdm-provider", "", "MDM provider: intune, jamf (or set MDM_PROVIDER env)")
	mdmAPIToken := flag.String("mdm-api-token", "", "MDM API token (or set MDM_API_TOKEN env)")
	mdmBaseURL := flag.String("mdm-base-url", "", "MDM base URL — Jamf only (or set MDM_BASE_URL env)")
	mdmTenantID := flag.String("mdm-tenant-id", "", "Azure AD tenant ID — Intune only (or set MDM_TENANT_ID env)")
	mdmClientID := flag.String("mdm-client-id", "", "Azure AD client ID — Intune only (or set MDM_CLIENT_ID env)")
	mdmClientSecret := flag.String("mdm-client-secret", "", "Azure AD client secret — Intune only (or set MDM_CLIENT_SECRET env)")

	// IPC socket for CloudSH integration
	ipcSocket := flag.String("ipc-socket", "", "Unix socket path for CloudSH IPC (e.g., /var/run/cloudflared-fips/compliance.sock)")

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

	// FIPS backend info
	mux.HandleFunc("GET /api/v1/backend", func(w http.ResponseWriter, r *http.Request) {
		info := fipsbackend.DetectInfo()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})

	// Deployment tier info
	mux.HandleFunc("GET /api/v1/deployment", func(w http.ResponseWriter, r *http.Request) {
		tier := deployment.GetTier(*deployTier)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tier)
	})

	// FIPS 140-3 migration status
	mux.HandleFunc("GET /api/v1/migration", func(w http.ResponseWriter, r *http.Request) {
		status := fipsbackend.GetMigrationStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// All backend migration info
	mux.HandleFunc("GET /api/v1/migration/backends", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fipsbackend.AllBackendMigrationInfo())
	})

	// Signature manifest (if available)
	mux.HandleFunc("GET /api/v1/signatures", func(w http.ResponseWriter, r *http.Request) {
		sigPath := *manifestPath
		// Look for signatures.json next to build-manifest.json
		if idx := strings.LastIndex(sigPath, "/"); idx >= 0 {
			sigPath = sigPath[:idx] + "/signatures.json"
		} else {
			sigPath = "signatures.json"
		}
		manifest, err := signing.ReadSignatureManifest(sigPath)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status": "no signatures found",
				"error":  err.Error(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(manifest)
	})

	// MDM integration
	mdmProviderStr := envOrFlag(*mdmProvider, "MDM_PROVIDER")
	if mdmProviderStr != "" {
		mdmConfig := clientdetect.MDMConfig{
			Provider:     clientdetect.MDMProvider(mdmProviderStr),
			APIToken:     envOrFlag(*mdmAPIToken, "MDM_API_TOKEN"),
			BaseURL:      envOrFlag(*mdmBaseURL, "MDM_BASE_URL"),
			TenantID:     envOrFlag(*mdmTenantID, "MDM_TENANT_ID"),
			ClientID:     envOrFlag(*mdmClientID, "MDM_CLIENT_ID"),
			ClientSecret: envOrFlag(*mdmClientSecret, "MDM_CLIENT_SECRET"),
		}
		mdmClient := clientdetect.NewMDMClient(mdmConfig)
		if mdmClient.IsConfigured() {
			fmt.Fprintf(os.Stderr, "MDM integration enabled: %s\n", mdmProviderStr)
			mux.HandleFunc("GET /api/v1/mdm/devices", func(w http.ResponseWriter, r *http.Request) {
				devices, err := mdmClient.FetchDevices()
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadGateway)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(devices)
			})
			mux.HandleFunc("GET /api/v1/mdm/summary", func(w http.ResponseWriter, r *http.Request) {
				summary := mdmClient.ComplianceSummary()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(summary)
			})
		} else {
			fmt.Fprintf(os.Stderr, "MDM provider %s configured but missing credentials\n", mdmProviderStr)
		}
	}

	// Serve the React frontend (all assets bundled, air-gap friendly)
	fs := http.FileServer(http.Dir(*staticDir))
	mux.Handle("/", fs)

	fmt.Fprintf(os.Stderr, "Deployment tier: %s\n", *deployTier)
	fmt.Fprintf(os.Stderr, "Client detection: TLS inspector active, posture API at /api/v1/posture\n")

	// Start IPC socket server for CloudSH integration (if configured)
	if *ipcSocket != "" {
		ipcCtx, ipcCancel := context.WithCancel(context.Background())
		defer ipcCancel()
		ipcServer := ipc.NewServer(*ipcSocket, checker, *manifestPath)
		go func() {
			fmt.Fprintf(os.Stderr, "CloudSH IPC socket: %s\n", ipcServer.SocketPath())
			if err := ipcServer.Start(ipcCtx); err != nil {
				fmt.Fprintf(os.Stderr, "IPC server error: %v\n", err)
			}
		}()
	}

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

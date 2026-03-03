// Command dashboard starts the FIPS compliance dashboard HTTP server.
// It serves the React frontend and the compliance API.
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
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/dashboard"
	"github.com/cloudflared-fips/cloudflared-fips/internal/ipc"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/alerts"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/audit"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/cfapi"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/clientdetect"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/deployment"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fipsbackend"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/signing"
)

const shutdownTimeout = 10 * time.Second

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost-only by default)")
	manifestPath := flag.String("manifest", "configs/build-manifest.json", "path to build manifest")
	staticDir := flag.String("static", "dashboard/dist", "path to static frontend files")
	configPath := flag.String("config", "", "path to cloudflared config file (for drift detection)")
	metricsAddr := flag.String("metrics-addr", "localhost:2000", "cloudflared metrics endpoint")
	ingressTargets := flag.String("ingress-targets", "", "comma-separated local service endpoints to probe (host:port)")

	// Gateway proxy stats (Tier 3 / per-site FIPS gateway)
	proxyAddr := flag.String("proxy-addr", "", "fips-proxy metrics address for gateway client stats (e.g., localhost:8081)")

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

	// Tunnel setup (one-shot mode: create DNS CNAME + configure tunnel ingress, then exit)
	setupTunnel := flag.Bool("setup-tunnel", false, "one-shot: create DNS CNAME and configure tunnel ingress, then exit")
	teardownTunnel := flag.Bool("teardown-tunnel", false, "one-shot: delete DNS CNAME and tunnel, then exit")
	tunnelName := flag.String("tunnel-name", "cloudflared-fips", "name for auto-created tunnel (default: cloudflared-fips)")
	publicHostname := flag.String("public-hostname", "", "public hostname for tunnel ingress (e.g., dashboard.example.com)")
	hostnameService := flag.String("hostname-service", "http://localhost:8080", "backend service URL for tunnel ingress")

	// Fleet mode flags
	fleetMode := flag.Bool("fleet-mode", false, "enable fleet controller mode (registers nodes, stores reports)")
	dbPath := flag.String("db-path", "/var/lib/cloudflared-fips/fleet.db", "path to fleet SQLite database")
	adminAPIKey := flag.String("admin-api-key", "", "API key for fleet admin operations (or set FLEET_ADMIN_KEY env)")
	controllerURL := flag.String("controller-url", "", "URL of fleet controller (enables reporter mode)")
	nodeAPIKey := flag.String("node-api-key", "", "API key for this node's fleet authentication (or set NODE_API_KEY env)")
	nodeName := flag.String("node-name", "", "name for this node in fleet (defaults to hostname)")
	nodeRegion := flag.String("node-region", "", "region label for this node")
	nodeID := flag.String("node-id", "", "node ID from enrollment (or set NODE_ID env)")

	// Version flag
	showVersion := flag.Bool("version", false, "print version and exit")

	// Security Operations flags
	auditLogPath := flag.String("audit-log", "", "path to audit log file (enables AU-2 compliance; e.g., /var/log/cloudflared-fips/audit.json)")
	syslogAddr := flag.String("syslog-addr", "", "syslog address for audit log forwarding (e.g., tcp://siem:514)")
	dashboardToken := flag.String("dashboard-token", "", "Bearer token for dashboard API auth (or set DASHBOARD_TOKEN env)")
	alertWebhooks := flag.String("alert-webhook", "", "comma-separated webhook URLs for compliance alerts")
	tokenPathFlag := flag.String("token-path", "", "path to tunnel token file for expiry monitoring")
	certPathsFlag := flag.String("cert-paths", "", "comma-separated TLS certificate paths to monitor for expiry")
	secretsPathsFlag := flag.String("secrets-paths", "", "comma-separated directories to scan for secret file permissions")
	upstreamChecksum := flag.String("upstream-checksum", "", "expected SHA-256 hash of upstream cloudflared binary")
	enforcementMode := flag.String("enforcement-mode", "audit", "security policy enforcement mode: enforce, audit, disabled")

	flag.Parse()

	if *showVersion {
		fmt.Println(buildinfo.Version)
		return
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)

	// --setup-tunnel: one-shot mode — create DNS + configure ingress, then exit
	if *setupTunnel {
		runSetupTunnel(logger, envOrFlag(*cfToken, "CF_API_TOKEN"),
			envOrFlag(*cfZoneID, "CF_ZONE_ID"),
			envOrFlag(*cfAccountID, "CF_ACCOUNT_ID"),
			envOrFlag(*cfTunnelID, "CF_TUNNEL_ID"),
			*tunnelName, *publicHostname, *hostnameService)
		return
	}

	// --teardown-tunnel: one-shot mode — delete DNS records + tunnel, then exit
	if *teardownTunnel {
		runTeardownTunnel(logger, envOrFlag(*cfToken, "CF_API_TOKEN"),
			envOrFlag(*cfZoneID, "CF_ZONE_ID"),
			envOrFlag(*cfAccountID, "CF_ACCOUNT_ID"),
			envOrFlag(*cfTunnelID, "CF_TUNNEL_ID"))
		return
	}

	logger.Printf("%s", buildinfo.String())
	logger.Printf("Starting dashboard server on %s", *addr)
	logger.Printf("Dashboard is localhost-only by default. Use --addr 0.0.0.0:8080 to expose.")

	// Top-level context cancelled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Configure live compliance checker with real system queries
	var targets []string
	if *ingressTargets != "" {
		targets = strings.Split(*ingressTargets, ",")
	}

	// --- Audit Logger (AU-2, AU-3, AU-6) ---
	var auditLogger *audit.AuditLogger
	auditPath := envOrFlag(*auditLogPath, "AUDIT_LOG_PATH")
	if auditPath != "" {
		var auditOpts []audit.Option
		sysAddr := envOrFlag(*syslogAddr, "SYSLOG_ADDR")
		if sysAddr != "" {
			// Parse "tcp://host:port" or "udp://host:port"
			if u, err := url.Parse(sysAddr); err == nil {
				auditOpts = append(auditOpts, audit.WithSyslog(u.Scheme, u.Host))
				logger.Printf("Syslog forwarding: %s → %s", u.Scheme, u.Host)
			}
		}
		var err error
		auditLogger, err = audit.NewAuditLogger(auditPath, auditOpts...)
		if err != nil {
			logger.Fatalf("Failed to create audit logger: %v", err)
		}
		defer auditLogger.Close()
		logger.Printf("Audit logging enabled: %s", auditPath)
		auditLogger.Log(audit.AuditEvent{
			EventType: "system_event",
			Severity:  "info",
			Actor:     "system",
			Action:    "started",
			Detail:    fmt.Sprintf("Dashboard started: %s on %s", buildinfo.Version, *addr),
			NISTRef:   "AU-2",
		})
	}

	// --- Alert Manager (CA-7, SI-4) ---
	var alertManager *alerts.AlertManager
	webhookURLs := envOrFlag(*alertWebhooks, "ALERT_WEBHOOKS")
	if webhookURLs != "" {
		var webhooks []alerts.WebhookConfig
		for _, u := range strings.Split(webhookURLs, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				webhooks = append(webhooks, alerts.WebhookConfig{URL: u})
			}
		}
		alertManager = alerts.NewAlertManager(auditLogger, webhooks)
		logger.Printf("Alerting enabled: %d webhook(s)", len(webhooks))
	} else {
		alertManager = alerts.NewAlertManager(auditLogger, nil)
	}

	// --- Dashboard Auth Token ---
	dashToken := envOrFlag(*dashboardToken, "DASHBOARD_TOKEN")
	authEnabled := dashToken != ""
	if authEnabled {
		logger.Printf("Dashboard API authentication enabled")
	}

	// --- Parse Security Operations options ---
	var certPaths []string
	if cp := envOrFlag(*certPathsFlag, ""); cp != "" {
		certPaths = strings.Split(cp, ",")
	}
	var secretsPaths []string
	if sp := envOrFlag(*secretsPathsFlag, ""); sp != "" {
		secretsPaths = strings.Split(sp, ",")
	}

	liveChecker := compliance.NewLiveChecker(
		compliance.WithManifestPath(*manifestPath),
		compliance.WithConfigPath(*configPath),
		compliance.WithMetricsAddr(*metricsAddr),
		compliance.WithIngressTargets(targets),
		compliance.WithAuditLogger(auditLogger),
		compliance.WithAlertManager(alertManager),
		compliance.WithAuthEnabled(authEnabled),
		compliance.WithTokenPath(envOrFlag(*tokenPathFlag, "")),
		compliance.WithCertPaths(certPaths),
		compliance.WithSecretsPaths(secretsPaths),
		compliance.WithUpstreamChecksum(envOrFlag(*upstreamChecksum, "")),
		compliance.WithEnforcementMode(*enforcementMode),
	)

	// Build compliance sections from live checks
	checker := compliance.NewChecker()
	checker.AddSection(liveChecker.RunTunnelChecks())
	checker.AddSection(liveChecker.RunLocalServiceChecks())
	checker.AddSection(liveChecker.RunBuildSupplyChainChecks())
	checker.AddSection(liveChecker.RunSecurityOpsChecks())

	// Cloudflare API integration (if token provided)
	token := envOrFlag(*cfToken, "CF_API_TOKEN")
	zoneID := envOrFlag(*cfZoneID, "CF_ZONE_ID")
	accountID := envOrFlag(*cfAccountID, "CF_ACCOUNT_ID")
	tunnelID := envOrFlag(*cfTunnelID, "CF_TUNNEL_ID")

	if token != "" && zoneID != "" {
		logger.Printf("Cloudflare API integration enabled (zone: %s)", zoneID)
		cfClient := cfapi.NewClient(token)
		cfChecker := cfapi.NewComplianceChecker(cfClient, zoneID, accountID, tunnelID)
		checker.AddSection(cfChecker.RunEdgeChecks())
	} else {
		logger.Printf("Cloudflare API integration disabled (set --cf-api-token and --cf-zone-id to enable)")
	}

	// Client FIPS detection
	inspector := clientdetect.NewInspector(1000)
	postureCollector := clientdetect.NewPostureCollector()

	// Add client posture section from TLS inspection + device reports
	clientChecker := clientdetect.NewComplianceChecker(inspector, postureCollector)
	checker.AddSection(clientChecker.RunClientPostureChecks())

	// Gateway proxy stats (fetches client TLS inspection data from fips-proxy)
	if *proxyAddr != "" {
		logger.Printf("Gateway proxy stats enabled: fetching from %s", *proxyAddr)
		proxyChecker := compliance.NewProxyStatsChecker(*proxyAddr)
		checker.AddSection(proxyChecker.RunGatewayClientChecks())
	}

	handler := dashboard.NewHandler(*manifestPath, checker)
	handler.AuditLogger = auditLogger
	handler.AlertManager = alertManager

	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, handler)

	mux.HandleFunc("GET /api/v1/clients", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(info)
	})

	// Deployment tier info
	mux.HandleFunc("GET /api/v1/deployment", func(w http.ResponseWriter, r *http.Request) {
		tier := deployment.GetTier(*deployTier)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tier)
	})

	// FIPS 140-3 migration status
	mux.HandleFunc("GET /api/v1/migration", func(w http.ResponseWriter, r *http.Request) {
		status := fipsbackend.GetMigrationStatus()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	})

	// All backend migration info
	mux.HandleFunc("GET /api/v1/migration/backends", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fipsbackend.AllBackendMigrationInfo())
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
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "no signatures found",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
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
			logger.Printf("MDM integration enabled: %s", mdmProviderStr)
			mux.HandleFunc("GET /api/v1/mdm/devices", func(w http.ResponseWriter, r *http.Request) {
				devices, err := mdmClient.FetchDevices()
				if err != nil {
					logger.Printf("MDM device fetch error: %v", err)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadGateway)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "unable to fetch MDM devices"})
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(devices)
			})
			mux.HandleFunc("GET /api/v1/mdm/summary", func(w http.ResponseWriter, r *http.Request) {
				summary := mdmClient.ComplianceSummary()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(summary)
			})
		} else {
			logger.Printf("MDM provider %s configured but missing credentials", mdmProviderStr)
		}
	}

	// Fleet mode: controller accepts node registrations and compliance reports
	var fleetStore fleet.Store
	if *fleetMode {
		logger.Printf("Fleet mode enabled, database: %s", *dbPath)
		store, err := fleet.NewSQLiteStore(*dbPath)
		if err != nil {
			logger.Fatalf("Failed to open fleet database: %v", err)
		}
		fleetStore = store
		defer store.Close()

		adminKey := envOrFlag(*adminAPIKey, "FLEET_ADMIN_KEY")
		eventCh := make(chan fleet.FleetEvent, 256)

		fleetHandler := dashboard.NewFleetHandler(dashboard.FleetHandlerConfig{
			Store:    store,
			AdminKey: adminKey,
			Logger:   logger,
			EventCh:  eventCh,
		})
		dashboard.RegisterFleetRoutes(mux, fleetHandler)

		// Start fleet event broadcaster
		go fleetHandler.BroadcastEvents(ctx.Done())

		// Start stale-node monitor
		monitor := fleet.NewMonitor(fleet.MonitorConfig{
			Store:   store,
			Logger:  logger,
			EventCh: eventCh,
		})
		go monitor.Run(ctx)

		logger.Printf("Fleet controller ready: %d API endpoints registered", 12)
	}

	// Fleet reporter mode: push compliance reports to a controller
	ctrlURL := envOrFlag(*controllerURL, "CONTROLLER_URL")
	if ctrlURL != "" {
		nID := envOrFlag(*nodeID, "NODE_ID")
		nKey := envOrFlag(*nodeAPIKey, "NODE_API_KEY")
		if nID != "" && nKey != "" {
			reporter := fleet.NewReporter(fleet.ReporterConfig{
				ControllerURL: ctrlURL,
				NodeID:        nID,
				APIKey:        nKey,
				Checker:       checker,
				Logger:        logger,
			})
			go reporter.Run(ctx)
			logger.Printf("Fleet reporter active → %s (node: %s)", ctrlURL, nID)
		} else {
			logger.Printf("Fleet reporter disabled: --node-id and --node-api-key required")
		}
	}

	// Suppress unused variable warnings for fleet flags used via envOrFlag
	_ = nodeName
	_ = nodeRegion
	_ = fleetStore

	// Serve the React frontend — embedded by default (air-gap friendly),
	// falls back to filesystem directory for development.
	mux.Handle("/", dashboard.EmbeddedStaticHandler(*staticDir))

	logger.Printf("Deployment tier: %s", *deployTier)
	logger.Printf("Client detection: TLS inspector active, posture API at /api/v1/posture")

	// Start IPC socket server for CloudSH integration (if configured)
	if *ipcSocket != "" {
		ipcServer := ipc.NewServer(*ipcSocket, checker, *manifestPath)
		go func() {
			logger.Printf("CloudSH IPC socket: %s", ipcServer.SocketPath())
			if err := ipcServer.Start(ctx); err != nil {
				logger.Printf("IPC server error: %v", err)
			}
		}()
	}

	// Wrap mux with auth middleware (AC-2, AC-3, AC-7)
	authMiddleware := dashboard.NewAuthMiddleware(dashboard.AuthConfig{
		Token:    dashToken,
		AuditLog: auditLogger,
	}, mux)

	// Configure HTTP server with timeouts
	server := &http.Server{
		Addr:              *addr,
		Handler:           dashboard.SecurityHeaders(authMiddleware),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second, // SSE needs longer write timeout
		IdleTimeout:       120 * time.Second,
	}

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	logger.Printf("Server ready on %s", *addr)

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		logger.Printf("Shutdown signal received, draining connections...")
	case err := <-errCh:
		logger.Printf("Server error: %v", err)
		os.Exit(1)
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("Shutdown error: %v", err)
		os.Exit(1)
	}

	logger.Printf("Server stopped gracefully")
}

// envOrFlag returns the flag value if non-empty, otherwise the environment variable.
func envOrFlag(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}

// runSetupTunnel is a one-shot mode that creates a Cloudflare Tunnel (if needed),
// sets up a DNS CNAME record, and configures tunnel ingress, then exits.
// Called by the provision script. When no tunnel ID is provided, it auto-creates
// one via the Cloudflare API and prints the tunnel ID and token to stdout
// (prefixed with TUNNEL_ID= and TUNNEL_TOKEN=) so the provision script can
// capture them.
func runSetupTunnel(logger *log.Logger, token, zoneID, accountID, tunnelID, name, hostname, service string) {
	logger.Printf("Setup tunnel: hostname=%s service=%s", hostname, service)

	if token == "" {
		logger.Printf("No API token provided — skipping tunnel setup")
		return
	}
	if accountID == "" {
		logger.Printf("No account ID provided — skipping tunnel setup")
		return
	}

	client := cfapi.NewClient(token)
	tunnelToken := ""

	// Step 1: Create tunnel if no tunnel ID provided
	if tunnelID == "" {
		tunnelName := name
		if tunnelName == "" {
			tunnelName = "cloudflared-fips"
		}
		logger.Printf("No tunnel ID provided — creating tunnel '%s'...", tunnelName)

		// Check for existing tunnel with same name first
		existing, err := client.ListTunnels(accountID)
		if err == nil {
			for _, t := range existing {
				if t.Name == tunnelName {
					logger.Printf("Found existing tunnel: %s (ID: %s)", t.Name, t.ID)
					tunnelID = t.ID
					// Get token for existing tunnel
					tok, err := client.GetTunnelToken(accountID, t.ID)
					if err != nil {
						logger.Printf("Warning: could not get token for existing tunnel: %v", err)
					} else {
						tunnelToken = tok
					}
					break
				}
			}
		}

		// Create new tunnel if none found
		if tunnelID == "" {
			result, err := client.CreateTunnel(accountID, tunnelName)
			if err != nil {
				logger.Fatalf("Failed to create tunnel: %v", err)
			}
			tunnelID = result.ID
			tunnelToken = result.Token
			logger.Printf("Tunnel created: %s (ID: %s)", tunnelName, tunnelID)
		}

		// Print machine-readable output for provision script to capture
		fmt.Printf("TUNNEL_ID=%s\n", tunnelID)
		fmt.Printf("TUNNEL_TOKEN=%s\n", tunnelToken)
	}

	// Step 2: Create DNS CNAME record (if hostname provided)
	if hostname != "" && zoneID != "" && tunnelID != "" {
		logger.Printf("Creating DNS CNAME: %s → %s.cfargotunnel.com", hostname, tunnelID)
		_, err := client.CreateDNSCNAME(zoneID, hostname, tunnelID)
		if err != nil {
			// Record may already exist — log but don't fail
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "81053") {
				logger.Printf("DNS record already exists for %s (OK)", hostname)
			} else {
				logger.Printf("Warning: failed to create DNS CNAME: %v", err)
			}
		} else {
			logger.Printf("DNS CNAME created: %s", hostname)
		}
	}

	// Step 3: Configure tunnel ingress (hostname → service mapping)
	if accountID != "" && tunnelID != "" && hostname != "" && service != "" {
		logger.Printf("Configuring tunnel ingress: %s → %s", hostname, service)
		ingress := []cfapi.TunnelIngressRule{
			{Hostname: hostname, Service: service},
			{Service: "http_status:404"},
		}
		if err := client.ConfigureTunnelIngress(accountID, tunnelID, ingress); err != nil {
			logger.Printf("Warning: failed to configure tunnel ingress: %v", err)
		} else {
			logger.Printf("Tunnel ingress configured successfully")
		}
	}

	logger.Printf("Tunnel setup complete")
}

// runTeardownTunnel is a one-shot mode that deletes DNS CNAME records and
// the tunnel itself, then exits. Counterpart to runSetupTunnel. Called by
// the unprovision script to clean up Cloudflare resources.
func runTeardownTunnel(logger *log.Logger, token, zoneID, accountID, tunnelID string) {
	logger.Printf("Teardown tunnel: account=%s tunnel=%s", accountID, tunnelID)

	if token == "" {
		logger.Fatalf("No API token provided (--cf-api-token or CF_API_TOKEN required)")
	}
	if accountID == "" || tunnelID == "" {
		logger.Fatalf("--cf-account-id and --cf-tunnel-id are required for teardown")
	}

	client := cfapi.NewClient(token)
	exitCode := 0

	// Step 1: Find and delete DNS CNAME records pointing to this tunnel
	if zoneID != "" {
		logger.Printf("Looking for DNS records pointing to %s.cfargotunnel.com...", tunnelID)
		records, err := client.FindDNSRecord(zoneID, "", "CNAME")
		if err != nil {
			logger.Printf("Warning: failed to query DNS records: %v", err)
		} else {
			tunnelTarget := tunnelID + ".cfargotunnel.com"
			deleted := 0
			for _, r := range records {
				if r.Content == tunnelTarget {
					logger.Printf("Deleting DNS CNAME: %s → %s (ID: %s)", r.Name, r.Content, r.ID)
					if err := client.DeleteDNSRecord(zoneID, r.ID); err != nil {
						logger.Printf("Warning: failed to delete DNS record %s: %v", r.ID, err)
						exitCode = 1
					} else {
						deleted++
					}
				}
			}
			if deleted > 0 {
				logger.Printf("Deleted %d DNS record(s)", deleted)
			} else {
				logger.Printf("No DNS records found pointing to this tunnel")
			}
		}
	}

	// Step 2: Delete the tunnel
	logger.Printf("Deleting tunnel: %s", tunnelID)
	if err := client.DeleteTunnel(accountID, tunnelID); err != nil {
		logger.Printf("Error: failed to delete tunnel: %v", err)
		exitCode = 1
	} else {
		logger.Printf("Tunnel deleted successfully")
	}

	if exitCode != 0 {
		logger.Printf("Teardown completed with errors")
		os.Exit(exitCode)
	}
	logger.Printf("Tunnel teardown complete")
}

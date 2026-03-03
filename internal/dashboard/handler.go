// Package dashboard provides HTTP API handlers for the compliance dashboard.
//
// The dashboard is localhost-only by default and serves bundled frontend assets
// for air-gap friendly deployment. All compliance data is available as structured
// JSON via the API so CloudSH can embed or aggregate it.
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/alerts"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/audit"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/manifest"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	ManifestPath string
	Checker      *compliance.Checker
	AuditLogger  *audit.AuditLogger
	AlertManager *alerts.AlertManager
}

// NewHandler creates a new dashboard handler.
func NewHandler(manifestPath string, checker *compliance.Checker) *Handler {
	return &Handler{
		ManifestPath: manifestPath,
		Checker:      checker,
	}
}

// HandleCompliance returns the compliance report as JSON.
func (h *Handler) HandleCompliance(w http.ResponseWriter, r *http.Request) {
	report := h.Checker.GenerateReport()
	writeJSON(w, http.StatusOK, report)
}

// HandleManifest returns the build manifest as JSON.
func (h *Handler) HandleManifest(w http.ResponseWriter, r *http.Request) {
	m, err := manifest.ReadManifest(h.ManifestPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "unable to load build manifest",
		})
		return
	}
	// Override stale manifest fields with runtime build info injected via ldflags.
	if buildinfo.Version != "" && buildinfo.Version != "dev" {
		m.Version = strings.TrimPrefix(buildinfo.Version, "v")
	}
	if buildinfo.GitCommit != "" && buildinfo.GitCommit != "unknown" {
		m.Commit = buildinfo.GitCommit
	}
	if buildinfo.BuildDate != "" && buildinfo.BuildDate != "unknown" {
		m.BuildTime = buildinfo.BuildDate
	}

	// Detect upstream cloudflared version at runtime if manifest has placeholder.
	if m.CloudflaredUpstreamVersion == "" || m.CloudflaredUpstreamVersion == "unknown" || m.CloudflaredUpstreamVersion == "0.0.0" {
		if ver := detectCloudflaredVersion(); ver != "" {
			m.CloudflaredUpstreamVersion = ver
		}
	}

	writeJSON(w, http.StatusOK, m)
}

// HandleSelfTest runs the self-test suite and returns the report.
func (h *Handler) HandleSelfTest(w http.ResponseWriter, r *http.Request) {
	report, _ := selftest.GenerateReport(buildinfo.Version)
	writeJSON(w, http.StatusOK, report)
}

// HandleHealth returns a simple health check response.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": buildinfo.Version,
	})
}

// HandleExport returns the compliance report in the requested format.
// Supports format=json (default) and format=pdf (requires pandoc on the host).
func (h *Handler) HandleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	report := h.Checker.GenerateReport()

	switch format {
	case "json":
		w.Header().Set("Content-Disposition", "attachment; filename=compliance-report.json")
		writeJSON(w, http.StatusOK, report)
	case "pdf":
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error":   "PDF export requires pandoc on the host",
			"install": "dnf install -y pandoc (RHEL) or brew install pandoc (macOS)",
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "unsupported format",
			"formats": "json, pdf",
		})
	}
}

// HandleSSE provides a Server-Sent Events stream for real-time compliance updates.
// The dashboard frontend connects to this endpoint to receive live updates
// without polling. Properly handles client disconnects via request context.
func (h *Handler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Send initial compliance state
	if err := writeSSEEvent(w, flusher, "compliance", h.Checker.GenerateReport()); err != nil {
		return // Client disconnected
	}

	// Send periodic updates (every 30 seconds)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := writeSSEEvent(w, flusher, "compliance", h.Checker.GenerateReport()); err != nil {
				return // Client disconnected or write error
			}
		}
	}
}

// writeSSEEvent marshals data and writes it as an SSE event. Returns an error
// if the write fails (e.g., client disconnected).
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) error {
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, buf); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// SecurityHeaders wraps an http.Handler with standard security response headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// detectCloudflaredVersion runs "cloudflared version" and extracts the version string.
// Returns empty string if cloudflared is not installed or the command fails.
func detectCloudflaredVersion() string {
	out, err := exec.Command("cloudflared", "version").CombinedOutput()
	if err != nil {
		return ""
	}
	// Output format: "cloudflared version 2025.2.1 (built 2025-02-18-...)"
	for _, field := range strings.Fields(string(out)) {
		// Find a field that looks like a version number (digits and dots)
		if len(field) > 0 && field[0] >= '0' && field[0] <= '9' && strings.Contains(field, ".") {
			return field
		}
	}
	return ""
}

// HandleAuditEvents returns recent audit events from the ring buffer.
func (h *Handler) HandleAuditEvents(w http.ResponseWriter, r *http.Request) {
	if h.AuditLogger == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events": []interface{}{},
			"status": "audit logging not configured",
		})
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	events := h.AuditLogger.RecentEvents(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// HandleAuditSSE provides SSE stream of audit events.
func (h *Handler) HandleAuditSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if h.AuditLogger == nil {
		writeSSEEvent(w, flusher, "error", map[string]string{"error": "audit logging not configured"})
		return
	}

	// Send recent events first
	events := h.AuditLogger.RecentEvents(50)
	writeSSEEvent(w, flusher, "audit_batch", events)

	// Register listener for new events
	ch := make(chan audit.AuditEvent, 64)
	h.AuditLogger.AddListener(func(evt audit.AuditEvent) {
		select {
		case ch <- evt:
		default:
			// Drop if channel full
		}
	})

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-ch:
			if err := writeSSEEvent(w, flusher, "audit", evt); err != nil {
				return
			}
		}
	}
}

// HandleCredentialsStatus returns token and certificate expiry information.
func (h *Handler) HandleCredentialsStatus(w http.ResponseWriter, r *http.Request) {
	report := h.Checker.GenerateReport()

	type credStatus struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}

	var creds []credStatus
	for _, section := range report.Sections {
		for _, item := range section.Items {
			// Include credential-related checks
			if item.ID == "so-6" || item.ID == "so-7" || item.ID == "so-8" ||
				item.ID == "t-8" || item.ID == "l-3" {
				creds = append(creds, credStatus{
					ID:     item.ID,
					Name:   item.Name,
					Status: string(item.Status),
					Detail: item.What,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"credentials": creds,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleAlertTest sends a test alert to all configured webhooks.
func (h *Handler) HandleAlertTest(w http.ResponseWriter, r *http.Request) {
	if h.AlertManager == nil || !h.AlertManager.Configured() {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no webhooks configured (use --alert-webhook)",
		})
		return
	}

	results := h.AlertManager.TestWebhooks()
	response := make(map[string]string)
	allOK := true
	for url, err := range results {
		if err != nil {
			response[url] = err.Error()
			allOK = false
		} else {
			response[url] = "ok"
		}
	}

	status := http.StatusOK
	if !allOK {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]interface{}{
		"results": response,
		"success": allOK,
	})
}

// ComplianceInfoData holds static AO compliance posture information.
type ComplianceInfoData struct {
	ProductName      string         `json:"product_name"`
	ProductVersion   string         `json:"product_version"`
	Description      string         `json:"description"`
	Segments         []SegmentInfo  `json:"segments"`
	CryptoModules    []CryptoModule `json:"crypto_modules"`
	ControlsCovered  []ControlInfo  `json:"controls_covered"`
	VerificationInfo []VerifyInfo   `json:"verification_methods"`
	KnownGaps        []GapInfo      `json:"known_gaps"`
	DeploymentTiers  []TierInfo     `json:"deployment_tiers"`
	MigrationInfo    MigrationInfo  `json:"migration"`
	Documents        []DocInfo      `json:"documents"`
}

// SegmentInfo describes one of the three architecture segments.
type SegmentInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	FIPSControl string `json:"fips_control"`
}

// CryptoModule describes a FIPS cryptographic module.
type CryptoModule struct {
	Name        string `json:"name"`
	CMVPCert    string `json:"cmvp_cert"`
	Standard    string `json:"standard"`
	Platform    string `json:"platform"`
	BuildFlag   string `json:"build_flag"`
	Description string `json:"description"`
}

// ControlInfo maps a NIST control to implementation.
type ControlInfo struct {
	ControlID      string `json:"control_id"`
	ControlName    string `json:"control_name"`
	Implementation string `json:"implementation"`
	Status         string `json:"status"`
}

// VerifyInfo describes a verification method.
type VerifyInfo struct {
	Method      string `json:"method"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// GapInfo describes a known compliance gap.
type GapInfo struct {
	Gap        string `json:"gap"`
	Impact     string `json:"impact"`
	Mitigation string `json:"mitigation"`
}

// TierInfo describes a deployment tier.
type TierInfo struct {
	Tier        string `json:"tier"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MigrationInfo describes the FIPS 140-2 to 140-3 migration.
type MigrationInfo struct {
	SunsetDate string `json:"sunset_date"`
	Status     string `json:"status"`
	Plan       string `json:"plan"`
}

// DocInfo describes an AO document.
type DocInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// HandleComplianceInfo returns static AO compliance posture data.
func (h *Handler) HandleComplianceInfo(w http.ResponseWriter, r *http.Request) {
	info := ComplianceInfoData{
		ProductName:    "cloudflared-fips",
		ProductVersion: buildinfo.Version,
		Description:    "FIPS 140-3 compliant build of Cloudflare Tunnel (cloudflared) with real-time compliance dashboard and AO authorization documentation toolkit.",
		Segments: []SegmentInfo{
			{Name: "Segment 1: Client to Edge", Description: "Client OS/Browser to Cloudflare Edge (HTTPS). Controlled by client OS FIPS mode and browser TLS stack.", FIPSControl: "Client-side: Windows CNG, macOS CommonCrypto, Linux OpenSSL with FIPS mode"},
			{Name: "Segment 2: Edge to Tunnel", Description: "Cloudflare Edge to cloudflared tunnel daemon. This is the segment this product directly controls.", FIPSControl: "BoringCrypto (CMVP #4735, FIPS 140-3) or Go native FIPS module"},
			{Name: "Segment 3: Tunnel to Origin", Description: "cloudflared to local origin service. Loopback may be out of FIPS scope per AO interpretation.", FIPSControl: "TLS with FIPS-approved ciphers for networked segments; loopback exempt"},
		},
		CryptoModules: []CryptoModule{
			{Name: "BoringCrypto", CMVPCert: "#4735", Standard: "FIPS 140-3", Platform: "Linux amd64/arm64", BuildFlag: "GOEXPERIMENT=boringcrypto", Description: "Google's BoringSSL FIPS module, statically linked into Go binary"},
			{Name: "Go Native FIPS", CMVPCert: "Pending (CAVP A6650)", Standard: "FIPS 140-3", Platform: "All (Linux, macOS, Windows)", BuildFlag: "GODEBUG=fips140=on", Description: "Go 1.24+ native FIPS 140-3 crypto module (CMVP validation in progress)"},
			{Name: "RHEL OpenSSL", CMVPCert: "#4746, #4857", Standard: "FIPS 140-3", Platform: "RHEL 9", BuildFlag: "OS fips=1 boot param", Description: "Host OS validated OpenSSL module (system-level FIPS mode)"},
		},
		ControlsCovered: []ControlInfo{
			{ControlID: "SC-13", ControlName: "Cryptographic Protection", Implementation: "BoringCrypto/Go FIPS module for all TLS operations; cipher suite restriction", Status: "implemented"},
			{ControlID: "SC-8", ControlName: "Transmission Confidentiality", Implementation: "TLS 1.2+ on all segments; FIPS-approved cipher suites only", Status: "implemented"},
			{ControlID: "SC-12", ControlName: "Cryptographic Key Establishment", Implementation: "ECDHE with NIST P-256/P-384 curves; key rotation procedures documented", Status: "implemented"},
			{ControlID: "SC-28", ControlName: "Protection of Information at Rest", Implementation: "Secret file permission checks; encrypted token storage", Status: "implemented"},
			{ControlID: "IA-7", ControlName: "Cryptographic Module Authentication", Implementation: "FIPS self-test (KATs) at startup; binary integrity verification", Status: "implemented"},
			{ControlID: "IA-5", ControlName: "Authenticator Management", Implementation: "Token expiry monitoring; certificate lifecycle management", Status: "implemented"},
			{ControlID: "AC-2", ControlName: "Account Management", Implementation: "Dashboard Bearer token authentication; fleet enrollment tokens", Status: "implemented"},
			{ControlID: "AC-3", ControlName: "Access Enforcement", Implementation: "API authentication middleware; role-based fleet access", Status: "implemented"},
			{ControlID: "AC-7", ControlName: "Unsuccessful Logon Attempts", Implementation: "IP-based lockout after 5 failures in 5 minutes", Status: "implemented"},
			{ControlID: "AU-2", ControlName: "Event Logging", Implementation: "JSON-lines audit log with 7 event types", Status: "implemented"},
			{ControlID: "AU-3", ControlName: "Content of Audit Records", Implementation: "Timestamp, event type, severity, actor, resource, action, detail, NIST reference", Status: "implemented"},
			{ControlID: "AU-4", ControlName: "Audit Log Storage Capacity", Implementation: "Syslog forwarding to external SIEM; in-memory ring buffer", Status: "implemented"},
			{ControlID: "AU-9", ControlName: "Protection of Audit Information", Implementation: "File permissions check (0640); syslog forwarding for tamper resistance", Status: "implemented"},
			{ControlID: "CA-7", ControlName: "Continuous Monitoring", Implementation: "Real-time SSE dashboard updates every 30s; webhook alerts", Status: "implemented"},
			{ControlID: "SI-4", ControlName: "System Monitoring", Implementation: "Automated alerting on compliance changes; failed auth monitoring", Status: "implemented"},
			{ControlID: "SI-7", ControlName: "Software Integrity", Implementation: "Binary SHA-256 verification; upstream cloudflared integrity check; SBOM", Status: "implemented"},
			{ControlID: "CM-6", ControlName: "Configuration Settings", Implementation: "Config drift detection; OS FIPS mode verification", Status: "implemented"},
			{ControlID: "CM-14", ControlName: "Signed Components", Implementation: "GPG and cosign artifact signing; signature verification at startup", Status: "implemented"},
			{ControlID: "SA-11", ControlName: "Developer Testing", Implementation: "CI/CD with compliance checks; FIPS self-test in CI", Status: "implemented"},
			{ControlID: "PL-1", ControlName: "Security Planning Policy", Implementation: "Configurable enforcement mode (enforce/audit/disabled)", Status: "implemented"},
		},
		VerificationInfo: []VerifyInfo{
			{Method: "direct", Description: "Measured locally by the dashboard binary", Example: "FIPS self-test results, binary hash, OS FIPS mode"},
			{Method: "api", Description: "Queried from Cloudflare API", Example: "Access policy status, cipher config, TLS version"},
			{Method: "probe", Description: "Verified via TLS handshake inspection", Example: "Negotiated cipher suite, TLS version, certificate validity"},
			{Method: "inherited", Description: "Relies on provider's FedRAMP authorization", Example: "Cloudflare edge crypto module FIPS validation"},
			{Method: "reported", Description: "Client-reported via WARP/device posture agent", Example: "Client OS FIPS mode, MDM enrollment status"},
		},
		KnownGaps: []GapInfo{
			{Gap: "Cloudflare edge crypto module not independently FIPS-validated", Impact: "Edge TLS termination relies on Cloudflare's FedRAMP Moderate authorization", Mitigation: "Use Tier 2 (Keyless SSL + HSM) or Tier 3 (self-hosted FIPS proxy) for full control"},
			{Gap: "Chrome on Linux uses BoringSSL regardless of OS FIPS mode", Impact: "Client-to-edge TLS may not use OS FIPS-validated crypto", Mitigation: "Recommend Firefox with NSS FIPS mode on Linux; document in client hardening guide"},
			{Gap: "Go native FIPS 140-3 CMVP validation still pending", Impact: "Cross-platform FIPS (macOS/Windows) uses unvalidated module", Mitigation: "Use BoringCrypto on Linux; track CMVP progress; GoNativeCMVPValidated flag for single-flip update"},
			{Gap: "QUIC retry uses fixed AES-GCM nonce per RFC 9001", Impact: "Protocol-level FIPS deviation (documented, not a crypto weakness)", Mitigation: "Documented in quic-go audit; acceptable under GOEXPERIMENT=boringcrypto mode"},
		},
		DeploymentTiers: []TierInfo{
			{Tier: "1", Name: "Standard Cloudflare Tunnel", Description: "Client -> Cloudflare Edge -> cloudflared-fips -> Origin. Edge crypto inherited through FedRAMP."},
			{Tier: "2", Name: "Regional Services + Keyless SSL", Description: "Client -> FedRAMP DC (Regional Services) -> Keyless SSL (customer HSM) -> cloudflared-fips -> Origin. Private keys in customer FIPS 140-2 Level 3 HSMs."},
			{Tier: "3", Name: "Self-Hosted FIPS Edge Proxy", Description: "Client -> FIPS Proxy (GovCloud) -> [optional: Cloudflare for WAF/DDoS] -> cloudflared-fips -> Origin. Full TLS control."},
		},
		MigrationInfo: MigrationInfo{
			SunsetDate: "2026-09-21",
			Status:     "BoringCrypto #4735 (FIPS 140-3) available now with Go 1.24+. Go native FIPS CMVP pending.",
			Plan:       "Primary: BoringCrypto 140-3 on Linux. Fallback: Go native FIPS once CMVP validates. Microsoft systemcrypto for Windows/macOS.",
		},
		Documents: []DocInfo{
			{Name: "System Security Plan (SSP)", Path: "docs/ao-narrative.md", Description: "Module boundary definition, validated modules, crypto operations mapping"},
			{Name: "Cryptographic Module Usage", Path: "docs/crypto-module-usage.md", Description: "Operation -> Algorithm -> Module -> CMVP Certificate"},
			{Name: "FIPS Justification Letter", Path: "docs/fips-justification-letter.md", Description: "AO argument for leveraged validated module approach"},
			{Name: "Client Hardening Guide", Path: "docs/client-hardening-guide.md", Description: "Windows/RHEL/macOS FIPS mode + MDM policy templates"},
			{Name: "Continuous Monitoring Plan", Path: "docs/continuous-monitoring-plan.md", Description: "Dashboard as continuous monitoring tool"},
			{Name: "Incident Response Addendum", Path: "docs/incident-response-addendum.md", Description: "Crypto failure and compliance breach procedures"},
			{Name: "NIST 800-53 Control Mapping", Path: "docs/control-mapping.md", Description: "Control -> Implementation -> Evidence mapping"},
			{Name: "Architecture Diagram", Path: "docs/architecture-diagram.md", Description: "Three-segment architecture with Mermaid diagrams"},
			{Name: "Key Rotation Procedure", Path: "docs/key-rotation-procedure.md", Description: "GPG/cosign key rotation and re-signing"},
			{Name: "Deployment Tier Guide", Path: "docs/deployment-tier-guide.md", Description: "Tier 1/2/3 setup with HSM guides for 8 vendors"},
			{Name: "quic-go Crypto Audit", Path: "docs/quic-go-crypto-audit.md", Description: "FIPS crypto analysis of quic-go library"},
			{Name: "Compliance Checks Reference", Path: "docs/compliance-checks-reference.md", Description: "All 54 compliance checks with NIST mappings"},
		},
	}

	writeJSON(w, http.StatusOK, info)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

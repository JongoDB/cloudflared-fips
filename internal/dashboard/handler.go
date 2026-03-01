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
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/manifest"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	ManifestPath string
	Checker      *compliance.Checker
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

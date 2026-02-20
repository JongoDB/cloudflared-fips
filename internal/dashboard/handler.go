// Package dashboard provides HTTP API handlers for the compliance dashboard.
package dashboard

import (
	"encoding/json"
	"net/http"

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
			"error": "failed to read manifest: " + err.Error(),
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

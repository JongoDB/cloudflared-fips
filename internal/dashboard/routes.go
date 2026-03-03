package dashboard

import "net/http"

// RegisterRoutes sets up the API routes on the given ServeMux.
// All API paths are prefixed with /api/v1/ for CloudSH integration namespacing.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("GET /api/v1/compliance", h.HandleCompliance)
	mux.HandleFunc("GET /api/v1/manifest", h.HandleManifest)
	mux.HandleFunc("GET /api/v1/selftest", h.HandleSelfTest)
	mux.HandleFunc("GET /api/v1/health", h.HandleHealth)
	mux.HandleFunc("GET /api/v1/compliance/export", h.HandleExport)
	mux.HandleFunc("GET /api/v1/events", h.HandleSSE)

	// Audit and Security Operations endpoints
	mux.HandleFunc("GET /api/v1/audit/events", h.HandleAuditEvents)
	mux.HandleFunc("GET /api/v1/audit/events/stream", h.HandleAuditSSE)
	mux.HandleFunc("GET /api/v1/credentials/status", h.HandleCredentialsStatus)
	mux.HandleFunc("POST /api/v1/alerts/test", h.HandleAlertTest)
	mux.HandleFunc("GET /api/v1/compliance-info", h.HandleComplianceInfo)
}

// RegisterRoutesWithWS sets up the API routes including WebSocket support.
func RegisterRoutesWithWS(mux *http.ServeMux, h *Handler, hub *WSHub) {
	RegisterRoutes(mux, h)
	mux.HandleFunc("GET /api/v1/ws", hub.HandleWS)
}

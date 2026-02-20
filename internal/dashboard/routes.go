package dashboard

import "net/http"

// RegisterRoutes sets up the API routes on the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("GET /api/v1/compliance", h.HandleCompliance)
	mux.HandleFunc("GET /api/v1/manifest", h.HandleManifest)
	mux.HandleFunc("GET /api/v1/selftest", h.HandleSelfTest)
	mux.HandleFunc("GET /api/v1/health", h.HandleHealth)
}

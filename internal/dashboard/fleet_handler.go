package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet"
)

// FleetHandler serves the fleet management API endpoints.
type FleetHandler struct {
	store      fleet.Store
	enrollment *fleet.Enrollment
	adminKey   string
	logger     *log.Logger
	eventCh    chan fleet.FleetEvent
	sseClients map[chan fleet.FleetEvent]struct{}
	sseMu      sync.Mutex
}

// FleetHandlerConfig holds configuration for the fleet handler.
type FleetHandlerConfig struct {
	Store    fleet.Store
	AdminKey string // API key for admin operations (token management, node deletion)
	Logger   *log.Logger
	EventCh  chan fleet.FleetEvent
}

// NewFleetHandler creates a new fleet handler.
func NewFleetHandler(cfg FleetHandlerConfig) *FleetHandler {
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &FleetHandler{
		store:      cfg.Store,
		enrollment: fleet.NewEnrollment(cfg.Store),
		adminKey:   cfg.AdminKey,
		logger:     cfg.Logger,
		eventCh:    cfg.EventCh,
		sseClients: make(map[chan fleet.FleetEvent]struct{}),
	}
}

// RegisterFleetRoutes registers all fleet API endpoints on the given mux.
func RegisterFleetRoutes(mux *http.ServeMux, fh *FleetHandler) {
	mux.HandleFunc("POST /api/v1/fleet/tokens", fh.HandleCreateToken)
	mux.HandleFunc("GET /api/v1/fleet/tokens", fh.HandleListTokens)
	mux.HandleFunc("DELETE /api/v1/fleet/tokens/{id}", fh.HandleDeleteToken)
	mux.HandleFunc("POST /api/v1/fleet/enroll", fh.HandleEnroll)
	mux.HandleFunc("POST /api/v1/fleet/report", fh.HandleReport)
	mux.HandleFunc("POST /api/v1/fleet/heartbeat", fh.HandleHeartbeat)
	mux.HandleFunc("GET /api/v1/fleet/nodes", fh.HandleListNodes)
	mux.HandleFunc("GET /api/v1/fleet/nodes/{id}", fh.HandleGetNode)
	mux.HandleFunc("DELETE /api/v1/fleet/nodes/{id}", fh.HandleDeleteNode)
	mux.HandleFunc("GET /api/v1/fleet/summary", fh.HandleSummary)
	mux.HandleFunc("GET /api/v1/fleet/events", fh.HandleFleetSSE)
	mux.HandleFunc("GET /api/v1/fleet/nodes/{id}/report", fh.HandleGetNodeReport)
}

// BroadcastEvents fans out fleet events from the event channel to all SSE clients.
// Should be run as a goroutine.
func (fh *FleetHandler) BroadcastEvents(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case evt, ok := <-fh.eventCh:
			if !ok {
				return
			}
			fh.sseMu.Lock()
			for ch := range fh.sseClients {
				select {
				case ch <- evt:
				default:
				}
			}
			fh.sseMu.Unlock()
		}
	}
}

// HandleCreateToken creates a new enrollment token (admin only).
func (fh *FleetHandler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	var req fleet.CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	token, err := fh.enrollment.CreateToken(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, token)
}

// HandleListTokens returns active enrollment tokens (admin only).
func (fh *FleetHandler) HandleListTokens(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	tokens, err := fh.enrollment.ListTokens(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tokens"})
		return
	}
	if tokens == nil {
		tokens = []fleet.EnrollmentToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

// HandleDeleteToken removes an enrollment token (admin only).
func (fh *FleetHandler) HandleDeleteToken(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token id required"})
		return
	}

	if err := fh.enrollment.DeleteToken(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleEnroll processes a node enrollment request (token-based auth).
func (fh *FleetHandler) HandleEnroll(w http.ResponseWriter, r *http.Request) {
	var req fleet.EnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := fh.enrollment.Enroll(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	fh.logger.Printf("fleet: node enrolled: %s (name=%s)", resp.NodeID, req.Name)

	// Emit event
	node, _ := fh.store.GetNode(r.Context(), resp.NodeID)
	if node != nil && fh.eventCh != nil {
		select {
		case fh.eventCh <- fleet.FleetEvent{
			Type: "node_joined",
			Node: *node,
			Time: time.Now().UTC(),
		}:
		default:
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleReport receives a compliance report from a node (API key auth).
func (fh *FleetHandler) HandleReport(w http.ResponseWriter, r *http.Request) {
	node, ok := fh.authenticateNode(w, r)
	if !ok {
		return
	}

	var payload fleet.ComplianceReportPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid report"})
		return
	}

	// Verify node ID matches authenticated node
	if payload.NodeID != node.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "node id mismatch"})
		return
	}

	// Store report
	reportJSON, _ := json.Marshal(payload.Report)
	if err := fh.store.StoreReport(r.Context(), node.ID, reportJSON); err != nil {
		fh.logger.Printf("fleet: store report error: %v", err)
	}

	// Update node compliance counts and heartbeat
	summary := payload.Report.Summary
	_ = fh.store.UpdateNodeCompliance(r.Context(), node.ID, summary.Passed, summary.Failed, summary.Warnings)
	_ = fh.store.UpdateNodeHeartbeat(r.Context(), node.ID, time.Now().UTC())

	// Determine status from compliance
	status := fleet.StatusOnline
	if summary.Failed > 0 {
		status = fleet.StatusDegraded
	}
	_ = fh.store.UpdateNodeStatus(r.Context(), node.ID, status)

	// Emit event
	updated, _ := fh.store.GetNode(r.Context(), node.ID)
	if updated != nil && fh.eventCh != nil {
		select {
		case fh.eventCh <- fleet.FleetEvent{
			Type: "node_updated",
			Node: *updated,
			Time: time.Now().UTC(),
		}:
		default:
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// HandleHeartbeat processes a lightweight heartbeat from a node.
func (fh *FleetHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	node, ok := fh.authenticateNode(w, r)
	if !ok {
		return
	}

	if err := fh.store.UpdateNodeHeartbeat(r.Context(), node.ID, time.Now().UTC()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "heartbeat update failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleListNodes returns all nodes with optional filtering.
func (fh *FleetHandler) HandleListNodes(w http.ResponseWriter, r *http.Request) {
	filter := fleet.NodeFilter{
		Role:   fleet.NodeRole(r.URL.Query().Get("role")),
		Region: r.URL.Query().Get("region"),
		Status: fleet.NodeStatus(r.URL.Query().Get("status")),
	}

	nodes, err := fh.store.ListNodes(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list nodes"})
		return
	}
	if nodes == nil {
		nodes = []fleet.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// HandleGetNode returns details of a specific node.
func (fh *FleetHandler) HandleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id required"})
		return
	}

	node, err := fh.store.GetNode(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// HandleGetNodeReport returns the latest compliance report for a node.
func (fh *FleetHandler) HandleGetNodeReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id required"})
		return
	}

	report, err := fh.store.GetLatestReport(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no report found for node"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(report)
}

// HandleDeleteNode removes a node from the fleet (admin only).
func (fh *FleetHandler) HandleDeleteNode(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id required"})
		return
	}

	node, err := fh.store.GetNode(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	if err := fh.store.DeleteNode(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete node"})
		return
	}

	fh.logger.Printf("fleet: node removed: %s (name=%s)", id, node.Name)

	// Emit event
	if fh.eventCh != nil {
		select {
		case fh.eventCh <- fleet.FleetEvent{
			Type: "node_removed",
			Node: *node,
			Time: time.Now().UTC(),
		}:
		default:
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleSummary returns fleet-wide aggregate statistics.
func (fh *FleetHandler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := fh.store.GetSummary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to compute summary"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// HandleFleetSSE provides a Server-Sent Events stream for fleet changes.
func (fh *FleetHandler) HandleFleetSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial fleet summary
	summary, _ := fh.store.GetSummary(r.Context())
	if summary != nil {
		writeSSEEvent(w, flusher, "fleet_summary", summary)
	}

	// Send initial node list
	nodes, _ := fh.store.ListNodes(r.Context(), fleet.NodeFilter{})
	if nodes != nil {
		writeSSEEvent(w, flusher, "fleet_nodes", nodes)
	}

	// Register for events
	ch := make(chan fleet.FleetEvent, 32)
	fh.sseMu.Lock()
	fh.sseClients[ch] = struct{}{}
	fh.sseMu.Unlock()
	defer func() {
		fh.sseMu.Lock()
		delete(fh.sseClients, ch)
		fh.sseMu.Unlock()
	}()

	ctx := r.Context()
	// Also send periodic summary updates
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-ch:
			if err := writeSSEEvent(w, flusher, "fleet_event", evt); err != nil {
				return
			}
		case <-ticker.C:
			s, _ := fh.store.GetSummary(ctx)
			if s != nil {
				if err := writeSSEEvent(w, flusher, "fleet_summary", s); err != nil {
					return
				}
			}
		}
	}
}

// requireAdmin checks that the request has valid admin credentials.
func (fh *FleetHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if fh.adminKey == "" {
		// No admin key configured — allow all (development mode)
		return true
	}
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
		return false
	}
	key := strings.TrimPrefix(auth, "Bearer ")
	if key != fh.adminKey {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid admin credentials"})
		return false
	}
	return true
}

// authenticateNode validates the node API key from the Authorization header.
func (fh *FleetHandler) authenticateNode(w http.ResponseWriter, r *http.Request) (*fleet.Node, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authorization required"})
		return nil, false
	}

	apiKey := strings.TrimPrefix(auth, "Bearer ")
	hash := fleet.HashToken(apiKey)

	node, err := fh.store.GetNodeByAPIKey(r.Context(), hash)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid credentials"})
		return nil, false
	}

	return node, true
}

// FleetMode returns true if the handler is initialized for fleet mode.
func (fh *FleetHandler) FleetMode() bool {
	return fh.store != nil
}

// FleetModeInfo returns a summary for the health check.
func (fh *FleetHandler) FleetModeInfo() map[string]interface{} {
	return map[string]interface{}{
		"fleet_mode": true,
	}
}

package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
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
	policy     *fleet.CompliancePolicy
}

// FleetHandlerConfig holds configuration for the fleet handler.
type FleetHandlerConfig struct {
	Store    fleet.Store
	AdminKey string // API key for admin operations (token management, node deletion)
	Logger   *log.Logger
	EventCh  chan fleet.FleetEvent
	Policy   *fleet.CompliancePolicy // Compliance enforcement policy
}

// NewFleetHandler creates a new fleet handler.
func NewFleetHandler(cfg FleetHandlerConfig) *FleetHandler {
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	policy := cfg.Policy
	if policy == nil {
		policy = &fleet.CompliancePolicy{EnforcementMode: "audit"}
	}
	return &FleetHandler{
		store:      cfg.Store,
		enrollment: fleet.NewEnrollment(cfg.Store),
		adminKey:   cfg.AdminKey,
		logger:     cfg.Logger,
		eventCh:    cfg.EventCh,
		sseClients: make(map[chan fleet.FleetEvent]struct{}),
		policy:     policy,
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
	mux.HandleFunc("GET /api/v1/fleet/policy", fh.HandleGetPolicy)
	mux.HandleFunc("PUT /api/v1/fleet/policy", fh.HandleUpdatePolicy)
	mux.HandleFunc("GET /api/v1/fleet/routes", fh.HandleGetRoutes)
	// Remediation endpoints
	mux.HandleFunc("POST /api/v1/fleet/nodes/{id}/remediate", fh.HandleRequestRemediation)
	mux.HandleFunc("GET /api/v1/fleet/nodes/{id}/remediate", fh.HandlePollRemediations)
	mux.HandleFunc("POST /api/v1/fleet/nodes/{id}/remediate/result", fh.HandlePostRemediationResult)
	mux.HandleFunc("GET /api/v1/fleet/remediate/plan/{id}", fh.HandleGetRemediationPlan)
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

	// Evaluate compliance against policy
	fh.evaluateNodeCompliance(r.Context(), node.ID, payload)

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// evaluateNodeCompliance checks a node's report against the current policy
// and updates its compliance status.
func (fh *FleetHandler) evaluateNodeCompliance(ctx context.Context, nodeID string, payload fleet.ComplianceReportPayload) {
	if fh.policy == nil || fh.policy.EnforcementMode == "disabled" {
		return
	}

	compliant := true

	// Check each policy requirement against report items across all sections
	for _, section := range payload.Report.Sections {
		for _, item := range section.Items {
			if fh.policy.RequireOSFIPS && item.Name == "OS FIPS mode" && item.Status != "pass" {
				compliant = false
			}
			if fh.policy.RequireDiskEnc && item.Name == "Disk encryption" && item.Status != "pass" {
				compliant = false
			}
			if item.Name == "FIPS backend active" && item.Status != "pass" {
				compliant = false
			}
		}
	}

	status := fleet.ComplianceCompliant
	if !compliant {
		status = fleet.ComplianceNonCompliant
	}

	_ = fh.store.UpdateNodeComplianceStatus(ctx, nodeID, string(status))

	if fh.policy.EnforcementMode == "enforce" && !compliant {
		fh.logger.Printf("fleet: node %s is non-compliant (enforcement mode: enforce)", nodeID)
	}
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
	_, _ = w.Write(report)
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
		_ = writeSSEEvent(w, flusher, "fleet_summary", summary)
	}

	// Send initial node list
	nodes, _ := fh.store.ListNodes(r.Context(), fleet.NodeFilter{})
	if nodes != nil {
		_ = writeSSEEvent(w, flusher, "fleet_nodes", nodes)
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

// HandleGetPolicy returns the current compliance policy.
func (fh *FleetHandler) HandleGetPolicy(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, fh.policy)
}

// HandleUpdatePolicy updates the compliance policy (admin only).
func (fh *FleetHandler) HandleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	var policy fleet.CompliancePolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	switch policy.EnforcementMode {
	case "enforce", "audit", "disabled":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enforcement_mode must be enforce, audit, or disabled"})
		return
	}

	fh.policy = &policy
	fh.logger.Printf("fleet: compliance policy updated: mode=%s", policy.EnforcementMode)
	writeJSON(w, http.StatusOK, fh.policy)
}

// HandleGetRoutes returns the effective routing table: only compliant server nodes.
func (fh *FleetHandler) HandleGetRoutes(w http.ResponseWriter, r *http.Request) {
	nodes, err := fh.store.ListNodes(r.Context(), fleet.NodeFilter{Role: fleet.RoleServer})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list nodes"})
		return
	}

	type route struct {
		NodeID           string                      `json:"node_id"`
		NodeName         string                      `json:"node_name"`
		Service          *fleet.ServiceRegistration  `json:"service,omitempty"`
		Status           fleet.NodeStatus            `json:"status"`
		ComplianceStatus fleet.NodeComplianceStatus  `json:"compliance_status"`
		Routable         bool                        `json:"routable"`
	}

	var routes []route
	for _, n := range nodes {
		routable := n.Status == fleet.StatusOnline
		if fh.policy != nil && fh.policy.EnforcementMode == "enforce" {
			routable = routable && n.ComplianceStatus == fleet.ComplianceCompliant
		}
		routes = append(routes, route{
			NodeID:           n.ID,
			NodeName:         n.Name,
			Service:          n.Service,
			Status:           n.Status,
			ComplianceStatus: n.ComplianceStatus,
			Routable:         routable,
		})
	}

	if routes == nil {
		routes = []route{}
	}
	writeJSON(w, http.StatusOK, routes)
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

// HandleRequestRemediation creates a remediation request for a node (admin only).
func (fh *FleetHandler) HandleRequestRemediation(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	nodeID := r.PathValue("id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id required"})
		return
	}

	// Verify node exists
	if _, err := fh.store.GetNode(r.Context(), nodeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	var body struct {
		Actions []string `json:"actions"`
		DryRun  bool     `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(body.Actions) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "actions list required"})
		return
	}

	reqID := generateID()
	req := &fleet.RemediationRequest{
		ID:        reqID,
		NodeID:    nodeID,
		Actions:   body.Actions,
		DryRun:    body.DryRun,
		Status:    fleet.RemediationPending,
		CreatedAt: time.Now().UTC(),
	}

	if err := fh.store.CreateRemediationRequest(r.Context(), req); err != nil {
		fh.logger.Printf("fleet: create remediation request error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create remediation request"})
		return
	}

	fh.logger.Printf("fleet: remediation requested for node %s: %v (dry_run=%v)", nodeID, body.Actions, body.DryRun)

	// Emit SSE event
	if fh.eventCh != nil {
		node, _ := fh.store.GetNode(r.Context(), nodeID)
		if node != nil {
			select {
			case fh.eventCh <- fleet.FleetEvent{
				Type: "remediation_requested",
				Node: *node,
				Time: time.Now().UTC(),
			}:
			default:
			}
		}
	}

	writeJSON(w, http.StatusCreated, req)
}

// HandlePollRemediations returns pending remediation requests for a node (node auth).
func (fh *FleetHandler) HandlePollRemediations(w http.ResponseWriter, r *http.Request) {
	node, ok := fh.authenticateNode(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("id")
	if nodeID != node.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "node id mismatch"})
		return
	}

	reqs, err := fh.store.GetPendingRemediations(r.Context(), nodeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get pending remediations"})
		return
	}
	if reqs == nil {
		reqs = []fleet.RemediationRequest{}
	}
	writeJSON(w, http.StatusOK, reqs)
}

// HandlePostRemediationResult receives a remediation result from a node (node auth).
func (fh *FleetHandler) HandlePostRemediationResult(w http.ResponseWriter, r *http.Request) {
	node, ok := fh.authenticateNode(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("id")
	if nodeID != node.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "node id mismatch"})
		return
	}

	var body struct {
		RequestID string          `json:"request_id"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Verify the request belongs to this node
	req, err := fh.store.GetRemediationRequest(r.Context(), body.RequestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "remediation request not found"})
		return
	}
	if req.NodeID != nodeID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "request does not belong to this node"})
		return
	}

	if err := fh.store.CompleteRemediation(r.Context(), body.RequestID, body.Result); err != nil {
		fh.logger.Printf("fleet: complete remediation error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to complete remediation"})
		return
	}

	fh.logger.Printf("fleet: remediation completed for node %s (request %s)", nodeID, body.RequestID)

	// Emit SSE event
	if fh.eventCh != nil {
		select {
		case fh.eventCh <- fleet.FleetEvent{
			Type: "remediation_completed",
			Node: *node,
			Time: time.Now().UTC(),
		}:
		default:
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// HandleGetRemediationPlan returns available remediation actions for a node
// based on its latest compliance report (admin only).
func (fh *FleetHandler) HandleGetRemediationPlan(w http.ResponseWriter, r *http.Request) {
	if !fh.requireAdmin(w, r) {
		return
	}

	nodeID := r.PathValue("id")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id required"})
		return
	}

	report, err := fh.store.GetLatestReport(r.Context(), nodeID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no compliance report found"})
		return
	}

	// Parse the report to identify failed items
	var compReport struct {
		Sections []struct {
			Items []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Status      string `json:"status"`
				Remediation string `json:"remediation"`
			} `json:"items"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(report, &compReport); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse compliance report"})
		return
	}

	// Map failed items to available actions
	type planAction struct {
		ID           string `json:"id"`
		Description  string `json:"description"`
		AutoExec     bool   `json:"auto_exec"`
		Instructions string `json:"instructions"`
		FailedCheck  string `json:"failed_check"`
	}

	var actions []planAction
	for _, section := range compReport.Sections {
		for _, item := range section.Items {
			if item.Status == "pass" {
				continue
			}
			switch item.ID {
			case "ag-fips":
				actions = append(actions, planAction{
					ID: "enable_os_fips", Description: "Enable OS FIPS mode",
					AutoExec: true, Instructions: item.Remediation, FailedCheck: item.Name,
				})
			case "ag-warp":
				if strings.Contains(item.Remediation, "not connected") || strings.Contains(item.Remediation, "not running") {
					actions = append(actions, planAction{
						ID: "connect_warp", Description: "Connect Cloudflare WARP",
						AutoExec: true, Instructions: "warp-cli connect", FailedCheck: item.Name,
					})
				} else {
					actions = append(actions, planAction{
						ID: "install_warp", Description: "Install Cloudflare WARP",
						AutoExec: true, Instructions: item.Remediation, FailedCheck: item.Name,
					})
				}
			case "ag-disk":
				actions = append(actions, planAction{
					ID: "enable_disk_encryption", Description: "Enable full-disk encryption",
					AutoExec: false, Instructions: item.Remediation, FailedCheck: item.Name,
				})
			case "ag-mdm":
				actions = append(actions, planAction{
					ID: "enroll_mdm", Description: "Enroll device in MDM",
					AutoExec: false, Instructions: item.Remediation, FailedCheck: item.Name,
				})
			}
		}
	}

	if actions == nil {
		actions = []planAction{}
	}
	writeJSON(w, http.StatusOK, actions)
}

// generateID produces a simple unique ID for remediation requests.
func generateID() string {
	return fmt.Sprintf("rem-%d", time.Now().UnixNano())
}

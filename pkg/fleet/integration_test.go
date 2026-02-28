package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// TestIntegration_FullEnrollmentAndReportFlow tests the complete lifecycle:
// create token → enroll → send report → verify data.
func TestIntegration_FullEnrollmentAndReportFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(filepath.Join(dir, "integration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	// 1. Create enrollment token
	enrollment := NewEnrollment(store)
	token, err := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleServer,
		Region:    "us-east",
		MaxUses:   5,
		ExpiresIn: 3600,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Token created: %s", token.ID)

	// 2. Enroll 3 nodes
	var nodeIDs []string
	var apiKeys []string
	for i, name := range []string{"server-east-1", "server-east-2", "server-east-3"} {
		resp, err := enrollment.Enroll(ctx, EnrollmentRequest{
			Token:       token.Token,
			Name:        name,
			Version:     "0.1.0",
			FIPSBackend: "BoringCrypto",
		})
		if err != nil {
			t.Fatalf("enroll %d: %v", i, err)
		}
		nodeIDs = append(nodeIDs, resp.NodeID)
		apiKeys = append(apiKeys, resp.APIKey)
		t.Logf("Node enrolled: %s (ID: %s)", name, resp.NodeID)
	}

	// 3. Check token usage
	tokens, _ := enrollment.ListTokens(ctx)
	if len(tokens) != 1 || tokens[0].UsedCount != 3 {
		t.Errorf("expected 1 token with 3 uses, got %d tokens", len(tokens))
	}

	// 4. Send compliance reports
	for i, nodeID := range nodeIDs {
		report := compliance.ComplianceReport{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Sections: []compliance.Section{{
				ID:   "tunnel",
				Name: "Tunnel",
				Items: []compliance.ChecklistItem{
					{ID: "t1", Name: "BoringCrypto Active", Status: compliance.StatusPass},
					{ID: "t2", Name: "OS FIPS Mode", Status: compliance.StatusPass},
				},
			}},
			Summary: compliance.Summary{Total: 2, Passed: 2},
		}
		// Node 3 has a failure
		if i == 2 {
			report.Sections[0].Items[1].Status = compliance.StatusFail
			report.Summary.Passed = 1
			report.Summary.Failed = 1
		}

		reportJSON, _ := json.Marshal(report)
		if err := store.StoreReport(ctx, nodeID, reportJSON); err != nil {
			t.Fatalf("store report %d: %v", i, err)
		}
		// Update compliance counts
		store.UpdateNodeCompliance(ctx, nodeID, report.Summary.Passed, report.Summary.Failed, report.Summary.Warnings)
		if report.Summary.Failed > 0 {
			store.UpdateNodeStatus(ctx, nodeID, StatusDegraded)
		}
	}

	// 5. Verify fleet summary
	summary, err := store.GetSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", summary.TotalNodes)
	}
	if summary.Online != 2 {
		t.Errorf("Online = %d, want 2", summary.Online)
	}
	if summary.Degraded != 1 {
		t.Errorf("Degraded = %d, want 1", summary.Degraded)
	}
	if summary.FullyCompliant != 2 {
		t.Errorf("FullyCompliant = %d, want 2", summary.FullyCompliant)
	}

	// 6. Verify reports can be retrieved
	for _, nodeID := range nodeIDs {
		report, err := store.GetLatestReport(ctx, nodeID)
		if err != nil {
			t.Fatalf("GetLatestReport %s: %v", nodeID, err)
		}
		var parsed compliance.ComplianceReport
		if err := json.Unmarshal(report, &parsed); err != nil {
			t.Fatalf("parse report: %v", err)
		}
		if len(parsed.Sections) != 1 {
			t.Errorf("sections = %d, want 1", len(parsed.Sections))
		}
	}

	// 7. Verify node lookup by API key
	for i, key := range apiKeys {
		hash := HashToken(key)
		node, err := store.GetNodeByAPIKey(ctx, hash)
		if err != nil {
			t.Fatalf("GetNodeByAPIKey %d: %v", i, err)
		}
		if node.ID != nodeIDs[i] {
			t.Errorf("node ID mismatch: got %s, want %s", node.ID, nodeIDs[i])
		}
	}

	// 8. Delete a node
	if err := store.DeleteNode(ctx, nodeIDs[2]); err != nil {
		t.Fatal(err)
	}
	summary2, _ := store.GetSummary(ctx)
	if summary2.TotalNodes != 2 {
		t.Errorf("after delete: TotalNodes = %d, want 2", summary2.TotalNodes)
	}

	t.Log("Integration test complete: enrollment → report → summary → delete all passed")
}

// TestIntegration_MonitorMarksStaleNodes tests the monitor goroutine.
func TestIntegration_MonitorMarksStaleNodes(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(filepath.Join(dir, "monitor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	// Create a node with an old heartbeat
	staleTime := time.Now().UTC().Add(-5 * time.Minute)
	node := &Node{
		ID: "stale-1", Name: "stale", Role: RoleServer,
		EnrolledAt: staleTime, LastHeartbeat: staleTime, Status: StatusOnline,
	}
	store.CreateNode(ctx, node, "h-stale")

	eventCh := make(chan FleetEvent, 10)
	monitor := NewMonitor(MonitorConfig{
		Store:         store,
		DegradedAfter: 1 * time.Second,
		OfflineAfter:  2 * time.Second,
		CheckInterval: 100 * time.Millisecond,
		EventCh:       eventCh,
	})

	mctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	go monitor.Run(mctx)

	<-mctx.Done()

	// Check that the node was marked offline
	got, _ := store.GetNode(ctx, "stale-1")
	if got.Status != StatusOffline {
		t.Errorf("expected offline, got %s", got.Status)
	}

	// Check events were emitted
	if len(eventCh) == 0 {
		t.Error("expected at least one event from monitor")
	}
}

// TestIntegration_ReporterHTTP tests the reporter sends HTTP requests.
func TestIntegration_ReporterHTTP(t *testing.T) {
	var received int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received++
		if r.URL.Path == "/api/v1/fleet/report" {
			var payload ComplianceReportPayload
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.NodeID != "test-node" {
				t.Errorf("unexpected node ID: %s", payload.NodeID)
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	checker := compliance.NewChecker()
	checker.AddSection(compliance.Section{
		ID:   "test",
		Name: "Test",
		Items: []compliance.ChecklistItem{
			{ID: "t1", Name: "Test Check", Status: compliance.StatusPass},
		},
	})

	reporter := NewReporter(ReporterConfig{
		ControllerURL: ts.URL,
		NodeID:        "test-node",
		APIKey:        "test-key",
		Checker:       checker,
		Interval:      100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	go reporter.Run(ctx)

	<-ctx.Done()

	// Should have sent initial report + at least 1-2 more + heartbeats
	if received < 2 {
		t.Errorf("expected at least 2 HTTP calls, got %d", received)
	}
}

// TestIntegration_ComplianceReportFormat tests that compliance reports
// stored by the fleet system have the correct structure.
func TestIntegration_ComplianceReportFormat(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(filepath.Join(dir, "format.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	now := time.Now().UTC()
	store.CreateNode(ctx, &Node{
		ID: "n1", Name: "test", Role: RoleServer,
		EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline,
	}, "h1")

	// Build a realistic compliance report
	report := compliance.ComplianceReport{
		Timestamp: now.Format(time.RFC3339),
		Sections: []compliance.Section{
			{
				ID:          "tunnel",
				Name:        "Tunnel — cloudflared",
				Description: "FIPS compliance of the cloudflared tunnel binary",
				Items: []compliance.ChecklistItem{
					{ID: "t1", Name: "BoringCrypto Active", Status: compliance.StatusPass, Severity: "critical", VerificationMethod: compliance.VerifyDirect},
					{ID: "t2", Name: "OS FIPS Mode", Status: compliance.StatusPass, Severity: "critical", VerificationMethod: compliance.VerifyDirect},
					{ID: "t3", Name: "FIPS Self-Test", Status: compliance.StatusPass, Severity: "critical", VerificationMethod: compliance.VerifyDirect},
				},
			},
		},
		Summary: compliance.Summary{Total: 3, Passed: 3},
	}

	reportJSON, _ := json.Marshal(report)
	store.StoreReport(ctx, "n1", reportJSON)

	// Retrieve and verify
	got, _ := store.GetLatestReport(ctx, "n1")
	var parsed compliance.ComplianceReport
	json.Unmarshal(got, &parsed)

	if parsed.Summary.Total != 3 || parsed.Summary.Passed != 3 {
		t.Errorf("summary = %+v", parsed.Summary)
	}
	if len(parsed.Sections) != 1 {
		t.Fatalf("sections = %d", len(parsed.Sections))
	}
	if len(parsed.Sections[0].Items) != 3 {
		t.Errorf("items = %d", len(parsed.Sections[0].Items))
	}

	// Verify serialization roundtrip preserves verification method
	if parsed.Sections[0].Items[0].VerificationMethod != compliance.VerifyDirect {
		t.Errorf("verification method = %q, want direct", parsed.Sections[0].Items[0].VerificationMethod)
	}
}

// TestIntegration_AgentChecks tests the agent check suite runs.
func TestIntegration_AgentChecks(t *testing.T) {
	agent := NewAgentChecks()
	section := agent.RunChecks()

	if section.ID != "agent-posture" {
		t.Errorf("section ID = %q, want agent-posture", section.ID)
	}
	if len(section.Items) < 5 {
		t.Errorf("expected at least 5 checks, got %d", len(section.Items))
	}

	// Verify all items have required fields
	for _, item := range section.Items {
		if item.ID == "" || item.Name == "" {
			t.Errorf("item missing ID or Name: %+v", item)
		}
		if item.Status == "" {
			t.Errorf("item %s has empty status", item.ID)
		}
		if item.Severity == "" {
			t.Errorf("item %s has empty severity", item.ID)
		}
	}
}

func init() {
	// Suppress HTTP test noise
	_ = bytes.NewReader
}

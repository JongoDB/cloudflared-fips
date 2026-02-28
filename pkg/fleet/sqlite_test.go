package fleet

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-fleet.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteStore_CreateAndGetNode(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	node := &Node{
		ID:            "node-1",
		Name:          "us-east-1",
		Role:          RoleServer,
		Region:        "us-east",
		Labels:        map[string]string{"env": "prod"},
		EnrolledAt:    now,
		LastHeartbeat: now,
		Status:        StatusOnline,
		Version:       "0.1.0",
		FIPSBackend:   "BoringCrypto",
	}

	if err := store.CreateNode(ctx, node, "hash-abc"); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := store.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Name != "us-east-1" {
		t.Errorf("Name = %q, want %q", got.Name, "us-east-1")
	}
	if got.Role != RoleServer {
		t.Errorf("Role = %q, want %q", got.Role, RoleServer)
	}
	if got.Region != "us-east" {
		t.Errorf("Region = %q, want %q", got.Region, "us-east")
	}
	if got.Status != StatusOnline {
		t.Errorf("Status = %q, want %q", got.Status, StatusOnline)
	}
	if got.Labels["env"] != "prod" {
		t.Errorf("Labels[env] = %q, want %q", got.Labels["env"], "prod")
	}
}

func TestSQLiteStore_ListNodes_Filter(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	nodes := []struct {
		node *Node
		hash string
	}{
		{&Node{ID: "n1", Name: "svr1", Role: RoleServer, Region: "us-east", EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}, "h1"},
		{&Node{ID: "n2", Name: "svr2", Role: RoleServer, Region: "us-west", EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}, "h2"},
		{&Node{ID: "n3", Name: "prx1", Role: RoleProxy, Region: "us-east", EnrolledAt: now, LastHeartbeat: now, Status: StatusDegraded}, "h3"},
		{&Node{ID: "n4", Name: "cli1", Role: RoleClient, Region: "eu-west", EnrolledAt: now, LastHeartbeat: now, Status: StatusOffline}, "h4"},
	}
	for _, n := range nodes {
		if err := store.CreateNode(ctx, n.node, n.hash); err != nil {
			t.Fatalf("CreateNode %s: %v", n.node.ID, err)
		}
	}

	// Filter by role
	servers, err := store.ListNodes(ctx, NodeFilter{Role: RoleServer})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("server count = %d, want 2", len(servers))
	}

	// Filter by region
	east, err := store.ListNodes(ctx, NodeFilter{Region: "us-east"})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(east) != 2 {
		t.Errorf("us-east count = %d, want 2", len(east))
	}

	// Filter by status
	offline, err := store.ListNodes(ctx, NodeFilter{Status: StatusOffline})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(offline) != 1 {
		t.Errorf("offline count = %d, want 1", len(offline))
	}

	// All nodes
	all, err := store.ListNodes(ctx, NodeFilter{})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("total count = %d, want 4", len(all))
	}
}

func TestSQLiteStore_UpdateHeartbeat(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusDegraded}
	store.CreateNode(ctx, node, "h1")

	later := now.Add(5 * time.Minute)
	if err := store.UpdateNodeHeartbeat(ctx, "n1", later); err != nil {
		t.Fatalf("UpdateNodeHeartbeat: %v", err)
	}

	got, _ := store.GetNode(ctx, "n1")
	if got.Status != StatusOnline {
		t.Errorf("Status after heartbeat = %q, want online", got.Status)
	}
}

func TestSQLiteStore_UpdateCompliance(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "h1")

	if err := store.UpdateNodeCompliance(ctx, "n1", 38, 2, 1); err != nil {
		t.Fatalf("UpdateNodeCompliance: %v", err)
	}

	got, _ := store.GetNode(ctx, "n1")
	if got.CompliancePass != 38 || got.ComplianceFail != 2 || got.ComplianceWarn != 1 {
		t.Errorf("Compliance = %d/%d/%d, want 38/2/1", got.CompliancePass, got.ComplianceFail, got.ComplianceWarn)
	}
}

func TestSQLiteStore_DeleteNode(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "h1")

	if err := store.DeleteNode(ctx, "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	_, err := store.GetNode(ctx, "n1")
	if err == nil {
		t.Error("expected error getting deleted node")
	}
}

func TestSQLiteStore_GetNodeByAPIKey(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "keyhash-123")

	got, err := store.GetNodeByAPIKey(ctx, "keyhash-123")
	if err != nil {
		t.Fatalf("GetNodeByAPIKey: %v", err)
	}
	if got.ID != "n1" {
		t.Errorf("ID = %q, want n1", got.ID)
	}
}

func TestSQLiteStore_Tokens(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	token := &EnrollmentToken{
		ID:        "tok-1",
		Role:      RoleServer,
		Region:    "us-east",
		ExpiresAt: now.Add(time.Hour),
		MaxUses:   5,
		CreatedAt: now,
	}
	if err := store.CreateToken(ctx, token, "tokenhash-abc"); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	got, err := store.GetToken(ctx, "tokenhash-abc")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.ID != "tok-1" || got.Role != RoleServer {
		t.Errorf("Token = %+v", got)
	}

	tokens, err := store.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Errorf("token count = %d, want 1", len(tokens))
	}

	if err := store.IncrementTokenUsage(ctx, "tok-1"); err != nil {
		t.Fatalf("IncrementTokenUsage: %v", err)
	}
	got2, _ := store.GetToken(ctx, "tokenhash-abc")
	if got2.UsedCount != 1 {
		t.Errorf("UsedCount = %d, want 1", got2.UsedCount)
	}

	if err := store.DeleteToken(ctx, "tok-1"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	tokens2, _ := store.ListTokens(ctx)
	if len(tokens2) != 0 {
		t.Errorf("token count after delete = %d, want 0", len(tokens2))
	}
}

func TestSQLiteStore_Reports(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "h1")

	report := []byte(`{"timestamp":"2026-02-28T00:00:00Z","sections":[],"summary":{"total":41,"passed":41}}`)
	if err := store.StoreReport(ctx, "n1", report); err != nil {
		t.Fatalf("StoreReport: %v", err)
	}

	got, err := store.GetLatestReport(ctx, "n1")
	if err != nil {
		t.Fatalf("GetLatestReport: %v", err)
	}
	if string(got) != string(report) {
		t.Errorf("report mismatch")
	}
}

func TestSQLiteStore_Summary(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	nodes := []*Node{
		{ID: "n1", Name: "s1", Role: RoleServer, Region: "us-east", EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline},
		{ID: "n2", Name: "s2", Role: RoleServer, Region: "us-west", EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline},
		{ID: "n3", Name: "p1", Role: RoleProxy, Region: "us-east", EnrolledAt: now, LastHeartbeat: now, Status: StatusDegraded},
		{ID: "n4", Name: "c1", Role: RoleClient, Region: "eu-west", EnrolledAt: now, LastHeartbeat: now, Status: StatusOffline},
	}
	for i, n := range nodes {
		store.CreateNode(ctx, n, "h"+n.ID)
		// Set compliance - n3 has failures
		if i == 2 {
			store.UpdateNodeCompliance(ctx, n.ID, 30, 5, 2)
		} else {
			store.UpdateNodeCompliance(ctx, n.ID, 41, 0, 0)
		}
	}

	summary, err := store.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.TotalNodes != 4 {
		t.Errorf("TotalNodes = %d, want 4", summary.TotalNodes)
	}
	if summary.Online != 2 {
		t.Errorf("Online = %d, want 2", summary.Online)
	}
	if summary.Degraded != 1 {
		t.Errorf("Degraded = %d, want 1", summary.Degraded)
	}
	if summary.Offline != 1 {
		t.Errorf("Offline = %d, want 1", summary.Offline)
	}
	if summary.ByRole["server"] != 2 {
		t.Errorf("ByRole[server] = %d, want 2", summary.ByRole["server"])
	}
	// FullyCompliant: n1, n2 are online with 0 failures. n3 has failures. n4 is offline (excluded).
	if summary.FullyCompliant != 2 {
		t.Errorf("FullyCompliant = %d, want 2", summary.FullyCompliant)
	}
}

func TestSQLiteStore_UpdateNodeStatus(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "h1")

	// Mark degraded
	if err := store.UpdateNodeStatus(ctx, "n1", StatusDegraded); err != nil {
		t.Fatalf("UpdateNodeStatus to degraded: %v", err)
	}
	got, _ := store.GetNode(ctx, "n1")
	if got.Status != StatusDegraded {
		t.Errorf("Status = %q, want degraded", got.Status)
	}

	// Mark offline
	if err := store.UpdateNodeStatus(ctx, "n1", StatusOffline); err != nil {
		t.Fatalf("UpdateNodeStatus to offline: %v", err)
	}
	got2, _ := store.GetNode(ctx, "n1")
	if got2.Status != StatusOffline {
		t.Errorf("Status = %q, want offline", got2.Status)
	}

	// Back to online
	if err := store.UpdateNodeStatus(ctx, "n1", StatusOnline); err != nil {
		t.Fatalf("UpdateNodeStatus to online: %v", err)
	}
	got3, _ := store.GetNode(ctx, "n1")
	if got3.Status != StatusOnline {
		t.Errorf("Status = %q, want online", got3.Status)
	}
}

func TestSQLiteStore_GetLatestReport_Nonexistent(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	_, err := store.GetLatestReport(ctx, "nonexistent-node")
	if err == nil {
		t.Error("expected error for nonexistent node report")
	}
}

func TestSQLiteStore_GetNodeByAPIKey_Nonexistent(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	_, err := store.GetNodeByAPIKey(ctx, "nonexistent-hash")
	if err == nil {
		t.Error("expected error for nonexistent API key")
	}
}

func TestSQLiteStore_GetNode_Nonexistent(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	_, err := store.GetNode(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestSQLiteStore_MultipleReports_LatestReturned(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "h1")

	// Store two reports with >1s gap to guarantee different RFC3339 timestamps
	report1 := []byte(`{"version":"1"}`)
	report2 := []byte(`{"version":"2"}`)
	if err := store.StoreReport(ctx, "n1", report1); err != nil {
		t.Fatalf("StoreReport 1: %v", err)
	}
	// SQLite timestamps use RFC3339 (second precision), need >1s gap
	time.Sleep(1100 * time.Millisecond)
	if err := store.StoreReport(ctx, "n1", report2); err != nil {
		t.Fatalf("StoreReport 2: %v", err)
	}

	got, err := store.GetLatestReport(ctx, "n1")
	if err != nil {
		t.Fatalf("GetLatestReport: %v", err)
	}
	if string(got) != `{"version":"2"}` {
		t.Errorf("GetLatestReport = %q, want report 2", string(got))
	}
}

func TestSQLiteStore_DeleteNode_CascadesReports(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	store.CreateNode(ctx, node, "h1")
	store.StoreReport(ctx, "n1", []byte(`{"report":true}`))

	if err := store.DeleteNode(ctx, "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	// Reports should be cascade-deleted
	_, err := store.GetLatestReport(ctx, "n1")
	if err == nil {
		t.Error("expected error for report of deleted node")
	}
}

func TestSQLiteStore_ListNodes_NoFilter(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	// Empty DB
	nodes, err := store.ListNodes(ctx, NodeFilter{})
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestSQLiteStore_Summary_EmptyDB(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	summary, err := store.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if summary.TotalNodes != 0 {
		t.Errorf("TotalNodes = %d, want 0", summary.TotalNodes)
	}
	if summary.FullyCompliant != 0 {
		t.Errorf("FullyCompliant = %d, want 0", summary.FullyCompliant)
	}
}

func TestSQLiteStore_DuplicateNodeID(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	if err := store.CreateNode(ctx, node, "h1"); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Duplicate ID should fail
	node2 := &Node{ID: "n1", Name: "dup", Role: RoleProxy, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	err := store.CreateNode(ctx, node2, "h2")
	if err == nil {
		t.Error("expected error for duplicate node ID")
	}
}

func TestSQLiteStore_DuplicateAPIKeyHash(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	node := &Node{ID: "n1", Name: "test", Role: RoleServer, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	if err := store.CreateNode(ctx, node, "same-hash"); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Duplicate API key hash should fail (UNIQUE constraint)
	node2 := &Node{ID: "n2", Name: "test2", Role: RoleProxy, EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline}
	err := store.CreateNode(ctx, node2, "same-hash")
	if err == nil {
		t.Error("expected error for duplicate API key hash")
	}
}

func TestSQLiteStore_PersistReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Create and populate
	store1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	now := time.Now().UTC()
	store1.CreateNode(context.Background(), &Node{
		ID: "n1", Name: "test", Role: RoleServer,
		EnrolledAt: now, LastHeartbeat: now, Status: StatusOnline,
	}, "h1")
	store1.Close()

	// Reopen and verify
	store2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	got, err := store2.GetNode(context.Background(), "n1")
	if err != nil {
		t.Fatalf("GetNode after reopen: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want test", got.Name)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file should exist: %v", err)
	}
}

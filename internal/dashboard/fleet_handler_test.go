package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/fleet"
)

func testFleetHandler(t *testing.T) (*FleetHandler, fleet.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-fleet.db")
	store, err := fleet.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	eventCh := make(chan fleet.FleetEvent, 64)
	fh := NewFleetHandler(FleetHandlerConfig{
		Store:    store,
		AdminKey: "admin-secret",
		EventCh:  eventCh,
	})
	return fh, store
}

func TestFleetHandler_CreateToken(t *testing.T) {
	fh, _ := testFleetHandler(t)

	body := `{"role":"server","region":"us-east","max_uses":5,"expires_in":3600}`
	req := httptest.NewRequest("POST", "/api/v1/fleet/tokens", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer admin-secret")
	w := httptest.NewRecorder()

	fh.HandleCreateToken(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d. Body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var token fleet.EnrollmentToken
	if err := json.Unmarshal(w.Body.Bytes(), &token); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if token.Token == "" {
		t.Error("token string should be returned")
	}
	if token.Role != fleet.RoleServer {
		t.Errorf("role = %q, want server", token.Role)
	}
}

func TestFleetHandler_CreateTokenUnauthorized(t *testing.T) {
	fh, _ := testFleetHandler(t)

	body := `{"role":"server"}`
	req := httptest.NewRequest("POST", "/api/v1/fleet/tokens", bytes.NewBufferString(body))
	// No auth header
	w := httptest.NewRecorder()

	fh.HandleCreateToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestFleetHandler_CreateTokenWrongKey(t *testing.T) {
	fh, _ := testFleetHandler(t)

	body := `{"role":"server"}`
	req := httptest.NewRequest("POST", "/api/v1/fleet/tokens", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()

	fh.HandleCreateToken(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestFleetHandler_EnrollAndReport(t *testing.T) {
	fh, store := testFleetHandler(t)

	// Create token
	enrollment := fleet.NewEnrollment(store)
	token, err := enrollment.CreateToken(context.Background(), fleet.CreateTokenRequest{
		Role:      fleet.RoleServer,
		MaxUses:   1,
		ExpiresIn: 3600,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Enroll
	enrollBody, _ := json.Marshal(fleet.EnrollmentRequest{
		Token:       token.Token,
		Name:        "test-server",
		Version:     "0.1.0",
		FIPSBackend: "BoringCrypto",
	})
	req := httptest.NewRequest("POST", "/api/v1/fleet/enroll", bytes.NewReader(enrollBody))
	w := httptest.NewRecorder()
	fh.HandleEnroll(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, body: %s", w.Code, w.Body.String())
	}

	var enrollResp fleet.EnrollmentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &enrollResp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if enrollResp.NodeID == "" {
		t.Fatal("node ID empty")
	}

	// Send heartbeat
	hbBody, _ := json.Marshal(fleet.HeartbeatRequest{NodeID: enrollResp.NodeID})
	req2 := httptest.NewRequest("POST", "/api/v1/fleet/heartbeat", bytes.NewReader(hbBody))
	req2.Header.Set("Authorization", "Bearer "+enrollResp.APIKey)
	w2 := httptest.NewRecorder()
	fh.HandleHeartbeat(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("heartbeat status = %d", w2.Code)
	}
}

func TestFleetHandler_ListNodes(t *testing.T) {
	fh, store := testFleetHandler(t)

	// Enroll two nodes
	enrollment := fleet.NewEnrollment(store)
	tok, _ := enrollment.CreateToken(context.Background(), fleet.CreateTokenRequest{
		Role: fleet.RoleServer, MaxUses: 5, ExpiresIn: 3600,
	})

	for _, name := range []string{"server-1", "server-2"} {
		if _, err := enrollment.Enroll(context.Background(), fleet.EnrollmentRequest{
			Token: tok.Token, Name: name,
		}); err != nil {
			t.Fatalf("Enroll %s: %v", name, err)
		}
	}

	req := httptest.NewRequest("GET", "/api/v1/fleet/nodes", nil)
	w := httptest.NewRecorder()
	fh.HandleListNodes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var nodes []fleet.Node
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("node count = %d, want 2", len(nodes))
	}
}

func TestFleetHandler_Summary(t *testing.T) {
	fh, store := testFleetHandler(t)

	enrollment := fleet.NewEnrollment(store)
	tok, _ := enrollment.CreateToken(context.Background(), fleet.CreateTokenRequest{
		Role: fleet.RoleServer, MaxUses: 5, ExpiresIn: 3600,
	})
	if _, err := enrollment.Enroll(context.Background(), fleet.EnrollmentRequest{
		Token: tok.Token, Name: "s1",
	}); err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/fleet/summary", nil)
	w := httptest.NewRecorder()
	fh.HandleSummary(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var summary fleet.FleetSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if summary.TotalNodes != 1 {
		t.Errorf("TotalNodes = %d, want 1", summary.TotalNodes)
	}
}

func TestFleetHandler_DeleteNode(t *testing.T) {
	fh, store := testFleetHandler(t)

	enrollment := fleet.NewEnrollment(store)
	tok, _ := enrollment.CreateToken(context.Background(), fleet.CreateTokenRequest{
		Role: fleet.RoleServer, MaxUses: 1, ExpiresIn: 3600,
	})
	resp, _ := enrollment.Enroll(context.Background(), fleet.EnrollmentRequest{
		Token: tok.Token, Name: "s1",
	})

	// Create a new request with path value
	mux := http.NewServeMux()
	RegisterFleetRoutes(mux, fh)

	req := httptest.NewRequest("DELETE", "/api/v1/fleet/nodes/"+resp.NodeID, nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestFleetHandler_ListTokens(t *testing.T) {
	fh, store := testFleetHandler(t)

	enrollment := fleet.NewEnrollment(store)
	if _, err := enrollment.CreateToken(context.Background(), fleet.CreateTokenRequest{
		Role: fleet.RoleServer, ExpiresIn: 3600,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := enrollment.CreateToken(context.Background(), fleet.CreateTokenRequest{
		Role: fleet.RoleClient, ExpiresIn: 3600,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/v1/fleet/tokens", nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	w := httptest.NewRecorder()
	fh.HandleListTokens(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var tokens []fleet.EnrollmentToken
	if err := json.Unmarshal(w.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(tokens) != 2 {
		t.Errorf("token count = %d, want 2", len(tokens))
	}
}

func TestFleetHandler_BroadcastEvents(t *testing.T) {
	fh, _ := testFleetHandler(t)
	done := make(chan struct{})

	// Register an SSE client
	clientCh := make(chan fleet.FleetEvent, 10)
	fh.sseMu.Lock()
	fh.sseClients[clientCh] = struct{}{}
	fh.sseMu.Unlock()

	go fh.BroadcastEvents(done)

	// Send an event through the event channel
	fh.eventCh <- fleet.FleetEvent{
		Type: "node_joined",
		Node: fleet.Node{ID: "test-node-1", Name: "server-1"},
	}

	// Wait for the event to be broadcast to the client
	select {
	case evt := <-clientCh:
		if evt.Type != "node_joined" {
			t.Errorf("event type = %q, want node_joined", evt.Type)
		}
		if evt.Node.ID != "test-node-1" {
			t.Errorf("event Node.ID = %q, want test-node-1", evt.Node.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}

	// Close done channel to stop BroadcastEvents goroutine
	close(done)
}

func TestFleetHandler_BroadcastEvents_ChannelClosed(t *testing.T) {
	fh, _ := testFleetHandler(t)
	done := make(chan struct{})

	finished := make(chan struct{})
	go func() {
		fh.BroadcastEvents(done)
		close(finished)
	}()

	// Close the event channel — BroadcastEvents should return
	close(fh.eventCh)

	select {
	case <-finished:
		// Good — BroadcastEvents returned when eventCh was closed
	case <-time.After(time.Second):
		close(done) // Clean up
		t.Fatal("BroadcastEvents did not return when eventCh was closed")
	}
}

func TestFleetHandler_BroadcastEvents_SlowClient(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-fleet.db")
	store, err := fleet.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	eventCh := make(chan fleet.FleetEvent, 64)
	fh := NewFleetHandler(FleetHandlerConfig{
		Store:    store,
		AdminKey: "admin-secret",
		EventCh:  eventCh,
	})
	done := make(chan struct{})

	// Register a slow client with buffer size 0 (will block)
	slowCh := make(chan fleet.FleetEvent) // unbuffered
	fh.sseMu.Lock()
	fh.sseClients[slowCh] = struct{}{}
	fh.sseMu.Unlock()

	go fh.BroadcastEvents(done)

	// Send an event — slow client should be skipped (default case in select)
	fh.eventCh <- fleet.FleetEvent{Type: "test"}

	// Give broadcast goroutine time to process
	time.Sleep(50 * time.Millisecond)

	// BroadcastEvents should not be stuck — verify by sending another event
	fh.eventCh <- fleet.FleetEvent{Type: "test2"}
	time.Sleep(50 * time.Millisecond)

	close(done)
}

func TestFleetHandler_EmptyListsReturnArrays(t *testing.T) {
	fh, _ := testFleetHandler(t)

	// Empty nodes list should return [] not null
	req := httptest.NewRequest("GET", "/api/v1/fleet/nodes", nil)
	w := httptest.NewRecorder()
	fh.HandleListNodes(w, req)

	if w.Body.String() == "null\n" {
		t.Error("empty nodes list should return [], not null")
	}

	// Empty tokens list
	req2 := httptest.NewRequest("GET", "/api/v1/fleet/tokens", nil)
	req2.Header.Set("Authorization", "Bearer admin-secret")
	w2 := httptest.NewRecorder()
	fh.HandleListTokens(w2, req2)

	if w2.Body.String() == "null\n" {
		t.Error("empty tokens list should return [], not null")
	}
}

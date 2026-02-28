package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func TestNewWSHub(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)
	if hub == nil {
		t.Fatal("NewWSHub returned nil")
	}
}

func TestWSHub_ActiveConnections_Empty(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)
	if hub.ActiveConnections() != 0 {
		t.Errorf("ActiveConnections() = %d, want 0", hub.ActiveConnections())
	}
}

func TestWSHub_RegisterUnregister(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	c := &wsClient{
		send: make(chan []byte, 1),
		done: make(chan struct{}),
	}

	hub.register(c)
	if hub.ActiveConnections() != 1 {
		t.Errorf("after register: ActiveConnections() = %d, want 1", hub.ActiveConnections())
	}

	hub.unregister(c)
	if hub.ActiveConnections() != 0 {
		t.Errorf("after unregister: ActiveConnections() = %d, want 0", hub.ActiveConnections())
	}

	// done channel should be closed after unregister
	select {
	case <-c.done:
		// Good
	default:
		t.Error("done channel should be closed after unregister")
	}
}

func TestWSHub_Broadcast(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	c := &wsClient{
		send: make(chan []byte, 1),
		done: make(chan struct{}),
	}
	hub.register(c)
	defer hub.unregister(c)

	hub.broadcast()

	select {
	case data := <-c.send:
		var report compliance.ComplianceReport
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatalf("broadcast data is not valid compliance report JSON: %v", err)
		}
		if report.Summary.Total != 2 {
			t.Errorf("report total = %d, want 2", report.Summary.Total)
		}
	default:
		t.Error("expected broadcast data on send channel")
	}
}

func TestWSHub_BroadcastSkipsSlowClient(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	// Client with zero-buffer channel — will be skipped
	slow := &wsClient{
		send: make(chan []byte), // unbuffered
		done: make(chan struct{}),
	}
	hub.register(slow)
	defer hub.unregister(slow)

	// Should not block or panic
	hub.broadcast()
}

func TestWSHub_MultipleClients(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	c1 := &wsClient{send: make(chan []byte, 1), done: make(chan struct{})}
	c2 := &wsClient{send: make(chan []byte, 1), done: make(chan struct{})}
	c3 := &wsClient{send: make(chan []byte, 1), done: make(chan struct{})}

	hub.register(c1)
	hub.register(c2)
	hub.register(c3)

	if hub.ActiveConnections() != 3 {
		t.Errorf("ActiveConnections() = %d, want 3", hub.ActiveConnections())
	}

	hub.broadcast()

	// All should receive
	for i, c := range []*wsClient{c1, c2, c3} {
		select {
		case <-c.send:
			// Good
		default:
			t.Errorf("client %d did not receive broadcast", i)
		}
	}

	hub.unregister(c2)
	if hub.ActiveConnections() != 2 {
		t.Errorf("after unregister: ActiveConnections() = %d, want 2", hub.ActiveConnections())
	}

	hub.unregister(c1)
	hub.unregister(c3)
}

func TestWSHub_Run_StopsOnCancel(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hub.Run(ctx, 50*time.Millisecond)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestWSHub_Run_BroadcastsPeriodically(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	c := &wsClient{send: make(chan []byte, 10), done: make(chan struct{})}
	hub.register(c)
	defer hub.unregister(c)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	hub.Run(ctx, 30*time.Millisecond)

	// Should have received multiple broadcasts
	count := 0
	for {
		select {
		case <-c.send:
			count++
		default:
			goto done
		}
	}
done:
	if count < 2 {
		t.Errorf("received %d broadcasts in 150ms at 30ms interval, want >= 2", count)
	}
}

// ---------------------------------------------------------------------------
// HandleWS
// ---------------------------------------------------------------------------

func TestHandleWS_NoUpgrade_Fallback(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	w := httptest.NewRecorder()

	hub.HandleWS(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (fallback), got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["fallback"] != true {
		t.Error("expected fallback=true in response")
	}
	if body["type"] != "compliance" {
		t.Errorf("type = %v, want compliance", body["type"])
	}
	// Should include compliance data
	if body["data"] == nil {
		t.Error("expected data field in fallback response")
	}
}

func TestHandleWS_WithUpgrade_NotImplemented(t *testing.T) {
	handler := NewHandler("", testChecker())
	hub := NewWSHub(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()

	hub.HandleWS(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 (no WS library), got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["alternative"] != "/api/v1/events (SSE)" {
		t.Errorf("alternative = %q, want /api/v1/events (SSE)", body["alternative"])
	}
}

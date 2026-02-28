package fleet

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func testComplianceChecker() *compliance.Checker {
	c := compliance.NewChecker()
	c.AddSection(compliance.Section{
		ID:   "test",
		Name: "Test",
		Items: []compliance.ChecklistItem{
			{ID: "t-1", Name: "Item 1", Status: compliance.StatusPass},
		},
	})
	return c
}

func TestNewReporter_Defaults(t *testing.T) {
	r := NewReporter(ReporterConfig{
		ControllerURL: "http://localhost:8080",
		NodeID:        "node-1",
		APIKey:        "key-1",
		Checker:       testComplianceChecker(),
	})

	if r.interval != 30*time.Second {
		t.Errorf("default interval = %v, want 30s", r.interval)
	}
	if r.nodeID != "node-1" {
		t.Errorf("nodeID = %q, want node-1", r.nodeID)
	}
	if r.controllerURL != "http://localhost:8080" {
		t.Errorf("controllerURL = %q", r.controllerURL)
	}
}

func TestNewReporter_CustomInterval(t *testing.T) {
	r := NewReporter(ReporterConfig{
		ControllerURL: "http://localhost:8080",
		NodeID:        "node-1",
		Checker:       testComplianceChecker(),
		Interval:      10 * time.Second,
	})

	if r.interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s", r.interval)
	}
}

func TestReporter_SendsInitialReport(t *testing.T) {
	var reportCount atomic.Int32
	var heartbeatCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/fleet/report":
			reportCount.Add(1)
			// Verify auth header
			if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
				t.Errorf("auth = %q, want Bearer test-key", auth)
			}
			// Verify body is valid JSON with node_id
			body, _ := io.ReadAll(r.Body)
			var payload ComplianceReportPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("invalid report payload: %v", err)
			}
			if payload.NodeID != "node-1" {
				t.Errorf("payload.NodeID = %q, want node-1", payload.NodeID)
			}
		case "/api/v1/fleet/heartbeat":
			heartbeatCount.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	r := NewReporter(ReporterConfig{
		ControllerURL: server.URL,
		NodeID:        "node-1",
		APIKey:        "test-key",
		Checker:       testComplianceChecker(),
		Interval:      50 * time.Millisecond,
		Logger:        log.New(io.Discard, "", 0),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	r.Run(ctx)

	// Should have sent at least the initial report + 1-2 more
	if got := reportCount.Load(); got < 1 {
		t.Errorf("report count = %d, want >= 1", got)
	}
	// Heartbeats at half interval (25ms), should have several
	if got := heartbeatCount.Load(); got < 1 {
		t.Errorf("heartbeat count = %d, want >= 1", got)
	}
}

func TestReporter_HandlesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	r := NewReporter(ReporterConfig{
		ControllerURL: server.URL,
		NodeID:        "node-1",
		APIKey:        "key",
		Checker:       testComplianceChecker(),
		Interval:      50 * time.Millisecond,
		Logger:        log.New(io.Discard, "", 0),
	})

	// Should not panic on server errors
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r.Run(ctx)
}

func TestReporter_HandlesUnreachableServer(t *testing.T) {
	r := NewReporter(ReporterConfig{
		ControllerURL: "http://127.0.0.1:1",
		NodeID:        "node-1",
		APIKey:        "key",
		Checker:       testComplianceChecker(),
		Interval:      50 * time.Millisecond,
		Logger:        log.New(io.Discard, "", 0),
	})

	// Should not panic on connection refused
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r.Run(ctx)
}

func TestReporter_StopsOnContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	r := NewReporter(ReporterConfig{
		ControllerURL: server.URL,
		NodeID:        "node-1",
		APIKey:        "key",
		Checker:       testComplianceChecker(),
		Interval:      1 * time.Hour, // Long interval so it only sends initial
		Logger:        log.New(io.Discard, "", 0),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// Success: Run returned after cancel
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

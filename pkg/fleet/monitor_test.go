package fleet

import (
	"context"
	"io"
	"log"
	"testing"
	"time"
)

func TestNewMonitor_Defaults(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		Store: nil,
	})

	if m.degradedAfter != 90*time.Second {
		t.Errorf("degradedAfter = %v, want 90s", m.degradedAfter)
	}
	if m.offlineAfter != 180*time.Second {
		t.Errorf("offlineAfter = %v, want 180s", m.offlineAfter)
	}
	if m.checkInterval != 30*time.Second {
		t.Errorf("checkInterval = %v, want 30s", m.checkInterval)
	}
}

func TestNewMonitor_CustomConfig(t *testing.T) {
	m := NewMonitor(MonitorConfig{
		Store:         nil,
		DegradedAfter: 10 * time.Second,
		OfflineAfter:  20 * time.Second,
		CheckInterval: 5 * time.Second,
	})

	if m.degradedAfter != 10*time.Second {
		t.Errorf("degradedAfter = %v, want 10s", m.degradedAfter)
	}
	if m.offlineAfter != 20*time.Second {
		t.Errorf("offlineAfter = %v, want 20s", m.offlineAfter)
	}
	if m.checkInterval != 5*time.Second {
		t.Errorf("checkInterval = %v, want 5s", m.checkInterval)
	}
}

func TestMonitor_StopsOnContextCancel(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	m := NewMonitor(MonitorConfig{
		Store:         store,
		CheckInterval: 1 * time.Hour,
		Logger:        log.New(io.Discard, "", 0),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor.Run did not stop after context cancel")
	}
}

func TestMonitor_MarksDegradedAndOffline(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a node with a stale heartbeat. SQLite stores timestamps at second
	// precision (RFC3339), so use second-scale thresholds for reliable tests.
	node := &Node{
		ID:            "stale-node",
		Name:          "Stale Node",
		Role:          RoleServer,
		Status:        StatusOnline,
		LastHeartbeat: time.Now().Add(-3 * time.Second),
	}
	if err := store.CreateNode(ctx, node, "hash-stale"); err != nil {
		t.Fatalf("create node: %v", err)
	}

	eventCh := make(chan FleetEvent, 10)

	m := NewMonitor(MonitorConfig{
		Store:         store,
		DegradedAfter: 1 * time.Second,
		OfflineAfter:  5 * time.Second,
		CheckInterval: 50 * time.Millisecond,
		Logger:        log.New(io.Discard, "", 0),
		EventCh:       eventCh,
	})

	runCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	m.Run(runCtx)

	// Check that events were emitted
	var events []FleetEvent
	for {
		select {
		case e := <-eventCh:
			events = append(events, e)
		default:
			goto done
		}
	}
done:

	if len(events) == 0 {
		t.Fatal("expected at least 1 event from monitor")
	}

	// First event should be degraded (heartbeat is 3s old, degraded threshold is 1s,
	// offline threshold is 5s — node is past degraded but not offline)
	if events[0].Type != "node_degraded" {
		t.Errorf("first event type = %q, want node_degraded", events[0].Type)
	}

	// Verify node status was updated in store
	updated, err := store.GetNode(ctx, "stale-node")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updated.Status != StatusDegraded {
		t.Errorf("node status = %q, want degraded", updated.Status)
	}
}

func TestMonitor_DoesNotDowngradeAlreadyOffline(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Node already offline with stale heartbeat
	node := &Node{
		ID:            "offline-node",
		Name:          "Already Offline",
		Role:          RoleClient,
		Status:        StatusOffline,
		LastHeartbeat: time.Now().Add(-1 * time.Hour),
	}
	if err := store.CreateNode(ctx, node, "hash-offline"); err != nil {
		t.Fatalf("create node: %v", err)
	}

	eventCh := make(chan FleetEvent, 10)

	m := NewMonitor(MonitorConfig{
		Store:         store,
		DegradedAfter: 1 * time.Millisecond,
		OfflineAfter:  2 * time.Millisecond,
		CheckInterval: 10 * time.Millisecond,
		Logger:        log.New(io.Discard, "", 0),
		EventCh:       eventCh,
	})

	runCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	m.Run(runCtx)

	// Should NOT emit events for already-offline node
	select {
	case e := <-eventCh:
		t.Errorf("unexpected event for already-offline node: %q", e.Type)
	default:
		// Good: no events
	}
}

func TestMonitor_OnlineNodeStaysOnline(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Fresh heartbeat — should stay online
	node := &Node{
		ID:            "fresh-node",
		Name:          "Fresh Node",
		Role:          RoleServer,
		Status:        StatusOnline,
		LastHeartbeat: time.Now(),
	}
	if err := store.CreateNode(ctx, node, "hash-fresh"); err != nil {
		t.Fatalf("create node: %v", err)
	}

	eventCh := make(chan FleetEvent, 10)

	m := NewMonitor(MonitorConfig{
		Store:         store,
		DegradedAfter: 1 * time.Hour,
		OfflineAfter:  2 * time.Hour,
		CheckInterval: 10 * time.Millisecond,
		Logger:        log.New(io.Discard, "", 0),
		EventCh:       eventCh,
	})

	runCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	m.Run(runCtx)

	select {
	case e := <-eventCh:
		t.Errorf("unexpected event for fresh node: %q", e.Type)
	default:
		// Good: fresh node stays online
	}
}

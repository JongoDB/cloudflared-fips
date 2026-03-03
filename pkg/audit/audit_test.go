package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewAuditLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "audit.json")

	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer al.Close()

	// Directory should have been created
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("log dir not created: %v", err)
	}

	// File should exist with restricted permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	perm := info.Mode().Perm()
	if perm&0o007 != 0 {
		t.Errorf("log file world-accessible: %o", perm)
	}
}

func TestLogAndRecentEvents(t *testing.T) {
	dir := t.TempDir()
	al, err := NewAuditLogger(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	events := []AuditEvent{
		{EventType: "system_event", Severity: "info", Actor: "system", Action: "started", Detail: "dashboard started"},
		{EventType: "auth_attempt", Severity: "info", Actor: "api:127.0.0.1", Action: "login_success", Detail: "token auth"},
		{EventType: "compliance_change", Severity: "critical", Actor: "system", Resource: "t-1", Action: "status_changed", Detail: "pass -> fail"},
	}

	for _, e := range events {
		al.Log(e)
	}

	// Verify ring buffer
	recent := al.RecentEvents(10)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(recent))
	}

	// Most recent first
	if recent[0].EventType != "compliance_change" {
		t.Errorf("expected newest first, got %s", recent[0].EventType)
	}

	// Verify timestamps were set
	for _, e := range recent {
		if e.Timestamp == "" {
			t.Error("expected timestamp to be set")
		}
	}

	// Verify JSON-lines file
	data, err := os.ReadFile(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSON lines, got %d", len(lines))
	}

	var parsed AuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("invalid JSON line: %v", err)
	}
	if parsed.EventType != "system_event" {
		t.Errorf("first line event_type = %s, want system_event", parsed.EventType)
	}
}

func TestRecentEventsLimit(t *testing.T) {
	dir := t.TempDir()
	al, err := NewAuditLogger(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	for i := 0; i < 5; i++ {
		al.Log(AuditEvent{EventType: "system_event", Severity: "info", Detail: "event"})
	}

	recent := al.RecentEvents(2)
	if len(recent) != 2 {
		t.Errorf("expected 2 events with limit=2, got %d", len(recent))
	}
}

func TestRecentEventsEmpty(t *testing.T) {
	dir := t.TempDir()
	al, err := NewAuditLogger(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	recent := al.RecentEvents(10)
	if recent != nil {
		t.Errorf("expected nil for empty ring, got %v", recent)
	}
}

func TestHasEvents(t *testing.T) {
	dir := t.TempDir()
	al, err := NewAuditLogger(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	if al.HasEvents() {
		t.Error("expected no events initially")
	}

	al.Log(AuditEvent{EventType: "system_event", Severity: "info"})

	if !al.HasEvents() {
		t.Error("expected events after Log()")
	}
}

func TestAddListener(t *testing.T) {
	dir := t.TempDir()
	al, err := NewAuditLogger(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	var received []AuditEvent
	var mu sync.Mutex
	al.AddListener(func(evt AuditEvent) {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
	})

	al.Log(AuditEvent{EventType: "compliance_change", Severity: "critical", Detail: "test"})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 listener event, got %d", len(received))
	}
	if received[0].Detail != "test" {
		t.Errorf("listener got detail=%q, want %q", received[0].Detail, "test")
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	dir := t.TempDir()
	al, err := NewAuditLogger(filepath.Join(dir, "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	// Fill beyond ring buffer capacity
	for i := 0; i < ringSize+50; i++ {
		al.Log(AuditEvent{
			EventType: "system_event",
			Severity:  "info",
			Detail:    "event",
		})
	}

	// Should still return up to ringSize events
	recent := al.RecentEvents(0) // 0 = all
	if len(recent) != ringSize {
		t.Errorf("expected %d events after wraparound, got %d", ringSize, len(recent))
	}
}

func TestFilePathAndSyslogActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.json")
	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	if al.FilePath() != path {
		t.Errorf("FilePath() = %q, want %q", al.FilePath(), path)
	}

	if al.SyslogActive() {
		t.Error("expected syslog inactive without WithSyslog")
	}
}

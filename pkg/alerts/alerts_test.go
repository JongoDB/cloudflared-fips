package alerts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/audit"
)

func newTestAuditLogger(t *testing.T) *audit.AuditLogger {
	t.Helper()
	al, err := audit.NewAuditLogger(filepath.Join(t.TempDir(), "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { al.Close() })
	return al
}

func TestNewAlertManager(t *testing.T) {
	al := newTestAuditLogger(t)
	am := NewAlertManager(al, nil)
	if am.Configured() {
		t.Error("expected not configured with nil webhooks")
	}
	if am.WebhookCount() != 0 {
		t.Error("expected 0 webhooks")
	}
}

func TestConfiguredWithWebhooks(t *testing.T) {
	al := newTestAuditLogger(t)
	am := NewAlertManager(al, []WebhookConfig{
		{URL: "https://example.com/hook"},
	})
	if !am.Configured() {
		t.Error("expected configured with webhooks")
	}
	if am.WebhookCount() != 1 {
		t.Errorf("expected 1 webhook, got %d", am.WebhookCount())
	}
}

func TestTestWebhooks(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode webhook: %v", err)
		}
		if payload.EventType != "test" {
			t.Errorf("expected test event, got %s", payload.EventType)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	al := newTestAuditLogger(t)
	am := NewAlertManager(al, []WebhookConfig{
		{URL: srv.URL},
	})

	results := am.TestWebhooks()
	if err := results[srv.URL]; err != nil {
		t.Errorf("test webhook failed: %v", err)
	}
	if called.Load() != 1 {
		t.Errorf("expected 1 call, got %d", called.Load())
	}
}

func TestOnEventComplianceChange(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	al := newTestAuditLogger(t)
	am := NewAlertManager(al, []WebhookConfig{
		{URL: srv.URL},
	})
	_ = am // listener already registered

	al.Log(audit.AuditEvent{
		EventType: "compliance_change",
		Severity:  "critical",
		Actor:     "system",
		Resource:  "t-1",
		Action:    "status_changed",
		Detail:    "pass -> fail",
	})

	// Wait for async webhook
	time.Sleep(200 * time.Millisecond)

	if called.Load() != 1 {
		t.Errorf("expected 1 webhook call for compliance_change, got %d", called.Load())
	}
}

func TestCooldownPreventsStorm(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	al := newTestAuditLogger(t)
	am := NewAlertManager(al, []WebhookConfig{
		{URL: srv.URL},
	})
	am.cooldown = 1 * time.Second // short cooldown for test

	// Fire same event twice rapidly
	for i := 0; i < 3; i++ {
		al.Log(audit.AuditEvent{
			EventType: "compliance_change",
			Severity:  "critical",
			Resource:  "t-1",
			Action:    "status_changed",
		})
	}

	time.Sleep(200 * time.Millisecond)

	if called.Load() != 1 {
		t.Errorf("expected 1 webhook (cooldown), got %d", called.Load())
	}
}

func TestEventFilter(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	al := newTestAuditLogger(t)
	am := NewAlertManager(al, []WebhookConfig{
		{URL: srv.URL, Events: []string{"auth_attempt"}},
	})
	_ = am

	// This event type doesn't match the filter
	al.Log(audit.AuditEvent{
		EventType: "compliance_change",
		Severity:  "critical",
		Resource:  "t-2",
		Action:    "status_changed",
	})

	time.Sleep(200 * time.Millisecond)

	if called.Load() != 0 {
		t.Errorf("expected 0 webhook calls (filtered), got %d", called.Load())
	}
}

func TestShouldAlert(t *testing.T) {
	am := &AlertManager{}

	tests := []struct {
		evt  audit.AuditEvent
		want bool
	}{
		{audit.AuditEvent{EventType: "compliance_change"}, true},
		{audit.AuditEvent{EventType: "auth_attempt", Action: "login_failed"}, true},
		{audit.AuditEvent{EventType: "auth_attempt", Action: "login_success"}, false},
		{audit.AuditEvent{EventType: "auth_attempt", Action: "lockout"}, true},
		{audit.AuditEvent{EventType: "credential_lifecycle", Severity: "warning"}, true},
		{audit.AuditEvent{EventType: "credential_lifecycle", Severity: "info"}, false},
		{audit.AuditEvent{EventType: "system_event", Severity: "critical"}, true},
		{audit.AuditEvent{EventType: "system_event", Severity: "info"}, false},
		{audit.AuditEvent{EventType: "api_access"}, false},
	}

	for _, tt := range tests {
		got := am.shouldAlert(tt.evt)
		if got != tt.want {
			t.Errorf("shouldAlert(%s/%s/%s) = %v, want %v",
				tt.evt.EventType, tt.evt.Action, tt.evt.Severity, got, tt.want)
		}
	}
}

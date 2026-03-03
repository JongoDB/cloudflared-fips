// Package alerts provides automated webhook alerting for compliance events.
//
// The AlertManager registers as an audit listener and fires webhooks when
// compliance items change status (CA-7, SI-4 compliance — NIST SP 800-53).
package alerts

import (
	"sync"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/audit"
)

// WebhookConfig defines a webhook notification target.
type WebhookConfig struct {
	URL    string   `json:"url" yaml:"url"`
	Events []string `json:"events" yaml:"events"` // empty = all events
}

// AlertManager dispatches webhook notifications on audit events.
type AlertManager struct {
	webhooks  []WebhookConfig
	auditLog  *audit.AuditLogger
	cooldowns map[string]time.Time // key: event_type+resource, prevents alert storms
	mu        sync.Mutex
	cooldown  time.Duration
}

// NewAlertManager creates an AlertManager and registers it as an audit listener.
// If auditLog is nil, the manager still tracks webhooks but won't receive events.
func NewAlertManager(auditLog *audit.AuditLogger, webhooks []WebhookConfig) *AlertManager {
	am := &AlertManager{
		webhooks:  webhooks,
		auditLog:  auditLog,
		cooldowns: make(map[string]time.Time),
		cooldown:  5 * time.Minute,
	}

	if auditLog != nil {
		auditLog.AddListener(am.onEvent)
	}

	return am
}

// Configured returns true if at least one webhook is configured.
func (am *AlertManager) Configured() bool {
	return len(am.webhooks) > 0
}

// WebhookCount returns the number of configured webhooks.
func (am *AlertManager) WebhookCount() int {
	return len(am.webhooks)
}

// TestWebhooks sends a test payload to all configured webhooks.
// Returns a map of URL → error (nil on success).
func (am *AlertManager) TestWebhooks() map[string]error {
	results := make(map[string]error)
	payload := WebhookPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		EventType: "test",
		Severity:  "info",
		Summary:   "Alert test from cloudflared-fips",
		Detail:    "This is a test alert to verify webhook connectivity.",
	}

	for _, wh := range am.webhooks {
		results[wh.URL] = sendWebhook(wh.URL, payload)
	}

	// Log the test
	if am.auditLog != nil {
		am.auditLog.Log(audit.AuditEvent{
			EventType: "alert_sent",
			Severity:  "info",
			Actor:     "admin",
			Action:    "test_alert",
			Detail:    "Test alert sent to all webhooks",
			NISTRef:   "CA-7, SI-4",
		})
	}

	return results
}

// onEvent is the audit listener callback. It evaluates whether the event
// should trigger a webhook alert.
func (am *AlertManager) onEvent(evt audit.AuditEvent) {
	// Only alert on compliance changes and critical events
	if !am.shouldAlert(evt) {
		return
	}

	// Check cooldown
	cooldownKey := evt.EventType + ":" + evt.Resource
	am.mu.Lock()
	if last, ok := am.cooldowns[cooldownKey]; ok && time.Since(last) < am.cooldown {
		am.mu.Unlock()
		return
	}
	am.cooldowns[cooldownKey] = time.Now()
	am.mu.Unlock()

	payload := WebhookPayload{
		Timestamp: evt.Timestamp,
		EventType: evt.EventType,
		Severity:  evt.Severity,
		Summary:   evt.Action + ": " + evt.Resource,
		Detail:    evt.Detail,
		NISTRef:   evt.NISTRef,
	}

	for _, wh := range am.webhooks {
		if am.matchesFilter(wh, evt) {
			go func(url string) {
				sendWebhookWithRetry(url, payload, 3)
			}(wh.URL)
		}
	}
}

// shouldAlert returns true if the event warrants a webhook notification.
func (am *AlertManager) shouldAlert(evt audit.AuditEvent) bool {
	switch evt.EventType {
	case "compliance_change":
		return true
	case "auth_attempt":
		return evt.Action == "login_failed" || evt.Action == "lockout"
	case "credential_lifecycle":
		return evt.Severity == "warning" || evt.Severity == "critical"
	case "system_event":
		return evt.Severity == "critical"
	default:
		return false
	}
}

// matchesFilter returns true if the webhook should receive this event.
func (am *AlertManager) matchesFilter(wh WebhookConfig, evt audit.AuditEvent) bool {
	if len(wh.Events) == 0 {
		return true // no filter = all events
	}
	for _, e := range wh.Events {
		if e == evt.EventType {
			return true
		}
	}
	return false
}

// Package audit provides structured audit logging for FIPS compliance events.
//
// Every compliance change, API access, configuration modification, and
// authentication attempt is logged as a JSON-lines entry for AU-2, AU-3,
// and AU-6 compliance (NIST SP 800-53).
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent represents a single auditable event.
type AuditEvent struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"event_type"` // compliance_change, api_access, config_change, auth_attempt, credential_lifecycle, alert_sent, system_event
	Severity  string `json:"severity"`   // info, warning, critical
	Actor     string `json:"actor"`      // system, admin, node:<id>, api:<remote_addr>
	Resource  string `json:"resource"`
	Action    string `json:"action"` // status_changed, accessed, modified, login_success, login_failed
	Detail    string `json:"detail"`
	NISTRef   string `json:"nist_ref,omitempty"`
}

// ringSize is the default in-memory ring buffer capacity.
const ringSize = 1000

// syslogWriter abstracts syslog.Writer for cross-platform support.
// On Linux, this is backed by log/syslog; on other platforms it is nil.
type syslogWriter interface {
	Crit(m string) error
	Warning(m string) error
	Info(m string) error
	Close() error
}

// AuditLogger writes audit events to a JSON-lines file and optionally
// forwards to syslog. It also maintains an in-memory ring buffer for
// the /api/v1/audit/events endpoint.
type AuditLogger struct {
	file      *os.File
	mu        sync.Mutex
	syslogW   syslogWriter
	ring      []AuditEvent
	ringIdx   int
	ringFull  bool
	listeners []func(AuditEvent)
}

// Option configures an AuditLogger.
type Option func(*AuditLogger)

// NewAuditLogger creates an audit logger that writes JSON-lines to path.
// The parent directory is created if it doesn't exist.
func NewAuditLogger(path string, opts ...Option) (*AuditLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("audit: create log dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("audit: open %s: %w", path, err)
	}

	al := &AuditLogger{
		file: f,
		ring: make([]AuditEvent, ringSize),
	}
	for _, opt := range opts {
		opt(al)
	}
	return al, nil
}

// Log writes an audit event to the JSON-lines file, the ring buffer,
// syslog (if configured), and notifies all listeners.
func (al *AuditLogger) Log(evt AuditEvent) {
	if evt.Timestamp == "" {
		evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	// Write JSON line
	if al.file != nil {
		buf, err := json.Marshal(evt)
		if err == nil {
			_, _ = al.file.Write(buf)
			_, _ = al.file.Write([]byte("\n"))
		}
	}

	// Ring buffer
	al.ring[al.ringIdx] = evt
	al.ringIdx++
	if al.ringIdx >= ringSize {
		al.ringIdx = 0
		al.ringFull = true
	}

	// Syslog
	if al.syslogW != nil {
		line, _ := json.Marshal(evt)
		switch evt.Severity {
		case "critical":
			_ = al.syslogW.Crit(string(line))
		case "warning":
			_ = al.syslogW.Warning(string(line))
		default:
			_ = al.syslogW.Info(string(line))
		}
	}

	// Notify listeners (non-blocking copies to avoid holding lock)
	listeners := make([]func(AuditEvent), len(al.listeners))
	copy(listeners, al.listeners)

	// Release lock before calling listeners
	al.mu.Unlock()
	for _, fn := range listeners {
		fn(evt)
	}
	al.mu.Lock() // re-acquire for deferred unlock
}

// AddListener registers a callback invoked for every audit event.
// Used by the alert manager to trigger webhooks.
func (al *AuditLogger) AddListener(fn func(AuditEvent)) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.listeners = append(al.listeners, fn)
}

// RecentEvents returns the most recent events from the ring buffer,
// up to limit. Events are returned newest-first.
func (al *AuditLogger) RecentEvents(limit int) []AuditEvent {
	al.mu.Lock()
	defer al.mu.Unlock()

	var total int
	if al.ringFull {
		total = ringSize
	} else {
		total = al.ringIdx
	}
	if total == 0 {
		return nil
	}
	if limit <= 0 || limit > total {
		limit = total
	}

	result := make([]AuditEvent, 0, limit)
	idx := al.ringIdx - 1
	for i := 0; i < limit; i++ {
		if idx < 0 {
			idx = ringSize - 1
		}
		result = append(result, al.ring[idx])
		idx--
	}
	return result
}

// HasEvents returns true if at least one event has been logged.
func (al *AuditLogger) HasEvents() bool {
	al.mu.Lock()
	defer al.mu.Unlock()
	return al.ringFull || al.ringIdx > 0
}

// FilePath returns the path of the audit log file.
func (al *AuditLogger) FilePath() string {
	if al.file == nil {
		return ""
	}
	return al.file.Name()
}

// SyslogActive returns true if syslog forwarding is configured.
func (al *AuditLogger) SyslogActive() bool {
	return al.syslogW != nil
}

// Close flushes and closes the audit log file and syslog connection.
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	var errs []error
	if al.file != nil {
		if err := al.file.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if al.syslogW != nil {
		if err := al.syslogW.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

package dashboard

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/audit"
)

// failedAuth tracks failed authentication attempts per IP for AC-7 lockout.
type failedAuth struct {
	count    int
	lastFail time.Time
}

// AuthMiddleware wraps an http.Handler with Bearer token authentication.
// When token is empty, all requests pass through (localhost-only backward compat).
type AuthMiddleware struct {
	token       string
	auditLog    *audit.AuditLogger
	failTracker map[string]*failedAuth
	mu          sync.Mutex
	next        http.Handler

	maxFails    int           // default: 5
	lockoutDur  time.Duration // default: 5 minutes
	failWindow  time.Duration // default: 5 minutes
}

// AuthConfig configures the auth middleware.
type AuthConfig struct {
	Token    string
	AuditLog *audit.AuditLogger
}

// NewAuthMiddleware creates auth middleware. If token is empty, it passes
// all requests through (open mode for localhost-only deployments).
func NewAuthMiddleware(cfg AuthConfig, next http.Handler) *AuthMiddleware {
	return &AuthMiddleware{
		token:       cfg.Token,
		auditLog:    cfg.AuditLog,
		failTracker: make(map[string]*failedAuth),
		next:        next,
		maxFails:    5,
		lockoutDur:  5 * time.Minute,
		failWindow:  5 * time.Minute,
	}
}

// Enabled returns true if token authentication is active.
func (am *AuthMiddleware) Enabled() bool {
	return am.token != ""
}

func (am *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Open mode: no auth required
	if am.token == "" {
		am.next.ServeHTTP(w, r)
		return
	}

	// Static assets never require auth
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		am.next.ServeHTTP(w, r)
		return
	}

	// Health endpoint is always public
	if r.URL.Path == "/api/v1/health" {
		am.next.ServeHTTP(w, r)
		return
	}

	remoteIP := extractIP(r)

	// Check lockout
	if am.isLockedOut(remoteIP) {
		am.logAudit("auth_attempt", "warning", "api:"+remoteIP, "lockout",
			"Request rejected: IP temporarily locked out (AC-7)")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "too many failed attempts, try again later",
		})
		return
	}

	// Validate token
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "authorization required",
		})
		return
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token != am.token {
		am.recordFailure(remoteIP)
		am.logAudit("auth_attempt", "warning", "api:"+remoteIP, "login_failed",
			"Invalid bearer token")
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "invalid token",
		})
		return
	}

	// Success
	am.clearFailures(remoteIP)
	am.logAudit("auth_attempt", "info", "api:"+remoteIP, "login_success",
		"Bearer token authenticated for "+r.URL.Path)

	am.next.ServeHTTP(w, r)
}

// isLockedOut returns true if the IP has exceeded the failure threshold.
func (am *AuthMiddleware) isLockedOut(ip string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	fa, ok := am.failTracker[ip]
	if !ok {
		return false
	}

	// Reset if window expired
	if time.Since(fa.lastFail) > am.failWindow {
		delete(am.failTracker, ip)
		return false
	}

	return fa.count >= am.maxFails
}

// recordFailure increments the failure count for an IP.
func (am *AuthMiddleware) recordFailure(ip string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	fa, ok := am.failTracker[ip]
	if !ok || time.Since(fa.lastFail) > am.failWindow {
		am.failTracker[ip] = &failedAuth{count: 1, lastFail: time.Now()}
		return
	}
	fa.count++
	fa.lastFail = time.Now()
}

// clearFailures removes the failure record for an IP on successful auth.
func (am *AuthMiddleware) clearFailures(ip string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.failTracker, ip)
}

// HasLockouts returns true if any IP is currently locked out.
func (am *AuthMiddleware) HasLockouts() bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	now := time.Now()
	for _, fa := range am.failTracker {
		if fa.count >= am.maxFails && now.Sub(fa.lastFail) < am.failWindow {
			return true
		}
	}
	return false
}

func (am *AuthMiddleware) logAudit(eventType, severity, actor, action, detail string) {
	if am.auditLog == nil {
		return
	}
	am.auditLog.Log(audit.AuditEvent{
		EventType: eventType,
		Severity:  severity,
		Actor:     actor,
		Action:    action,
		Detail:    detail,
		NISTRef:   "AC-2, AC-3, AC-7",
	})
}

// extractIP gets the remote IP from the request.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

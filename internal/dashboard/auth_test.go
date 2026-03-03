package dashboard

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/pkg/audit"
)

func newTestAudit(t *testing.T) *audit.AuditLogger {
	t.Helper()
	al, err := audit.NewAuditLogger(filepath.Join(t.TempDir(), "audit.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { al.Close() })
	return al
}

func TestAuthMiddleware_OpenMode(t *testing.T) {
	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware(AuthConfig{Token: ""}, inner)
	if mw.Enabled() {
		t.Error("expected not enabled with empty token")
	}

	req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if !called {
		t.Error("expected inner handler to be called in open mode")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_RequiresToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	al := newTestAudit(t)
	mw := NewAuthMiddleware(AuthConfig{Token: "secret123", AuditLog: al}, inner)
	if !mw.Enabled() {
		t.Error("expected enabled with token")
	}

	// No auth header
	req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	al := newTestAudit(t)
	mw := NewAuthMiddleware(AuthConfig{Token: "secret123", AuditLog: al}, inner)

	req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if !called {
		t.Error("expected inner handler called with valid token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	al := newTestAudit(t)
	mw := NewAuthMiddleware(AuthConfig{Token: "secret123", AuditLog: al}, inner)

	req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 with invalid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_HealthPublic(t *testing.T) {
	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware(AuthConfig{Token: "secret123"}, inner)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if !called {
		t.Error("health endpoint should be public")
	}
}

func TestAuthMiddleware_StaticAssetsPublic(t *testing.T) {
	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewAuthMiddleware(AuthConfig{Token: "secret123"}, inner)

	req := httptest.NewRequest("GET", "/index.html", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if !called {
		t.Error("static assets should be public")
	}
}

func TestAuthMiddleware_Lockout(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	al := newTestAudit(t)
	mw := NewAuthMiddleware(AuthConfig{Token: "secret123", AuditLog: al}, inner)

	// Simulate 5 failed attempts
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	// 6th attempt should be locked out
	req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
	req.Header.Set("Authorization", "Bearer secret123") // even with correct token
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after lockout, got %d", w.Code)
	}

	if !mw.HasLockouts() {
		t.Error("expected HasLockouts() to be true")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		xff        string
		want       string
	}{
		{"192.168.1.1:1234", "", "192.168.1.1"},
		{"[::1]:1234", "", "[::1]"},
		{"192.168.1.1:1234", "10.0.0.1, 192.168.1.1", "10.0.0.1"},
	}

	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tt.remoteAddr
		if tt.xff != "" {
			r.Header.Set("X-Forwarded-For", tt.xff)
		}
		got := extractIP(r)
		if got != tt.want {
			t.Errorf("extractIP(remote=%q, xff=%q) = %q, want %q", tt.remoteAddr, tt.xff, got, tt.want)
		}
	}
}

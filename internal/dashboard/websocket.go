// Package dashboard provides HTTP handlers for the compliance dashboard.
//
// WebSocket handler provides an alternative to SSE for real-time compliance
// updates. Useful for environments where SSE proxying is unreliable.
package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// WSHub manages WebSocket connections and broadcasts compliance updates.
type WSHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
	handler *Handler
}

type wsClient struct {
	send chan []byte
	done chan struct{}
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(handler *Handler) *WSHub {
	return &WSHub{
		clients: make(map[*wsClient]struct{}),
		handler: handler,
	}
}

// Run starts the broadcast loop. Call in a goroutine.
func (hub *WSHub) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hub.broadcast()
		}
	}
}

func (hub *WSHub) broadcast() {
	report := hub.handler.Checker.GenerateReport()
	data, err := json.Marshal(report)
	if err != nil {
		return
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for client := range hub.clients {
		select {
		case client.send <- data:
		default:
			// Client too slow, skip this update
		}
	}
}

func (hub *WSHub) register(c *wsClient) {
	hub.mu.Lock()
	hub.clients[c] = struct{}{}
	hub.mu.Unlock()
}

func (hub *WSHub) unregister(c *wsClient) {
	hub.mu.Lock()
	delete(hub.clients, c)
	hub.mu.Unlock()
	close(c.done)
}

// HandleWS handles WebSocket upgrade requests.
// This uses a stdlib-only approach: the handler reads upgrade headers and
// communicates via the response writer. For production use with proper
// WebSocket framing, use nhooyr.io/websocket or gorilla/websocket.
//
// The current implementation falls back to long-polling JSON for maximum
// compatibility without adding an external WebSocket dependency.
func (hub *WSHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	// Check if the client is requesting a WebSocket upgrade
	if r.Header.Get("Upgrade") != "websocket" {
		// Fall back to long-poll: send one compliance snapshot and close
		report := hub.handler.Checker.GenerateReport()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":     "compliance",
			"data":     report,
			"fallback": true,
			"note":     "WebSocket upgrade not requested. Add nhooyr.io/websocket for full WS support.",
		})
		return
	}

	// For full WebSocket support, the nhooyr.io/websocket or gorilla/websocket
	// package would handle the upgrade here. Without an external dependency,
	// we return a helpful error.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error":       "WebSocket upgrade requires nhooyr.io/websocket dependency",
		"install":     "go get nhooyr.io/websocket",
		"alternative": "/api/v1/events (SSE)",
	})
}

// ActiveConnections returns the number of active WebSocket clients.
func (hub *WSHub) ActiveConnections() int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.clients)
}

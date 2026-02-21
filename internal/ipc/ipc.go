// Package ipc provides a Unix domain socket server for CloudSH integration.
//
// CloudSH (or any local process) connects to the socket and sends JSON-RPC
// style requests. This avoids HTTP overhead and works in air-gapped environments
// without needing a TCP port.
//
// Protocol:
//   - Client connects to Unix socket (default: /var/run/cloudflared-fips/compliance.sock)
//   - Client sends a JSON request (newline-terminated)
//   - Server responds with a JSON response (newline-terminated)
//   - Connection stays open for multiple request/response cycles
//
// Request format:
//   {"method": "compliance.status", "id": 1}
//   {"method": "selftest.run", "id": 2}
//   {"method": "manifest.get", "id": 3}
//   {"method": "backend.info", "id": 4}
//   {"method": "migration.status", "id": 5}
//
// Response format:
//   {"id": 1, "result": {...}, "error": null}
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/internal/selftest"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fipsbackend"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/manifest"
)

// DefaultSocketPath is the default Unix socket path.
const DefaultSocketPath = "/var/run/cloudflared-fips/compliance.sock"

// Request represents a JSON-RPC style request from CloudSH.
type Request struct {
	Method string          `json:"method"`
	ID     int             `json:"id"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC style response to CloudSH.
type Response struct {
	ID     int         `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *string     `json:"error"`
}

// Server is the IPC Unix socket server.
type Server struct {
	socketPath   string
	checker      *compliance.Checker
	manifestPath string
	listener     net.Listener
	mu           sync.Mutex
	clients      map[net.Conn]struct{}
}

// NewServer creates a new IPC server.
func NewServer(socketPath string, checker *compliance.Checker, manifestPath string) *Server {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	return &Server{
		socketPath:   socketPath,
		checker:      checker,
		manifestPath: manifestPath,
		clients:      make(map[net.Conn]struct{}),
	}
}

// Start begins listening on the Unix socket. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	// Ensure the socket directory exists
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	// Remove stale socket file
	os.Remove(s.socketPath)

	var err error
	s.listener, err = net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.socketPath, err)
	}

	// Set socket permissions (owner + group only)
	os.Chmod(s.socketPath, 0660)

	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}

		s.mu.Lock()
		s.clients[conn] = struct{}{}
		s.mu.Unlock()

		go s.handleConn(ctx, conn)
	}
}

// SocketPath returns the configured socket path.
func (s *Server) SocketPath() string {
	return s.socketPath
}

// ActiveConnections returns the number of active client connections.
func (s *Server) ActiveConnections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max message

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			errMsg := fmt.Sprintf("invalid request: %v", err)
			resp := Response{Error: &errMsg}
			writeResponse(conn, resp)
			continue
		}

		resp := s.dispatch(req)
		writeResponse(conn, resp)
	}
}

func (s *Server) dispatch(req Request) Response {
	resp := Response{ID: req.ID}

	switch req.Method {
	case "compliance.status":
		report := s.checker.GenerateReport()
		resp.Result = report

	case "selftest.run":
		report, _ := selftest.GenerateReport(buildinfo.Version)
		resp.Result = report

	case "manifest.get":
		m, err := manifest.ReadManifest(s.manifestPath)
		if err != nil {
			errMsg := err.Error()
			resp.Error = &errMsg
		} else {
			resp.Result = m
		}

	case "backend.info":
		resp.Result = fipsbackend.DetectInfo()

	case "migration.status":
		resp.Result = fipsbackend.GetMigrationStatus()

	case "ping":
		resp.Result = map[string]string{
			"status":  "ok",
			"version": buildinfo.Version,
		}

	default:
		errMsg := fmt.Sprintf("unknown method: %s", req.Method)
		resp.Error = &errMsg
	}

	return resp
}

func writeResponse(conn net.Conn, resp Response) {
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	conn.Write(data)
}

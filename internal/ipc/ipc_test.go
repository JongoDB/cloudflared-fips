package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

func testChecker() *compliance.Checker {
	checker := compliance.NewChecker()
	checker.AddSection(compliance.Section{
		ID:   "test-section",
		Name: "Test Section",
		Items: []compliance.ChecklistItem{
			{
				ID:     "t-1",
				Name:   "Test item",
				Status: compliance.StatusPass,
			},
		},
	})
	return checker
}

func startTestServer(t *testing.T) (*Server, string, context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	checker := testChecker()

	// Write a test manifest
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	server := NewServer(socketPath, checker, manifestPath)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	// Wait for server to start
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			return server, socketPath, cancel
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatal("server did not start in time")
	return nil, "", nil
}

func sendRequest(t *testing.T, conn net.Conn, method string, id int) Response {
	t.Helper()
	req := Request{Method: method, ID: id}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write request: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response received")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func TestNewServer(t *testing.T) {
	s := NewServer("", nil, "")
	if s.SocketPath() != DefaultSocketPath {
		t.Errorf("expected default path %q, got %q", DefaultSocketPath, s.SocketPath())
	}

	s2 := NewServer("/custom/path.sock", nil, "")
	if s2.SocketPath() != "/custom/path.sock" {
		t.Errorf("expected /custom/path.sock, got %q", s2.SocketPath())
	}
}

func TestServerPing(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "ping", 1)
	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %q", *resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["status"] != "ok" {
		t.Errorf("expected ok status, got %v", result["status"])
	}
}

func TestServerComplianceStatus(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "compliance.status", 2)
	if resp.Error != nil {
		t.Errorf("expected no error, got %q", *resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestServerManifestGet(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "manifest.get", 3)
	if resp.Error != nil {
		t.Errorf("expected no error, got %q", *resp.Error)
	}
}

func TestServerBackendInfo(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "backend.info", 4)
	if resp.Error != nil {
		t.Errorf("expected no error, got %q", *resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
}

func TestServerMigrationStatus(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "migration.status", 5)
	if resp.Error != nil {
		t.Errorf("expected no error, got %q", *resp.Error)
	}
}

func TestServerUnknownMethod(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "nonexistent.method", 99)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.ID != 99 {
		t.Errorf("expected ID 99, got %d", resp.ID)
	}
}

func TestServerMultipleRequests(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send multiple requests on the same connection
	methods := []string{"ping", "compliance.status", "backend.info"}
	for i, method := range methods {
		resp := sendRequest(t, conn, method, i+1)
		if resp.ID != i+1 {
			t.Errorf("request %d: expected ID %d, got %d", i, i+1, resp.ID)
		}
		if method != "nonexistent" && resp.Error != nil {
			t.Errorf("request %d (%s): unexpected error: %s", i, method, *resp.Error)
		}
	}
}

func TestServerActiveConnections(t *testing.T) {
	server, socketPath, cancel := startTestServer(t)
	defer cancel()

	if server.ActiveConnections() != 0 {
		t.Fatalf("expected 0 connections, got %d", server.ActiveConnections())
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send a request to ensure the connection is fully established
	sendRequest(t, conn, "ping", 1)

	if server.ActiveConnections() != 1 {
		t.Errorf("expected 1 connection, got %d", server.ActiveConnections())
	}

	conn.Close()
	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	if server.ActiveConnections() != 0 {
		t.Errorf("expected 0 connections after close, got %d", server.ActiveConnections())
	}
}

func TestServerInvalidJSON(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON
	conn.Write([]byte("not-json\n"))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response for invalid JSON")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestServerSelfTestRun(t *testing.T) {
	_, socketPath, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRequest(t, conn, "selftest.run", 6)
	if resp.Error != nil {
		t.Errorf("expected no error, got %q", *resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected selftest result")
	}
}

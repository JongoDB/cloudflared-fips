package fleet

import (
	"context"
	"testing"
	"time"
)

func TestEnrollment_CreateAndListTokens(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	token, err := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleServer,
		Region:    "us-east",
		MaxUses:   10,
		ExpiresIn: 3600,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if token.Token == "" {
		t.Error("Token string should be set on creation")
	}
	if token.Role != RoleServer {
		t.Errorf("Role = %q, want server", token.Role)
	}
	if token.MaxUses != 10 {
		t.Errorf("MaxUses = %d, want 10", token.MaxUses)
	}
	if time.Until(token.ExpiresAt) < 59*time.Minute {
		t.Error("ExpiresAt should be ~1 hour from now")
	}

	tokens, err := enrollment.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Errorf("token count = %d, want 1", len(tokens))
	}
}

func TestEnrollment_Enroll(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	// Create a token
	token, err := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleServer,
		Region:    "us-east",
		MaxUses:   2,
		ExpiresIn: 3600,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Enroll a node
	resp, err := enrollment.Enroll(ctx, EnrollmentRequest{
		Token:       token.Token,
		Name:        "server-1",
		Version:     "0.1.0",
		FIPSBackend: "BoringCrypto",
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if resp.NodeID == "" {
		t.Error("NodeID should not be empty")
	}
	if resp.APIKey == "" {
		t.Error("APIKey should not be empty")
	}
	if resp.ReportInterval != 30 {
		t.Errorf("ReportInterval = %d, want 30", resp.ReportInterval)
	}

	// Verify node was created
	node, err := store.GetNode(ctx, resp.NodeID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Name != "server-1" {
		t.Errorf("Name = %q, want server-1", node.Name)
	}
	if node.Role != RoleServer {
		t.Errorf("Role = %q, want server", node.Role)
	}
	if node.Region != "us-east" {
		t.Errorf("Region = %q, want us-east (inherited from token)", node.Region)
	}

	// Verify node auth works
	apiKeyHash := HashToken(resp.APIKey)
	authNode, err := store.GetNodeByAPIKey(ctx, apiKeyHash)
	if err != nil {
		t.Fatalf("GetNodeByAPIKey: %v", err)
	}
	if authNode.ID != resp.NodeID {
		t.Errorf("auth node ID = %q, want %q", authNode.ID, resp.NodeID)
	}
}

func TestEnrollment_InvalidToken(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	_, err := enrollment.Enroll(ctx, EnrollmentRequest{
		Token: "invalid-token",
		Name:  "test",
	})
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestEnrollment_ExpiredToken(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	token, _ := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleClient,
		ExpiresIn: 1,
	})

	// Wait for token to expire
	time.Sleep(2 * time.Second)

	_, err := enrollment.Enroll(ctx, EnrollmentRequest{
		Token: token.Token,
		Name:  "test",
	})
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestEnrollment_MaxUsesExceeded(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	token, _ := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleClient,
		MaxUses:   1,
		ExpiresIn: 3600,
	})

	// First enrollment should succeed
	_, err := enrollment.Enroll(ctx, EnrollmentRequest{
		Token: token.Token,
		Name:  "client-1",
	})
	if err != nil {
		t.Fatalf("first enroll: %v", err)
	}

	// Second should fail
	_, err = enrollment.Enroll(ctx, EnrollmentRequest{
		Token: token.Token,
		Name:  "client-2",
	})
	if err == nil {
		t.Error("expected error for max uses exceeded")
	}
}

func TestEnrollment_ClientReportInterval(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	token, _ := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleClient,
		MaxUses:   1,
		ExpiresIn: 3600,
	})

	resp, err := enrollment.Enroll(ctx, EnrollmentRequest{
		Token: token.Token,
		Name:  "client-1",
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if resp.ReportInterval != 60 {
		t.Errorf("ReportInterval for client = %d, want 60", resp.ReportInterval)
	}
}

func TestEnrollment_DeleteToken(t *testing.T) {
	store := tempDB(t)
	enrollment := NewEnrollment(store)
	ctx := context.Background()

	token, _ := enrollment.CreateToken(ctx, CreateTokenRequest{
		Role:      RoleServer,
		ExpiresIn: 3600,
	})

	if err := enrollment.DeleteToken(ctx, token.ID); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	tokens, _ := enrollment.ListTokens(ctx)
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens after delete, got %d", len(tokens))
	}
}

func TestHashToken(t *testing.T) {
	h1 := HashToken("test-token-1")
	h2 := HashToken("test-token-2")
	h3 := HashToken("test-token-1")

	if h1 == h2 {
		t.Error("different tokens should have different hashes")
	}
	if h1 != h3 {
		t.Error("same token should have same hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 (hex-encoded SHA-256)", len(h1))
	}
}

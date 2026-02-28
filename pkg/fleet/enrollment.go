package fleet

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Enrollment manages token-based zero-trust node enrollment.
type Enrollment struct {
	store Store
}

// NewEnrollment creates a new enrollment manager.
func NewEnrollment(store Store) *Enrollment {
	return &Enrollment{store: store}
}

// CreateToken generates a new enrollment token for the given role and region.
func (e *Enrollment) CreateToken(ctx context.Context, req CreateTokenRequest) (*EnrollmentToken, error) {
	if req.Role == "" {
		return nil, fmt.Errorf("role is required")
	}
	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600 // 1 hour default
	}

	raw, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	tokenID, err := generateSecureToken(16)
	if err != nil {
		return nil, fmt.Errorf("generate token id: %w", err)
	}

	token := &EnrollmentToken{
		ID:        tokenID,
		Token:     raw,
		Role:      req.Role,
		Region:    req.Region,
		ExpiresAt: time.Now().UTC().Add(time.Duration(req.ExpiresIn) * time.Second),
		MaxUses:   req.MaxUses,
		UsedCount: 0,
		CreatedAt: time.Now().UTC(),
	}

	hash := hashToken(raw)
	if err := e.store.CreateToken(ctx, token, hash); err != nil {
		return nil, fmt.Errorf("store token: %w", err)
	}

	return token, nil
}

// ListTokens returns all active enrollment tokens (without raw token values).
func (e *Enrollment) ListTokens(ctx context.Context) ([]EnrollmentToken, error) {
	return e.store.ListTokens(ctx)
}

// DeleteToken removes an enrollment token.
func (e *Enrollment) DeleteToken(ctx context.Context, id string) error {
	return e.store.DeleteToken(ctx, id)
}

// Enroll registers a new node using a valid enrollment token.
// Returns the node ID and API key for subsequent authentication.
func (e *Enrollment) Enroll(ctx context.Context, req EnrollmentRequest) (*EnrollmentResponse, error) {
	if req.Token == "" {
		return nil, fmt.Errorf("enrollment token is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("node name is required")
	}

	// Look up token
	tokenHash := hashToken(req.Token)
	token, err := e.store.GetToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid enrollment token")
	}

	// Validate token
	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, fmt.Errorf("enrollment token has expired")
	}
	if token.MaxUses > 0 && token.UsedCount >= token.MaxUses {
		return nil, fmt.Errorf("enrollment token has reached max uses")
	}

	// Generate node ID and API key
	nodeID, err := generateSecureToken(16)
	if err != nil {
		return nil, fmt.Errorf("generate node id: %w", err)
	}
	apiKey, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	region := req.Region
	if region == "" {
		region = token.Region
	}

	now := time.Now().UTC()
	node := &Node{
		ID:            nodeID,
		Name:          req.Name,
		Role:          token.Role,
		Region:        region,
		Labels:        req.Labels,
		EnrolledAt:    now,
		LastHeartbeat: now,
		Status:        StatusOnline,
		Version:       req.Version,
		FIPSBackend:   req.FIPSBackend,
	}

	apiKeyHash := hashToken(apiKey)
	if err := e.store.CreateNode(ctx, node, apiKeyHash); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	// Increment token usage
	if err := e.store.IncrementTokenUsage(ctx, token.ID); err != nil {
		// Non-fatal: node is already created
		_ = err
	}

	// Report interval varies by role
	interval := 30
	if token.Role == RoleClient {
		interval = 60
	}

	return &EnrollmentResponse{
		NodeID:         nodeID,
		APIKey:         apiKey,
		ReportInterval: interval,
	}, nil
}

// generateSecureToken generates a hex-encoded cryptographic random token.
func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken computes SHA-256 hash of a token for storage.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// HashToken is the exported version for use by HTTP handlers.
func HashToken(token string) string {
	return hashToken(token)
}

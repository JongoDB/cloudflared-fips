package fleet

import (
	"context"
	"time"
)

// Store defines the persistence interface for fleet data.
// The primary implementation uses SQLite (see sqlite.go).
type Store interface {
	// Node operations
	CreateNode(ctx context.Context, node *Node, apiKeyHash string) error
	GetNode(ctx context.Context, id string) (*Node, error)
	ListNodes(ctx context.Context, filter NodeFilter) ([]Node, error)
	UpdateNodeHeartbeat(ctx context.Context, id string, t time.Time) error
	UpdateNodeStatus(ctx context.Context, id string, status NodeStatus) error
	UpdateNodeCompliance(ctx context.Context, id string, pass, fail, warn int) error
	UpdateNodeComplianceStatus(ctx context.Context, id string, status string) error
	DeleteNode(ctx context.Context, id string) error
	GetNodeByAPIKey(ctx context.Context, apiKeyHash string) (*Node, error)

	// Enrollment tokens
	CreateToken(ctx context.Context, token *EnrollmentToken, tokenHash string) error
	GetToken(ctx context.Context, tokenHash string) (*EnrollmentToken, error)
	ListTokens(ctx context.Context) ([]EnrollmentToken, error)
	IncrementTokenUsage(ctx context.Context, id string) error
	DeleteToken(ctx context.Context, id string) error

	// Compliance reports
	StoreReport(ctx context.Context, nodeID string, report []byte) error
	GetLatestReport(ctx context.Context, nodeID string) ([]byte, error)

	// Fleet summary
	GetSummary(ctx context.Context) (*FleetSummary, error)

	// Remediation
	CreateRemediationRequest(ctx context.Context, req *RemediationRequest) error
	GetPendingRemediations(ctx context.Context, nodeID string) ([]RemediationRequest, error)
	CompleteRemediation(ctx context.Context, reqID string, result []byte) error
	GetRemediationRequest(ctx context.Context, id string) (*RemediationRequest, error)

	// Lifecycle
	Close() error
}

// Package fleet provides the multi-node fleet management system for cloudflared-fips.
//
// The fleet system uses a hub-and-spoke model where a controller node
// maintains a registry of all nodes (servers, proxies, clients) and
// aggregates their FIPS compliance status in real time.
package fleet

import (
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
)

// NodeRole defines the role of a fleet node.
type NodeRole string

const (
	RoleController NodeRole = "controller"
	RoleServer     NodeRole = "server"
	RoleProxy      NodeRole = "proxy"
	RoleClient     NodeRole = "client"
)

// NodeStatus represents the operational status of a fleet node.
type NodeStatus string

const (
	StatusOnline   NodeStatus = "online"
	StatusDegraded NodeStatus = "degraded"
	StatusOffline  NodeStatus = "offline"
)

// Node represents a registered fleet node.
type Node struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Role           NodeRole          `json:"role"`
	Region         string            `json:"region"`
	Labels         map[string]string `json:"labels,omitempty"`
	EnrolledAt     time.Time         `json:"enrolled_at"`
	LastHeartbeat  time.Time         `json:"last_heartbeat"`
	Status         NodeStatus        `json:"status"`
	Version        string            `json:"version"`
	FIPSBackend    string            `json:"fips_backend"`
	CompliancePass int               `json:"compliance_pass"`
	ComplianceFail int               `json:"compliance_fail"`
	ComplianceWarn int               `json:"compliance_warn"`
}

// EnrollmentToken is used for zero-trust node enrollment.
type EnrollmentToken struct {
	ID        string    `json:"id"`
	Token     string    `json:"token,omitempty"` // Only set on creation response
	Role      NodeRole  `json:"role"`
	Region    string    `json:"region,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxUses   int       `json:"max_uses"`
	UsedCount int       `json:"used_count"`
	CreatedAt time.Time `json:"created_at"`
}

// EnrollmentRequest is sent by a new node to join the fleet.
type EnrollmentRequest struct {
	Token       string            `json:"token"`
	Name        string            `json:"name"`
	Region      string            `json:"region,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Version     string            `json:"version"`
	FIPSBackend string            `json:"fips_backend"`
}

// EnrollmentResponse is returned after successful enrollment.
type EnrollmentResponse struct {
	NodeID         string `json:"node_id"`
	APIKey         string `json:"api_key"`
	ReportInterval int    `json:"report_interval"` // seconds
}

// ComplianceReportPayload wraps a compliance report with node identity.
type ComplianceReportPayload struct {
	NodeID  string                   `json:"node_id"`
	Report  compliance.ComplianceReport `json:"report"`
	Version string                   `json:"version"`
	Backend string                   `json:"fips_backend"`
}

// HeartbeatRequest is a lightweight keepalive from a node.
type HeartbeatRequest struct {
	NodeID string `json:"node_id"`
}

// FleetSummary provides aggregate fleet statistics.
type FleetSummary struct {
	TotalNodes  int            `json:"total_nodes"`
	Online      int            `json:"online"`
	Degraded    int            `json:"degraded"`
	Offline     int            `json:"offline"`
	ByRole      map[string]int `json:"by_role"`
	ByRegion    map[string]int `json:"by_region"`
	FullyCompliant int         `json:"fully_compliant"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// NodeFilter defines query parameters for listing nodes.
type NodeFilter struct {
	Role   NodeRole   `json:"role,omitempty"`
	Region string     `json:"region,omitempty"`
	Status NodeStatus `json:"status,omitempty"`
}

// FleetEvent is sent via SSE when fleet state changes.
type FleetEvent struct {
	Type string      `json:"type"` // "node_joined", "node_updated", "node_offline", "node_removed"
	Node Node        `json:"node"`
	Time time.Time   `json:"time"`
}

// CreateTokenRequest is the API request body for creating enrollment tokens.
type CreateTokenRequest struct {
	Role      NodeRole `json:"role"`
	Region    string   `json:"region,omitempty"`
	MaxUses   int      `json:"max_uses,omitempty"`
	ExpiresIn int      `json:"expires_in,omitempty"` // seconds, default 3600
}

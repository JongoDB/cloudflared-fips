package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using modernc.org/sqlite (pure Go, no CGO).
// This avoids conflicts with BoringCrypto's CGO requirements.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore opens or creates a SQLite database at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		role           TEXT NOT NULL,
		region         TEXT NOT NULL DEFAULT '',
		labels         TEXT NOT NULL DEFAULT '{}',
		enrolled_at    TEXT NOT NULL,
		last_heartbeat TEXT NOT NULL,
		status         TEXT NOT NULL DEFAULT 'online',
		version        TEXT NOT NULL DEFAULT '',
		fips_backend   TEXT NOT NULL DEFAULT '',
		api_key_hash   TEXT NOT NULL UNIQUE,
		compliance_pass   INTEGER NOT NULL DEFAULT 0,
		compliance_fail   INTEGER NOT NULL DEFAULT 0,
		compliance_warn   INTEGER NOT NULL DEFAULT 0,
		compliance_status TEXT NOT NULL DEFAULT 'unknown',
		service_json      TEXT NOT NULL DEFAULT '',
		grace_period_end  TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS enrollment_tokens (
		id         TEXT PRIMARY KEY,
		token_hash TEXT NOT NULL UNIQUE,
		role       TEXT NOT NULL,
		region     TEXT NOT NULL DEFAULT '',
		expires_at TEXT NOT NULL,
		max_uses   INTEGER NOT NULL DEFAULT 1,
		used_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS compliance_reports (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id   TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		timestamp TEXT NOT NULL,
		report    TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_reports_node_time ON compliance_reports(node_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
	CREATE INDEX IF NOT EXISTS idx_nodes_role ON nodes(role);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// CreateNode inserts a new node into the registry.
func (s *SQLiteStore) CreateNode(ctx context.Context, node *Node, apiKeyHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	labels, _ := json.Marshal(node.Labels)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes (id, name, role, region, labels, enrolled_at, last_heartbeat, status, version, fips_backend, api_key_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.Name, string(node.Role), node.Region, string(labels),
		node.EnrolledAt.UTC().Format(time.RFC3339),
		node.LastHeartbeat.UTC().Format(time.RFC3339),
		string(node.Status), node.Version, node.FIPSBackend, apiKeyHash,
	)
	return err
}

// GetNode retrieves a node by ID.
func (s *SQLiteStore) GetNode(ctx context.Context, id string) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getNodeLocked(ctx, id)
}

func (s *SQLiteStore) getNodeLocked(ctx context.Context, id string) (*Node, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, role, region, labels, enrolled_at, last_heartbeat, status, version, fips_backend, compliance_pass, compliance_fail, compliance_warn, compliance_status, service_json, grace_period_end
		 FROM nodes WHERE id = ?`, id)
	return scanNode(row)
}

// ListNodes returns nodes matching the given filter.
func (s *SQLiteStore) ListNodes(ctx context.Context, filter NodeFilter) ([]Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT id, name, role, region, labels, enrolled_at, last_heartbeat, status, version, fips_backend, compliance_pass, compliance_fail, compliance_warn, compliance_status, service_json, grace_period_end FROM nodes WHERE 1=1"
	var args []interface{}

	if filter.Role != "" {
		query += " AND role = ?"
		args = append(args, string(filter.Role))
	}
	if filter.Region != "" {
		query += " AND region = ?"
		args = append(args, filter.Region)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	query += " ORDER BY enrolled_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		n, err := scanNodeRows(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	return nodes, rows.Err()
}

// UpdateNodeHeartbeat updates the last heartbeat timestamp and sets status to online.
func (s *SQLiteStore) UpdateNodeHeartbeat(ctx context.Context, id string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET last_heartbeat = ?, status = 'online' WHERE id = ?`,
		t.UTC().Format(time.RFC3339), id)
	return err
}

// UpdateNodeStatus updates the node's operational status.
func (s *SQLiteStore) UpdateNodeStatus(ctx context.Context, id string, status NodeStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET status = ? WHERE id = ?`, string(status), id)
	return err
}

// UpdateNodeCompliance updates the node's compliance summary counts.
func (s *SQLiteStore) UpdateNodeCompliance(ctx context.Context, id string, pass, fail, warn int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET compliance_pass = ?, compliance_fail = ?, compliance_warn = ? WHERE id = ?`,
		pass, fail, warn, id)
	return err
}

// UpdateNodeComplianceStatus updates the compliance enforcement status of a node.
func (s *SQLiteStore) UpdateNodeComplianceStatus(ctx context.Context, id string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET compliance_status = ? WHERE id = ?`, status, id)
	return err
}

// DeleteNode removes a node from the registry.
func (s *SQLiteStore) DeleteNode(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id)
	return err
}

// GetNodeByAPIKey looks up a node by its API key hash.
func (s *SQLiteStore) GetNodeByAPIKey(ctx context.Context, apiKeyHash string) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, role, region, labels, enrolled_at, last_heartbeat, status, version, fips_backend, compliance_pass, compliance_fail, compliance_warn, compliance_status, service_json, grace_period_end
		 FROM nodes WHERE api_key_hash = ?`, apiKeyHash)
	return scanNode(row)
}

// CreateToken inserts a new enrollment token.
func (s *SQLiteStore) CreateToken(ctx context.Context, token *EnrollmentToken, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO enrollment_tokens (id, token_hash, role, region, expires_at, max_uses, used_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		token.ID, tokenHash, string(token.Role), token.Region,
		token.ExpiresAt.UTC().Format(time.RFC3339),
		token.MaxUses, token.UsedCount,
		token.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetToken retrieves an enrollment token by its hash.
func (s *SQLiteStore) GetToken(ctx context.Context, tokenHash string) (*EnrollmentToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRowContext(ctx,
		`SELECT id, role, region, expires_at, max_uses, used_count, created_at
		 FROM enrollment_tokens WHERE token_hash = ?`, tokenHash)

	var t EnrollmentToken
	var expiresAt, createdAt string
	err := row.Scan(&t.ID, &t.Role, &t.Region, &expiresAt, &t.MaxUses, &t.UsedCount, &createdAt)
	if err != nil {
		return nil, err
	}
	t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &t, nil
}

// ListTokens returns all enrollment tokens.
func (s *SQLiteStore) ListTokens(ctx context.Context) ([]EnrollmentToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, role, region, expires_at, max_uses, used_count, created_at
		 FROM enrollment_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []EnrollmentToken
	for rows.Next() {
		var t EnrollmentToken
		var expiresAt, createdAt string
		if err := rows.Scan(&t.ID, &t.Role, &t.Region, &expiresAt, &t.MaxUses, &t.UsedCount, &createdAt); err != nil {
			return nil, err
		}
		t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// IncrementTokenUsage increments the used_count for a token.
func (s *SQLiteStore) IncrementTokenUsage(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`UPDATE enrollment_tokens SET used_count = used_count + 1 WHERE id = ?`, id)
	return err
}

// DeleteToken removes an enrollment token.
func (s *SQLiteStore) DeleteToken(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM enrollment_tokens WHERE id = ?`, id)
	return err
}

// StoreReport saves a compliance report JSON for a node.
func (s *SQLiteStore) StoreReport(ctx context.Context, nodeID string, report []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO compliance_reports (node_id, timestamp, report) VALUES (?, ?, ?)`,
		nodeID, time.Now().UTC().Format(time.RFC3339), string(report))
	return err
}

// GetLatestReport returns the most recent compliance report for a node.
func (s *SQLiteStore) GetLatestReport(ctx context.Context, nodeID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var report string
	err := s.db.QueryRowContext(ctx,
		`SELECT report FROM compliance_reports WHERE node_id = ? ORDER BY timestamp DESC LIMIT 1`,
		nodeID).Scan(&report)
	if err != nil {
		return nil, err
	}
	return []byte(report), nil
}

// GetSummary computes fleet-wide aggregate statistics.
func (s *SQLiteStore) GetSummary(ctx context.Context) (*FleetSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &FleetSummary{
		ByRole:   make(map[string]int),
		ByRegion: make(map[string]int),
		UpdatedAt: time.Now().UTC(),
	}

	// Status counts
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM nodes GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		summary.TotalNodes += count
		switch NodeStatus(status) {
		case StatusOnline:
			summary.Online = count
		case StatusDegraded:
			summary.Degraded = count
		case StatusOffline:
			summary.Offline = count
		}
	}

	// Role counts
	rows2, err := s.db.QueryContext(ctx, `SELECT role, COUNT(*) FROM nodes GROUP BY role`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var role string
		var count int
		if err := rows2.Scan(&role, &count); err != nil {
			return nil, err
		}
		summary.ByRole[role] = count
	}

	// Region counts
	rows3, err := s.db.QueryContext(ctx, `SELECT region, COUNT(*) FROM nodes WHERE region != '' GROUP BY region`)
	if err != nil {
		return nil, err
	}
	defer rows3.Close()
	for rows3.Next() {
		var region string
		var count int
		if err := rows3.Scan(&region, &count); err != nil {
			return nil, err
		}
		summary.ByRegion[region] = count
	}

	// Fully compliant (no failures)
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM nodes WHERE compliance_fail = 0 AND status != 'offline'`).Scan(&summary.FullyCompliant)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

// populateNodeExtras fills in the extended node fields from their stored
// string representations after the core fields have been scanned.
func populateNodeExtras(n *Node, compStatus, serviceJSON, gracePeriodEnd string) {
	if compStatus != "" {
		n.ComplianceStatus = NodeComplianceStatus(compStatus)
	} else {
		n.ComplianceStatus = ComplianceUnknown
	}
	if serviceJSON != "" {
		var svc ServiceRegistration
		if json.Unmarshal([]byte(serviceJSON), &svc) == nil && svc.Name != "" {
			n.Service = &svc
		}
	}
	if gracePeriodEnd != "" {
		if t, err := time.Parse(time.RFC3339, gracePeriodEnd); err == nil {
			n.GracePeriodEnd = &t
		}
	}
}

// scanNode scans a single node from a *sql.Row.
func scanNode(row *sql.Row) (*Node, error) {
	var n Node
	var roleStr, statusStr, labelsStr, enrolledAt, lastHB string
	var compStatus, serviceJSON, gracePeriodEnd string
	err := row.Scan(&n.ID, &n.Name, &roleStr, &n.Region, &labelsStr,
		&enrolledAt, &lastHB, &statusStr, &n.Version, &n.FIPSBackend,
		&n.CompliancePass, &n.ComplianceFail, &n.ComplianceWarn,
		&compStatus, &serviceJSON, &gracePeriodEnd)
	if err != nil {
		return nil, err
	}
	n.Role = NodeRole(roleStr)
	n.Status = NodeStatus(statusStr)
	n.EnrolledAt, _ = time.Parse(time.RFC3339, enrolledAt)
	n.LastHeartbeat, _ = time.Parse(time.RFC3339, lastHB)
	json.Unmarshal([]byte(labelsStr), &n.Labels)
	populateNodeExtras(&n, compStatus, serviceJSON, gracePeriodEnd)
	return &n, nil
}

// scanNodeRows scans a single node from *sql.Rows.
func scanNodeRows(rows *sql.Rows) (*Node, error) {
	var n Node
	var roleStr, statusStr, labelsStr, enrolledAt, lastHB string
	var compStatus, serviceJSON, gracePeriodEnd string
	err := rows.Scan(&n.ID, &n.Name, &roleStr, &n.Region, &labelsStr,
		&enrolledAt, &lastHB, &statusStr, &n.Version, &n.FIPSBackend,
		&n.CompliancePass, &n.ComplianceFail, &n.ComplianceWarn,
		&compStatus, &serviceJSON, &gracePeriodEnd)
	if err != nil {
		return nil, err
	}
	n.Role = NodeRole(roleStr)
	n.Status = NodeStatus(statusStr)
	n.EnrolledAt, _ = time.Parse(time.RFC3339, enrolledAt)
	n.LastHeartbeat, _ = time.Parse(time.RFC3339, lastHB)
	json.Unmarshal([]byte(labelsStr), &n.Labels)
	populateNodeExtras(&n, compStatus, serviceJSON, gracePeriodEnd)
	return &n, nil
}

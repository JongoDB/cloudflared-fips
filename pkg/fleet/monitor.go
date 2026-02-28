package fleet

import (
	"context"
	"log"
	"time"
)

// Monitor periodically checks for stale nodes and marks them degraded or offline.
type Monitor struct {
	store          Store
	degradedAfter  time.Duration
	offlineAfter   time.Duration
	checkInterval  time.Duration
	logger         *log.Logger
	eventCh        chan<- FleetEvent
}

// MonitorConfig holds configuration for the fleet monitor.
type MonitorConfig struct {
	Store         Store
	DegradedAfter time.Duration // Time without heartbeat before "degraded" (default 90s)
	OfflineAfter  time.Duration // Time without heartbeat before "offline" (default 180s)
	CheckInterval time.Duration // How often to check (default 30s)
	Logger        *log.Logger
	EventCh       chan<- FleetEvent
}

// NewMonitor creates a stale-node monitor.
func NewMonitor(cfg MonitorConfig) *Monitor {
	if cfg.DegradedAfter == 0 {
		cfg.DegradedAfter = 90 * time.Second
	}
	if cfg.OfflineAfter == 0 {
		cfg.OfflineAfter = 180 * time.Second
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Monitor{
		store:         cfg.Store,
		degradedAfter: cfg.DegradedAfter,
		offlineAfter:  cfg.OfflineAfter,
		checkInterval: cfg.CheckInterval,
		logger:        cfg.Logger,
		eventCh:       cfg.EventCh,
	}
}

// Run starts the monitor loop. Blocks until context is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check(ctx)
		}
	}
}

func (m *Monitor) check(ctx context.Context) {
	nodes, err := m.store.ListNodes(ctx, NodeFilter{})
	if err != nil {
		m.logger.Printf("fleet monitor: list nodes error: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, node := range nodes {
		elapsed := now.Sub(node.LastHeartbeat)

		var newStatus NodeStatus
		if elapsed > m.offlineAfter && node.Status != StatusOffline {
			newStatus = StatusOffline
		} else if elapsed > m.degradedAfter && node.Status == StatusOnline {
			newStatus = StatusDegraded
		}

		if newStatus != "" {
			if err := m.store.UpdateNodeStatus(ctx, node.ID, newStatus); err != nil {
				m.logger.Printf("fleet monitor: update node %s status: %v", node.ID, err)
				continue
			}
			node.Status = newStatus
			if m.eventCh != nil {
				select {
				case m.eventCh <- FleetEvent{
					Type: "node_" + string(newStatus),
					Node: node,
					Time: now,
				}:
				default:
					// Don't block if channel is full
				}
			}
		}
	}
}

package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cloudflared-fips/cloudflared-fips/internal/compliance"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/buildinfo"
	"github.com/cloudflared-fips/cloudflared-fips/pkg/fipsbackend"
)

// Reporter periodically pushes compliance reports and heartbeats to the controller.
type Reporter struct {
	controllerURL string
	nodeID        string
	apiKey        string
	checker       *compliance.Checker
	interval      time.Duration
	logger        *log.Logger
	client        *http.Client
}

// ReporterConfig holds configuration for the fleet reporter.
type ReporterConfig struct {
	ControllerURL string
	NodeID        string
	APIKey        string
	Checker       *compliance.Checker
	Interval      time.Duration
	Logger        *log.Logger
}

// NewReporter creates a new fleet reporter.
func NewReporter(cfg ReporterConfig) *Reporter {
	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Reporter{
		controllerURL: cfg.ControllerURL,
		nodeID:        cfg.NodeID,
		apiKey:        cfg.APIKey,
		checker:       cfg.Checker,
		interval:      cfg.Interval,
		logger:        cfg.Logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Run starts the reporter loop. It pushes a full compliance report at the
// configured interval and a lightweight heartbeat at half that interval.
// Blocks until the context is cancelled.
func (r *Reporter) Run(ctx context.Context) {
	reportTicker := time.NewTicker(r.interval)
	heartbeatTicker := time.NewTicker(r.interval / 2)
	defer reportTicker.Stop()
	defer heartbeatTicker.Stop()

	// Send initial report immediately
	r.sendReport(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-reportTicker.C:
			r.sendReport(ctx)
		case <-heartbeatTicker.C:
			r.sendHeartbeat(ctx)
		}
	}
}

func (r *Reporter) sendReport(ctx context.Context) {
	report := r.checker.GenerateReport()
	info := fipsbackend.DetectInfo()

	payload := ComplianceReportPayload{
		NodeID:  r.nodeID,
		Report:  *report,
		Version: buildinfo.Version,
		Backend: info.Name,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		r.logger.Printf("fleet reporter: marshal error: %v", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/fleet/report", r.controllerURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		r.logger.Printf("fleet reporter: request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Printf("fleet reporter: report push failed: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Printf("fleet reporter: report push returned %d", resp.StatusCode)
	}
}

func (r *Reporter) sendHeartbeat(ctx context.Context) {
	payload := HeartbeatRequest{NodeID: r.nodeID}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/api/v1/fleet/heartbeat", r.controllerURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Printf("fleet reporter: heartbeat failed: %v", err)
		return
	}
	resp.Body.Close()
}

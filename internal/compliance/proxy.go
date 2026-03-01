package compliance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProxyStatsChecker fetches client TLS inspection stats from a fips-proxy
// instance and produces a "Gateway Clients" compliance section.
// This bridges the gap where clients connect to fips-proxy (:443) but the
// dashboard runs on a separate port (:8080) and cannot see ClientHello data.
type ProxyStatsChecker struct {
	proxyAddr  string
	httpClient *http.Client
}

// proxyClientsResponse matches the JSON returned by fips-proxy GET /api/v1/clients.
type proxyClientsResponse struct {
	Summary proxySummary `json:"summary"`
}

type proxySummary struct {
	Total       int `json:"total"`
	FIPSCapable int `json:"fips_capable"`
	NonFIPS     int `json:"non_fips"`
}

// NewProxyStatsChecker creates a checker that polls the fips-proxy metrics endpoint.
// proxyAddr should be the dashboard/metrics address of the proxy (e.g., "localhost:8081").
func NewProxyStatsChecker(proxyAddr string) *ProxyStatsChecker {
	return &ProxyStatsChecker{
		proxyAddr: proxyAddr,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// RunGatewayClientChecks produces the "Gateway Clients" compliance section
// by fetching TLS inspection data from the fips-proxy.
func (p *ProxyStatsChecker) RunGatewayClientChecks() Section {
	section := Section{
		ID:          "gateway",
		Name:        "Gateway Clients",
		Description: "Client TLS inspection via per-site FIPS gateway proxy",
	}

	stats, err := p.fetchStats()
	if err != nil {
		// Fail closed: unreachable proxy = unknown state
		section.Items = append(section.Items, ChecklistItem{
			ID:                 "gw-1",
			Name:               "Gateway Proxy Reachable",
			Status:             StatusUnknown,
			Severity:           "high",
			VerificationMethod: VerifyProbe,
			What:               fmt.Sprintf("Cannot reach FIPS gateway proxy at %s: %v", p.proxyAddr, err),
			Why:                "The gateway proxy provides client TLS inspection. If unreachable, client FIPS posture is unknown.",
			Remediation:        "Verify fips-proxy is running and --dashboard-addr matches --proxy-addr on the dashboard.",
			NISTRef:            "SC-8, SC-13",
		})
		section.Items = append(section.Items, unknownGatewayItem("gw-2", "FIPS-Capable Clients"))
		section.Items = append(section.Items, unknownGatewayItem("gw-3", "Non-FIPS Clients"))
		section.Items = append(section.Items, unknownGatewayItem("gw-4", "Last Connection"))
		return section
	}

	// Item 1: Total client connections
	section.Items = append(section.Items, ChecklistItem{
		ID:                 "gw-1",
		Name:               "Gateway Client Connections",
		Status:             StatusPass,
		Severity:           "medium",
		VerificationMethod: VerifyProbe,
		What:               fmt.Sprintf("Total client TLS connections inspected: %d", stats.Total),
		Why:                "Tracks volume of clients connecting through the FIPS gateway.",
		Remediation:        "No action needed — informational metric.",
		NISTRef:            "SC-8",
	})

	// Item 2: FIPS-capable clients
	fipsStatus := StatusPass
	var fipsWhat string
	if stats.Total > 0 {
		pct := float64(stats.FIPSCapable) / float64(stats.Total) * 100
		fipsWhat = fmt.Sprintf("%d of %d clients are FIPS-capable (%.0f%%)", stats.FIPSCapable, stats.Total, pct)
		if pct < 100 {
			fipsStatus = StatusWarning
		}
	} else {
		fipsWhat = "No client connections recorded yet"
		fipsStatus = StatusUnknown
	}
	section.Items = append(section.Items, ChecklistItem{
		ID:                 "gw-2",
		Name:               "FIPS-Capable Clients",
		Status:             fipsStatus,
		Severity:           "high",
		VerificationMethod: VerifyProbe,
		What:               fipsWhat,
		Why:                "Non-FIPS clients may negotiate weak cipher suites, breaking end-to-end FIPS compliance.",
		Remediation:        "Enable OS FIPS mode on client devices. See docs/client-hardening-guide.md.",
		NISTRef:            "SC-13, IA-7",
	})

	// Item 3: Non-FIPS clients (warning if any)
	nonFIPSStatus := StatusPass
	nonFIPSWhat := "No non-FIPS clients detected"
	if stats.NonFIPS > 0 {
		nonFIPSStatus = StatusWarning
		nonFIPSWhat = fmt.Sprintf("%d non-FIPS client connections detected — these clients offered non-FIPS cipher suites", stats.NonFIPS)
	}
	if stats.Total == 0 {
		nonFIPSStatus = StatusUnknown
		nonFIPSWhat = "No client connections recorded yet"
	}
	section.Items = append(section.Items, ChecklistItem{
		ID:                 "gw-3",
		Name:               "Non-FIPS Clients",
		Status:             nonFIPSStatus,
		Severity:           "high",
		VerificationMethod: VerifyProbe,
		What:               nonFIPSWhat,
		Why:                "Non-FIPS clients indicate endpoints that haven't been hardened. Each represents a compliance gap in the client segment.",
		Remediation:        "Enforce FIPS mode via MDM policy. Block non-FIPS clients with Cloudflare Access device posture rules.",
		NISTRef:            "SC-13, CM-6",
	})

	// Item 4: Gateway proxy status (already confirmed reachable)
	section.Items = append(section.Items, ChecklistItem{
		ID:                 "gw-4",
		Name:               "Gateway Proxy Active",
		Status:             StatusPass,
		Severity:           "high",
		VerificationMethod: VerifyProbe,
		What:               fmt.Sprintf("FIPS gateway proxy at %s is operational", p.proxyAddr),
		Why:                "The gateway proxy provides FIPS TLS termination for all client connections at this site.",
		Remediation:        "Restart fips-proxy service if this check fails.",
		NISTRef:            "SC-8, CP-8",
	})

	return section
}

// fetchStats queries the fips-proxy /api/v1/clients endpoint.
func (p *ProxyStatsChecker) fetchStats() (*proxySummary, error) {
	resp, err := p.httpClient.Get(fmt.Sprintf("http://%s/api/v1/clients", p.proxyAddr))
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result proxyClientsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	return &result.Summary, nil
}

// unknownGatewayItem creates a placeholder item when the proxy is unreachable.
func unknownGatewayItem(id, name string) ChecklistItem {
	return ChecklistItem{
		ID:                 id,
		Name:               name,
		Status:             StatusUnknown,
		Severity:           "high",
		VerificationMethod: VerifyProbe,
		What:               "Cannot determine — gateway proxy unreachable",
		Why:                "Requires connection to fips-proxy metrics endpoint.",
		Remediation:        "Verify fips-proxy is running and accessible.",
		NISTRef:            "SC-8, SC-13",
	}
}

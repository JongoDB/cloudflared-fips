package wizard

import (
	"strings"
	"testing"

	"github.com/cloudflared-fips/cloudflared-fips/internal/tui/config"
)

// ---------------------------------------------------------------------------
// last4
// ---------------------------------------------------------------------------

func TestLast4(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcdefghij", "ghij"},
		{"12345678", "5678"},
		{"abcd", "****"},
		{"abc", "***"},
		{"ab", "**"},
		{"a", "*"},
		{"", ""},
		{"secret-api-token-xyz", "-xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := last4(tt.input)
			if got != tt.want {
				t.Errorf("last4(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// displayMDM
// ---------------------------------------------------------------------------

func TestDisplayMDM(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"intune", "Microsoft Intune"},
		{"jamf", "Jamf Pro"},
		{"none", "None"},
		{"", "None"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := displayMDM(tt.provider)
			if got != tt.want {
				t.Errorf("displayMDM(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// renderProgress
// ---------------------------------------------------------------------------

func TestRenderProgress(t *testing.T) {
	tests := []struct {
		current int
		total   int
		title   string
		wants   []string // substrings that should appear
	}{
		{1, 3, "Tunnel", []string{"Step 1 of 3", "Tunnel"}},
		{2, 5, "Dashboard", []string{"Step 2 of 5", "Dashboard"}},
		{3, 3, "Review", []string{"Step 3 of 3", "Review"}},
	}

	for _, tt := range tests {
		got := renderProgress(tt.current, tt.total, tt.title)
		for _, want := range tt.wants {
			if !strings.Contains(got, want) {
				t.Errorf("renderProgress(%d, %d, %q) missing %q in output: %q",
					tt.current, tt.total, tt.title, want, got)
			}
		}
	}
}

func TestRenderProgress_DotCount(t *testing.T) {
	got := renderProgress(2, 4, "Test")
	// Should contain both ● (filled) and ○ (empty) dot characters
	if !strings.Contains(got, "●") {
		t.Error("renderProgress should contain ● (filled dot)")
	}
	if !strings.Contains(got, "○") {
		t.Error("renderProgress should contain ○ (empty dot)")
	}
}

// ---------------------------------------------------------------------------
// renderNavHints
// ---------------------------------------------------------------------------

func TestRenderNavHints_First(t *testing.T) {
	got := renderNavHints(true, false)
	if strings.Contains(got, "Back") {
		t.Error("first page should not show Back hint")
	}
	if !strings.Contains(got, "Next") {
		t.Error("non-last page should show Next hint")
	}
	if !strings.Contains(got, "Quit") {
		t.Error("should always show Quit hint")
	}
}

func TestRenderNavHints_Middle(t *testing.T) {
	got := renderNavHints(false, false)
	if !strings.Contains(got, "Back") {
		t.Error("middle page should show Back hint")
	}
	if !strings.Contains(got, "Next") {
		t.Error("middle page should show Next hint")
	}
}

func TestRenderNavHints_Last(t *testing.T) {
	got := renderNavHints(false, true)
	if !strings.Contains(got, "Back") {
		t.Error("last page should show Back hint")
	}
	if !strings.Contains(got, "Write Config") {
		t.Error("last page should show Write Config hint")
	}
	if strings.Contains(got, "Tab/Enter=Next") {
		t.Error("last page should not show Next hint")
	}
}

func TestRenderNavHints_FirstAndLast(t *testing.T) {
	got := renderNavHints(true, true)
	if strings.Contains(got, "Back") {
		t.Error("first+last should not show Back")
	}
	if !strings.Contains(got, "Write Config") {
		t.Error("last should show Write Config")
	}
}

// ---------------------------------------------------------------------------
// parseInt
// ---------------------------------------------------------------------------

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"0", 0},
		{"9999", 9999},
		{"", 0},
		{"abc", 0},
		{"12abc", 0},
		{"abc12", 0},
		{"42", 42},
		{"1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInt(tt.input)
			if got != tt.want {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// renderSummary
// ---------------------------------------------------------------------------

func TestRenderSummary_NilConfig(t *testing.T) {
	p := NewReviewPage()
	got := p.renderSummary()
	if !strings.Contains(got, "no configuration") {
		t.Errorf("nil config should show 'no configuration', got: %q", got)
	}
}

func TestRenderSummary_StandardTier(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Tunnel:          "abc-123-uuid",
		CredentialsFile: "/home/user/.cloudflared/abc.json",
		Protocol:        "quic",
		Ingress: []config.IngressRule{
			{Hostname: "app.example.com", Service: "https://localhost:8443"},
		},
		DeploymentTier: "standard",
		Dashboard: config.DashboardConfig{
			CFAPIToken:     "secret-token-value",
			ZoneID:         "zone-123",
			AccountID:      "acc-456",
			TunnelID:       "tun-789",
			MetricsAddress: "localhost:9090",
			MDM:            config.MDMConfig{Provider: "none"},
		},
		FIPS: config.FIPSConfig{
			SelfTestOnStart:      true,
			FailOnSelfTestFailure: true,
			VerifySignature:      false,
			SelfTestOutput:       "/var/log/cloudflared/selftest.json",
		},
	}

	got := p.renderSummary()

	// Tunnel section
	if !strings.Contains(got, "TUNNEL") {
		t.Error("should contain TUNNEL section header")
	}
	if !strings.Contains(got, "abc-123-uuid") {
		t.Error("should show tunnel UUID")
	}
	if !strings.Contains(got, "quic") {
		t.Error("should show protocol")
	}
	if !strings.Contains(got, "app.example.com") {
		t.Error("should show ingress hostname")
	}

	// Dashboard section — token should be masked
	if !strings.Contains(got, "DASHBOARD") {
		t.Error("should contain DASHBOARD WIRING section")
	}
	if strings.Contains(got, "secret-token-value") {
		t.Error("API token should be masked, not shown in cleartext")
	}
	if !strings.Contains(got, "****") {
		t.Error("API token should show masked form ****")
	}

	// Deployment tier
	if !strings.Contains(got, "DEPLOYMENT TIER") {
		t.Error("should contain DEPLOYMENT TIER section")
	}
	if !strings.Contains(got, "standard") {
		t.Error("should show tier value")
	}

	// FIPS options
	if !strings.Contains(got, "FIPS OPTIONS") {
		t.Error("should contain FIPS OPTIONS section")
	}
	if !strings.Contains(got, "true") {
		t.Error("should show self-test on start = true")
	}
}

func TestRenderSummary_Tier2Fields(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		DeploymentTier:   "regional_keyless",
		KeylessSSLHost:   "keyless.example.com",
		KeylessSSLPort:   2407,
		RegionalServices: true,
		Dashboard:        config.DashboardConfig{MDM: config.MDMConfig{Provider: "none"}},
		FIPS:             config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "keyless.example.com") {
		t.Error("tier 2 should show Keyless SSL Host")
	}
	if !strings.Contains(got, "2407") {
		t.Error("tier 2 should show Keyless SSL Port")
	}
}

func TestRenderSummary_Tier3Fields(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		DeploymentTier:  "self_hosted",
		ProxyListenAddr: "0.0.0.0:443",
		ProxyCertFile:   "/etc/pki/tls/proxy.pem",
		ProxyKeyFile:    "/etc/pki/tls/proxy-key.pem",
		ProxyUpstream:   "https://app.internal:8443",
		Dashboard:       config.DashboardConfig{MDM: config.MDMConfig{Provider: "none"}},
		FIPS:            config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "0.0.0.0:443") {
		t.Error("tier 3 should show Proxy Listen address")
	}
	if !strings.Contains(got, "proxy.pem") {
		t.Error("tier 3 should show Proxy Cert")
	}
	if !strings.Contains(got, "app.internal") {
		t.Error("tier 3 should show Proxy Upstream")
	}
}

func TestRenderSummary_MDMIntune(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		DeploymentTier: "standard",
		Dashboard: config.DashboardConfig{
			MDM: config.MDMConfig{
				Provider:     "intune",
				TenantID:     "tenant-abc",
				ClientID:     "client-xyz",
				ClientSecret: "super-secret-value",
			},
		},
		FIPS: config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "Microsoft Intune") {
		t.Error("should display 'Microsoft Intune' for intune provider")
	}
	if !strings.Contains(got, "tenant-abc") {
		t.Error("should show Intune Tenant ID")
	}
	// Client secret should be masked
	if strings.Contains(got, "super-secret-value") {
		t.Error("client secret should be masked")
	}
}

func TestRenderSummary_MDMJamf(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		DeploymentTier: "standard",
		Dashboard: config.DashboardConfig{
			MDM: config.MDMConfig{
				Provider: "jamf",
				BaseURL:  "https://jamf.example.com",
				APIToken: "jamf-api-token-12345",
			},
		},
		FIPS: config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "Jamf Pro") {
		t.Error("should display 'Jamf Pro' for jamf provider")
	}
	if !strings.Contains(got, "jamf.example.com") {
		t.Error("should show Jamf Base URL")
	}
	// API token should be masked
	if strings.Contains(got, "jamf-api-token-12345") {
		t.Error("API token should be masked")
	}
}

func TestRenderSummary_CatchAllIngress(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Ingress: []config.IngressRule{
			{Hostname: "", Service: "http_status:404"},
		},
		Dashboard: config.DashboardConfig{MDM: config.MDMConfig{Provider: "none"}},
		FIPS:      config.FIPSConfig{},
	}

	got := p.renderSummary()
	// Catch-all rules have empty hostname → show "* →"
	if !strings.Contains(got, "* →") {
		t.Error("catch-all ingress should show '* →'")
	}
}

// ---------------------------------------------------------------------------
// NewWizardModel
// ---------------------------------------------------------------------------

func TestNewWizardModel(t *testing.T) {
	m := NewWizardModel()
	if len(m.pages) != 5 {
		t.Errorf("expected 5 pages, got %d", len(m.pages))
	}
	if m.pageIndex != 0 {
		t.Errorf("pageIndex = %d, want 0", m.pageIndex)
	}
	if m.config == nil {
		t.Error("config should not be nil")
	}
	if m.done {
		t.Error("should not be done initially")
	}

	// Verify page titles
	expectedTitles := []string{"Tunnel Configuration", "Dashboard Wiring", "Deployment Tier", "FIPS Options", "Review & Write"}
	for i, want := range expectedTitles {
		got := m.pages[i].Title()
		if got != want {
			t.Errorf("page %d title = %q, want %q", i, got, want)
		}
	}
}

func TestNewWizardModel_View(t *testing.T) {
	m := NewWizardModel()
	m.width = 100
	m.height = 40
	for _, p := range m.pages {
		p.SetSize(96, 32)
	}
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
	if !strings.Contains(view, "cloudflared-fips") {
		t.Error("View should contain product name")
	}
	if !strings.Contains(view, "Step 1 of 5") {
		t.Error("View should show step progress")
	}
}

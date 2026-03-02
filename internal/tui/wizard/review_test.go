package wizard

import (
	"os"
	"runtime"
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
	if !strings.Contains(got, "Provision") {
		t.Error("last page should show Review & Provision hint")
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
	if !strings.Contains(got, "Provision") {
		t.Error("last should show Review & Provision")
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
// tierNumber
// ---------------------------------------------------------------------------

func TestTierNumber(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"standard", "1"},
		{"regional_keyless", "2"},
		{"self_hosted", "3"},
		{"", "1"},
		{"unknown", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tierNumber(tt.input)
			if got != tt.want {
				t.Errorf("tierNumber(%q) = %q, want %q", tt.input, got, tt.want)
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

func TestRenderSummary_ServerRole(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:            "server",
		DeploymentTier:  "standard",
		ServiceName:     "internal-api",
		ServiceHost:     "0.0.0.0",
		ServicePort:     8443,
		ServiceTLS:      true,
		ControllerURL:   "https://ctrl.example.com:8080",
		EnrollmentToken: "tok-secret-enrollment",
		FIPS: config.FIPSConfig{
			SelfTestOnStart:       true,
			FailOnSelfTestFailure: true,
			VerifySignature:       false,
			SelfTestOutput:        "/var/log/cloudflared/selftest.json",
		},
	}

	got := p.renderSummary()

	// Role & Tier section
	if !strings.Contains(got, "ROLE & TIER") {
		t.Error("should contain ROLE & TIER section")
	}
	if !strings.Contains(got, "server") {
		t.Error("should show role")
	}

	// Origin Service section (replaces TUNNEL for server)
	if !strings.Contains(got, "ORIGIN SERVICE") {
		t.Error("should contain ORIGIN SERVICE section for server role")
	}
	if !strings.Contains(got, "internal-api") {
		t.Error("should show service name")
	}
	if !strings.Contains(got, "8443") {
		t.Error("should show service port")
	}

	// Fleet enrollment section
	if !strings.Contains(got, "FLEET ENROLLMENT") {
		t.Error("should contain FLEET ENROLLMENT section for server role")
	}
	if !strings.Contains(got, "ctrl.example.com") {
		t.Error("should show controller URL")
	}

	// Server should NOT show TUNNEL section
	if strings.Contains(got, "TUNNEL") {
		t.Error("server role should not show TUNNEL section")
	}

	// Server should NOT show DASHBOARD WIRING (controller-only now)
	if strings.Contains(got, "DASHBOARD WIRING") {
		t.Error("server role should not show DASHBOARD WIRING section")
	}

	// FIPS options
	if !strings.Contains(got, "FIPS OPTIONS") {
		t.Error("should contain FIPS OPTIONS section")
	}
	if !strings.Contains(got, "true") {
		t.Error("should show self-test on start = true")
	}

	// Provision command
	if !strings.Contains(got, "PROVISION COMMAND") {
		t.Error("should contain PROVISION COMMAND section")
	}
	if !strings.Contains(got, "--role server") {
		t.Error("provision command should include --role server")
	}
}

func TestRenderSummary_ControllerRole(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:           "controller",
		AdminKey:       "super-secret-admin-key",
		TunnelToken:    "eyJhbGciOiJFUzI1NiIsImtpZCI6InRlc3QifQ",
		Protocol:       "quic",
		DeploymentTier: "standard",
		Ingress: []config.IngressRule{
			{Hostname: "app.example.com", Service: "https://localhost:8443"},
			{Service: "http_status:404"},
		},
		CompliancePolicy: config.CompliancePolicyConfig{
			EnforcementMode: "enforce",
			RequireOSFIPS:   true,
			RequireDiskEnc:  true,
		},
		Dashboard: config.DashboardConfig{
			CFAPIToken: "cf-api-token-secret",
			MDM:        config.MDMConfig{Provider: "none"},
		},
		FIPS: config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "CONTROLLER") {
		t.Error("should contain CONTROLLER section")
	}
	if strings.Contains(got, "super-secret-admin-key") {
		t.Error("admin key should be masked")
	}
	if !strings.Contains(got, "****") {
		t.Error("admin key should show masked form")
	}

	// Controller should have TUNNEL section
	if !strings.Contains(got, "TUNNEL") {
		t.Error("controller should contain TUNNEL section")
	}
	if !strings.Contains(got, "quic") {
		t.Error("should show protocol")
	}
	if !strings.Contains(got, "app.example.com") {
		t.Error("should show ingress hostname")
	}

	// Compliance policy
	if !strings.Contains(got, "COMPLIANCE POLICY") {
		t.Error("should contain COMPLIANCE POLICY section")
	}
	if !strings.Contains(got, "enforce") {
		t.Error("should show enforcement mode")
	}

	// Dashboard wiring (controller-only)
	if !strings.Contains(got, "DASHBOARD WIRING") {
		t.Error("controller should show DASHBOARD WIRING section")
	}
}

func TestRenderSummary_ClientRole(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:            "client",
		ControllerURL:   "https://ctrl.example.com:8080",
		EnrollmentToken: "tok-1234567890",
		DeploymentTier:  "standard",
		FIPS:            config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "AGENT") {
		t.Error("should contain AGENT section for client role")
	}
	if !strings.Contains(got, "ctrl.example.com") {
		t.Error("should show controller URL")
	}
	// Dashboard wiring should NOT appear for client
	if strings.Contains(got, "DASHBOARD WIRING") {
		t.Error("client role should not show DASHBOARD WIRING section")
	}
}

func TestRenderSummary_ProxyRole(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:            "proxy",
		DeploymentTier:  "self_hosted",
		ProxyListenAddr: "0.0.0.0:443",
		ProxyCertFile:   "/etc/pki/tls/proxy.pem",
		ProxyKeyFile:    "/etc/pki/tls/proxy-key.pem",
		TunnelToken:     "eyJproxy-tunnel-token",
		Protocol:        "quic",
		ControllerURL:   "https://ctrl.example.com:8080",
		EnrollmentToken: "tok-proxy-enroll",
		FIPS:            config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "FIPS FORWARD PROXY") {
		t.Error("should contain FIPS FORWARD PROXY section for proxy role")
	}
	if !strings.Contains(got, "0.0.0.0:443") {
		t.Error("should show proxy listen address")
	}
	if !strings.Contains(got, "proxy.pem") {
		t.Error("should show proxy cert")
	}

	// Proxy should have its own PROXY TUNNEL section
	if !strings.Contains(got, "PROXY TUNNEL") {
		t.Error("proxy should contain PROXY TUNNEL section")
	}

	// Proxy should have FLEET ENROLLMENT
	if !strings.Contains(got, "FLEET ENROLLMENT") {
		t.Error("proxy should contain FLEET ENROLLMENT section")
	}
	if !strings.Contains(got, "ctrl.example.com") {
		t.Error("should show controller URL")
	}
}

func TestRenderSummary_Tier2Fields(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:             "controller",
		DeploymentTier:   "regional_keyless",
		TunnelToken:      "eyJ-tier2-token",
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

func TestRenderSummary_MDMIntune(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:           "controller",
		DeploymentTier: "standard",
		TunnelToken:    "eyJ-mdm-test",
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
	if strings.Contains(got, "super-secret-value") {
		t.Error("client secret should be masked")
	}
}

func TestRenderSummary_MDMJamf(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:           "controller",
		DeploymentTier: "standard",
		TunnelToken:    "eyJ-jamf-test",
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
	if strings.Contains(got, "jamf-api-token-12345") {
		t.Error("API token should be masked")
	}
}

func TestRenderSummary_CatchAllIngress(t *testing.T) {
	p := NewReviewPage()
	p.cfg = &config.Config{
		Role:        "controller",
		TunnelToken: "eyJ-ingress-test",
		Ingress: []config.IngressRule{
			{Hostname: "", Service: "http_status:404"},
		},
		Dashboard: config.DashboardConfig{MDM: config.MDMConfig{Provider: "none"}},
		FIPS:      config.FIPSConfig{},
	}

	got := p.renderSummary()
	if !strings.Contains(got, "* →") {
		t.Error("catch-all ingress should show '* →'")
	}
}

// ---------------------------------------------------------------------------
// NewWizardModel
// ---------------------------------------------------------------------------

func TestNewWizardModel(t *testing.T) {
	m := NewWizardModel()
	// Default role is now "controller" → pages: RoleTier, ControllerConfig, DashboardWiring, FIPS, Review = 5
	if len(m.pages) != 5 {
		t.Errorf("expected 5 pages for default controller role, got %d", len(m.pages))
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

	// Verify page titles for default controller role
	expectedTitles := []string{"Role & Tier", "Controller & Tunnel", "Dashboard Wiring", "FIPS Options", "Review & Provision"}
	for i, want := range expectedTitles {
		if i >= len(m.pages) {
			break
		}
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

// ---------------------------------------------------------------------------
// BuildProvisionCommand
// ---------------------------------------------------------------------------

func TestFindProvisionScript_InstalledPath(t *testing.T) {
	// Create a temp dir with the provision script to simulate RPM install
	tmpDir := t.TempDir()
	fakeBin := tmpDir + "/cloudflared-fips-provision"
	if err := os.WriteFile(fakeBin, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Override the installed paths to point to our temp location
	orig := provisionInstalledPaths
	provisionInstalledPaths = []string{fakeBin}
	defer func() { provisionInstalledPaths = orig }()

	got := findProvisionScript()
	if got != fakeBin {
		t.Errorf("findProvisionScript() = %q, want %q (installed path)", got, fakeBin)
	}
}

func TestFindProvisionScript_FallbackWhenNotInstalled(t *testing.T) {
	// Override with a path that doesn't exist
	orig := provisionInstalledPaths
	provisionInstalledPaths = []string{"/nonexistent/cloudflared-fips-provision"}
	defer func() { provisionInstalledPaths = orig }()

	got := findProvisionScript()
	// Should fall back to dev script (LookPath also won't find it)
	if runtime.GOOS == "darwin" {
		if got != "./scripts/provision-macos.sh" {
			t.Errorf("findProvisionScript() = %q, want macOS fallback", got)
		}
	} else {
		if got != "./scripts/provision-linux.sh" {
			t.Errorf("findProvisionScript() = %q, want Linux fallback", got)
		}
	}
}

func TestFindProvisionScript_UsedByBuildProvisionCommand(t *testing.T) {
	// Verify BuildProvisionCommand picks up the installed path
	tmpDir := t.TempDir()
	fakeBin := tmpDir + "/cloudflared-fips-provision"
	if err := os.WriteFile(fakeBin, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatal(err)
	}

	orig := provisionInstalledPaths
	provisionInstalledPaths = []string{fakeBin}
	defer func() { provisionInstalledPaths = orig }()

	cfg := &config.Config{Role: "controller", DeploymentTier: "standard"}
	script, _ := BuildProvisionCommand(cfg)
	if script != fakeBin {
		t.Errorf("BuildProvisionCommand script = %q, want %q", script, fakeBin)
	}
}

func TestBuildProvisionCommand_ServerDefaults(t *testing.T) {
	// Force dev fallback so test works regardless of host
	orig := provisionInstalledPaths
	provisionInstalledPaths = []string{"/nonexistent/cloudflared-fips-provision"}
	defer func() { provisionInstalledPaths = orig }()

	cfg := &config.Config{
		Role:            "server",
		DeploymentTier:  "standard",
		ServiceName:     "my-api",
		ServiceHost:     "0.0.0.0",
		ServicePort:     8443,
		ServiceTLS:      true,
		ControllerURL:   "https://ctrl.example.com:8080",
		EnrollmentToken: "tok-abc",
	}

	script, args := BuildProvisionCommand(cfg)

	if runtime.GOOS == "darwin" {
		if script != "./scripts/provision-macos.sh" {
			t.Errorf("expected macOS script, got %q", script)
		}
	} else {
		if script != "./scripts/provision-linux.sh" {
			t.Errorf("expected Linux script, got %q", script)
		}
	}

	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "--role server") {
		t.Error("should include --role server")
	}
	if !strings.Contains(argStr, "--tier 1") {
		t.Error("should include --tier 1 for standard tier")
	}
	// Server should NOT have --tunnel-token
	if strings.Contains(argStr, "--tunnel-token") {
		t.Error("server should NOT include --tunnel-token")
	}
	// Server should have service flags
	if !strings.Contains(argStr, "--service-name my-api") {
		t.Error("should include --service-name")
	}
	if !strings.Contains(argStr, "--service-host 0.0.0.0") {
		t.Error("should include --service-host")
	}
	if !strings.Contains(argStr, "--service-port 8443") {
		t.Error("should include --service-port")
	}
	if !strings.Contains(argStr, "--service-tls") {
		t.Error("should include --service-tls")
	}
	// Server should have mandatory fleet enrollment
	if !strings.Contains(argStr, "--controller-url https://ctrl.example.com:8080") {
		t.Error("should include --controller-url")
	}
	if !strings.Contains(argStr, "--enrollment-token tok-abc") {
		t.Error("should include --enrollment-token")
	}
}

func TestBuildProvisionCommand_ControllerWithTunnel(t *testing.T) {
	cfg := &config.Config{
		Role:           "controller",
		DeploymentTier: "standard",
		TunnelToken:    "eyJ-controller-token",
		Protocol:       "quic",
		AdminKey:       "admin-key-123",
		CompliancePolicy: config.CompliancePolicyConfig{
			EnforcementMode: "enforce",
			RequireOSFIPS:   true,
			RequireDiskEnc:  true,
		},
		Dashboard: config.DashboardConfig{
			CFAPIToken: "token-abc",
			ZoneID:     "zone-123",
			AccountID:  "acc-456",
			TunnelID:   "tun-789",
		},
	}

	_, args := BuildProvisionCommand(cfg)
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--role controller") {
		t.Error("should include --role controller")
	}
	if !strings.Contains(argStr, "--tunnel-token eyJ-controller-token") {
		t.Error("controller should include --tunnel-token")
	}
	if !strings.Contains(argStr, "--protocol quic") {
		t.Error("should include --protocol")
	}
	if !strings.Contains(argStr, "--admin-key admin-key-123") {
		t.Error("should include --admin-key")
	}
	if !strings.Contains(argStr, "--enforcement-mode enforce") {
		t.Error("should include --enforcement-mode")
	}
	if !strings.Contains(argStr, "--require-os-fips") {
		t.Error("should include --require-os-fips")
	}
	if !strings.Contains(argStr, "--require-disk-enc") {
		t.Error("should include --require-disk-enc")
	}
	if !strings.Contains(argStr, "--cf-api-token token-abc") {
		t.Error("should include --cf-api-token")
	}
	if !strings.Contains(argStr, "--cf-zone-id zone-123") {
		t.Error("should include --cf-zone-id")
	}
}

func TestBuildProvisionCommand_ProxyWithTunnel(t *testing.T) {
	cfg := &config.Config{
		Role:            "proxy",
		DeploymentTier:  "self_hosted",
		TunnelToken:     "eyJ-proxy-tunnel",
		Protocol:        "http2",
		ProxyCertFile:   "/etc/pki/cert.pem",
		ProxyKeyFile:    "/etc/pki/key.pem",
		ControllerURL:   "https://ctrl.example.com:8080",
		EnrollmentToken: "tok-proxy-enroll",
	}

	_, args := BuildProvisionCommand(cfg)
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--role proxy") {
		t.Error("should include --role proxy")
	}
	if !strings.Contains(argStr, "--tier 3") {
		t.Error("should include --tier 3 for self_hosted")
	}
	if !strings.Contains(argStr, "--tunnel-token eyJ-proxy-tunnel") {
		t.Error("proxy should include --tunnel-token (its own tunnel)")
	}
	if !strings.Contains(argStr, "--protocol http2") {
		t.Error("should include --protocol")
	}
	if !strings.Contains(argStr, "--cert /etc/pki/cert.pem") {
		t.Error("should include --cert")
	}
	if !strings.Contains(argStr, "--key /etc/pki/key.pem") {
		t.Error("should include --key")
	}
	// Proxy should NOT have --upstream (removed)
	if strings.Contains(argStr, "--upstream") {
		t.Error("proxy should NOT include --upstream")
	}
	// Proxy should have mandatory fleet enrollment
	if !strings.Contains(argStr, "--controller-url https://ctrl.example.com:8080") {
		t.Error("should include --controller-url")
	}
	if !strings.Contains(argStr, "--enrollment-token tok-proxy-enroll") {
		t.Error("should include --enrollment-token")
	}
}

func TestBuildProvisionCommand_ClientWithFleet(t *testing.T) {
	cfg := &config.Config{
		Role:            "client",
		DeploymentTier:  "standard",
		ControllerURL:   "https://ctrl.example.com:8080",
		EnrollmentToken: "tok-abc123",
		NodeName:        "workstation-1",
		NodeRegion:      "us-east",
	}

	_, args := BuildProvisionCommand(cfg)
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--role client") {
		t.Error("should include --role client")
	}
	if !strings.Contains(argStr, "--enrollment-token tok-abc123") {
		t.Error("should include --enrollment-token")
	}
	if !strings.Contains(argStr, "--controller-url https://ctrl.example.com:8080") {
		t.Error("should include --controller-url")
	}
	if !strings.Contains(argStr, "--node-name workstation-1") {
		t.Error("should include --node-name")
	}
	if !strings.Contains(argStr, "--node-region us-east") {
		t.Error("should include --node-region")
	}
}

func TestBuildProvisionCommand_CFCredentials(t *testing.T) {
	cfg := &config.Config{
		Role:           "controller",
		DeploymentTier: "standard",
		TunnelToken:    "eyJ-cf-creds",
		Dashboard: config.DashboardConfig{
			CFAPIToken: "token-abc",
			ZoneID:     "zone-123",
			AccountID:  "acc-456",
			TunnelID:   "tun-789",
		},
	}

	_, args := BuildProvisionCommand(cfg)
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--cf-api-token token-abc") {
		t.Error("should include --cf-api-token")
	}
	if !strings.Contains(argStr, "--cf-zone-id zone-123") {
		t.Error("should include --cf-zone-id")
	}
	if !strings.Contains(argStr, "--cf-account-id acc-456") {
		t.Error("should include --cf-account-id")
	}
	if !strings.Contains(argStr, "--cf-tunnel-id tun-789") {
		t.Error("should include --cf-tunnel-id")
	}
}

func TestBuildProvisionCommand_SkipFIPS(t *testing.T) {
	cfg := &config.Config{
		Role:           "server",
		DeploymentTier: "standard",
		SkipFIPS:       true,
	}

	_, args := BuildProvisionCommand(cfg)
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--no-fips") {
		t.Error("should include --no-fips when SkipFIPS is true")
	}
}

// ---------------------------------------------------------------------------
// RoleTierPage
// ---------------------------------------------------------------------------

func TestRoleTierPage_Defaults(t *testing.T) {
	p := NewRoleTierPage()
	if p.SelectedRole() != "controller" {
		t.Errorf("default role = %q, want controller", p.SelectedRole())
	}
	if p.SelectedTier() != "standard" {
		t.Errorf("default tier = %q, want standard", p.SelectedTier())
	}
}

func TestRoleTierPage_Apply(t *testing.T) {
	p := NewRoleTierPage()
	cfg := config.NewDefaultConfig()
	p.Apply(cfg)
	if cfg.Role != "controller" {
		t.Errorf("cfg.Role = %q, want controller", cfg.Role)
	}
	if cfg.DeploymentTier != "standard" {
		t.Errorf("cfg.DeploymentTier = %q, want standard", cfg.DeploymentTier)
	}
}

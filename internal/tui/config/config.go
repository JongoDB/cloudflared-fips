// Package config provides the configuration structure for cloudflared-fips,
// matching the schema of configs/cloudflared-fips.yaml.
package config

// Config represents the full cloudflared-fips configuration file.
type Config struct {
	// Fleet / provisioning fields
	Role            string `yaml:"role,omitempty"`
	TunnelToken     string `yaml:"tunnel_token,omitempty"`
	ControllerURL   string `yaml:"controller_url,omitempty"`
	EnrollmentToken string `yaml:"enrollment_token,omitempty"`
	AdminKey        string `yaml:"fleet_admin_key,omitempty"`
	NodeName        string `yaml:"node_name,omitempty"`
	NodeRegion      string `yaml:"node_region,omitempty"`
	SkipFIPS        bool   `yaml:"skip_fips,omitempty"`

	// Transient flags (not persisted to YAML)
	WithCF bool `yaml:"-"`

	Tunnel          string          `yaml:"tunnel"`
	CredentialsFile string          `yaml:"credentials-file"`
	Protocol        string          `yaml:"protocol"`
	GracePeriod     string          `yaml:"grace-period"`
	DeploymentTier  string          `yaml:"deployment_tier"`
	FIPS            FIPSConfig      `yaml:"fips"`
	Transport       TransportConfig `yaml:"transport"`
	CipherSuites    []string        `yaml:"approved-cipher-suites"`
	CurvePrefs      []string        `yaml:"curve-preferences"`
	Ingress         []IngressRule   `yaml:"ingress"`
	LogLevel        string          `yaml:"loglevel"`
	LogFile         string          `yaml:"logfile"`
	Metrics         string          `yaml:"metrics"`

	// Dashboard wiring (custom extension fields)
	Dashboard DashboardConfig `yaml:"dashboard,omitempty"`

	// Tier 2 settings
	RegionalServices bool   `yaml:"regional_services,omitempty"`
	KeylessSSLHost   string `yaml:"keyless_ssl_host,omitempty"`
	KeylessSSLPort   int    `yaml:"keyless_ssl_port,omitempty"`

	// Tier 3 / proxy settings
	ProxyListenAddr string `yaml:"proxy_listen_addr,omitempty"`
	ProxyCertFile   string `yaml:"proxy_cert_file,omitempty"`
	ProxyKeyFile    string `yaml:"proxy_key_file,omitempty"`

	// Server origin service registration
	ServiceName string `yaml:"service_name,omitempty"`
	ServiceHost string `yaml:"service_host,omitempty"`
	ServicePort int    `yaml:"service_port,omitempty"`
	ServiceTLS  bool   `yaml:"service_tls,omitempty"`

	// Compliance enforcement policy (controller-only)
	CompliancePolicy CompliancePolicyConfig `yaml:"compliance_policy,omitempty"`
}

// CompliancePolicyConfig holds compliance enforcement settings (controller-only).
type CompliancePolicyConfig struct {
	EnforcementMode string `yaml:"enforcement_mode,omitempty"` // "enforce", "audit", "disabled"
	RequireOSFIPS   bool   `yaml:"require_os_fips,omitempty"`
	RequireDiskEnc  bool   `yaml:"require_disk_encryption,omitempty"`
	RequireMDM      bool   `yaml:"require_mdm,omitempty"`
	GracePeriodSec  int    `yaml:"grace_period_sec,omitempty"`
}

// FIPSConfig holds FIPS self-test settings.
type FIPSConfig struct {
	SelfTestOnStart       bool   `yaml:"self-test-on-start"`
	FailOnSelfTestFailure bool   `yaml:"fail-on-self-test-failure"`
	SelfTestOutput        string `yaml:"self-test-output"`
	VerifySignature       bool   `yaml:"verify-signature,omitempty"`
}

// TransportConfig holds TLS transport settings.
type TransportConfig struct {
	MinTLSVersion string `yaml:"min-tls-version"`
	MaxTLSVersion string `yaml:"max-tls-version"`
}

// IngressRule represents a single ingress routing rule.
type IngressRule struct {
	Hostname      string         `yaml:"hostname,omitempty"`
	Service       string         `yaml:"service"`
	OriginRequest *OriginRequest `yaml:"originRequest,omitempty"`
}

// OriginRequest holds per-origin TLS and connection settings.
type OriginRequest struct {
	NoTLSVerify      bool   `yaml:"noTLSVerify,omitempty"`
	OriginServerName string `yaml:"originServerName,omitempty"`
	ConnectTimeout   string `yaml:"connectTimeout,omitempty"`
	KeepAliveTimeout string `yaml:"keepAliveTimeout,omitempty"`
}

// DashboardConfig holds Cloudflare API and MDM settings for the dashboard.
type DashboardConfig struct {
	CFAPIToken     string    `yaml:"cf-api-token,omitempty"`
	ZoneID         string    `yaml:"zone-id,omitempty"`
	AccountID      string    `yaml:"account-id,omitempty"`
	TunnelID       string    `yaml:"tunnel-id,omitempty"`
	MetricsAddress string    `yaml:"metrics-address,omitempty"`
	MDM            MDMConfig `yaml:"mdm,omitempty"`
}

// MDMConfig holds MDM provider settings (Intune or Jamf).
type MDMConfig struct {
	Provider     string `yaml:"provider,omitempty"`
	TenantID     string `yaml:"tenant-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty"`
	ClientSecret string `yaml:"client-secret,omitempty"`
	BaseURL      string `yaml:"base-url,omitempty"`
	APIToken     string `yaml:"api-token,omitempty"`
}

// NewDefaultConfig returns a Config populated with safe defaults.
func NewDefaultConfig() *Config {
	return &Config{
		Role:            "server",
		CredentialsFile: "/etc/cloudflared/credentials.json",
		CompliancePolicy: CompliancePolicyConfig{
			EnforcementMode: "audit",
		},
		Protocol:        "quic",
		GracePeriod:     "30s",
		DeploymentTier:  "standard",
		FIPS: FIPSConfig{
			SelfTestOnStart:       true,
			FailOnSelfTestFailure: true,
			SelfTestOutput:        "/var/log/cloudflared/selftest.json",
		},
		Transport: TransportConfig{
			MinTLSVersion: "1.2",
			MaxTLSVersion: "1.3",
		},
		CipherSuites: []string{
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
			"TLS_AES_256_GCM_SHA384",
			"TLS_AES_128_GCM_SHA256",
			"TLS_CHACHA20_POLY1305_SHA256",
		},
		CurvePrefs: []string{"P-256", "P-384"},
		Ingress: []IngressRule{
			{Service: "http_status:404"},
		},
		LogLevel: "info",
		LogFile:  "/var/log/cloudflared/cloudflared.log",
		Metrics:  "localhost:2000",
		Dashboard: DashboardConfig{
			MetricsAddress: "localhost:2000",
		},
	}
}

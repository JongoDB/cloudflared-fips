// Package deployment provides deployment tier configuration and validation.
// Three tiers are supported:
//
//   - standard: Direct Cloudflare Tunnel (trust Cloudflare FedRAMP authorization)
//   - regional_keyless: Tunnel + Regional Services + Keyless SSL (HSM-backed keys)
//   - self_hosted: Self-hosted FIPS proxy (full control over TLS termination)
package deployment

import (
	"fmt"
	"strings"
)

// Tier represents a deployment architecture tier.
type Tier string

const (
	TierStandard        Tier = "standard"
	TierRegionalKeyless Tier = "regional_keyless"
	TierSelfHosted      Tier = "self_hosted"
)

// ParseTier parses a tier string, returning an error for invalid values.
func ParseTier(s string) (Tier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "standard", "1", "tier1":
		return TierStandard, nil
	case "regional_keyless", "regional-keyless", "2", "tier2":
		return TierRegionalKeyless, nil
	case "self_hosted", "self-hosted", "3", "tier3":
		return TierSelfHosted, nil
	default:
		return "", fmt.Errorf("unknown deployment tier: %q (valid: standard, regional_keyless, self_hosted)", s)
	}
}

// Info returns a human-readable description of the tier.
type TierInfo struct {
	Tier        Tier   `json:"tier"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Which compliance sections are "inherited" vs "verified"
	EdgeVerification   string `json:"edge_verification"`
	KeyManagement      string `json:"key_management"`
	ClientInspection   string `json:"client_inspection"`
	FedRAMPDependency  bool   `json:"fedramp_dependency"`
}

// GetTierInfo returns detailed information about a deployment tier.
func GetTierInfo(tier Tier) TierInfo {
	switch tier {
	case TierStandard:
		return TierInfo{
			Tier:              tier,
			Name:              "Standard Cloudflare Tunnel",
			Description:       "Direct tunnel to Cloudflare edge. Edge crypto relies on Cloudflare's FedRAMP Moderate authorization.",
			EdgeVerification:  "inherited",
			KeyManagement:     "Cloudflare-managed (edge key storage)",
			ClientInspection:  "Limited (Cloudflare logs, no ClientHello access)",
			FedRAMPDependency: true,
		}
	case TierRegionalKeyless:
		return TierInfo{
			Tier:              tier,
			Name:              "Regional Services + Keyless SSL",
			Description:       "Tunnel with Regional Services restricting TLS termination to FedRAMP US data centers. Private keys held in customer-managed FIPS 140-2 Level 3 HSMs.",
			EdgeVerification:  "api + inherited",
			KeyManagement:     "Customer HSM (FIPS 140-2 L3) via Keyless SSL",
			ClientInspection:  "Limited (Cloudflare logs, no ClientHello access)",
			FedRAMPDependency: true,
		}
	case TierSelfHosted:
		return TierInfo{
			Tier:              tier,
			Name:              "Self-Hosted FIPS Edge Proxy",
			Description:       "Customer-controlled FIPS proxy terminates TLS with BoringCrypto. Full ClientHello inspection and JA4 fingerprinting. Deployed in GovCloud.",
			EdgeVerification:  "direct",
			KeyManagement:     "Customer-managed (local cert + key, or HSM)",
			ClientInspection:  "Full (TLS ClientHello analysis, JA4, cipher logging)",
			FedRAMPDependency: false,
		}
	default:
		return TierInfo{
			Tier:        tier,
			Name:        "Unknown",
			Description: "Unknown deployment tier",
		}
	}
}

// Config holds deployment-specific configuration parsed from cloudflared-fips.yaml.
type Config struct {
	Tier Tier `json:"deployment_tier" yaml:"deployment_tier"`

	// Tier 2 settings
	RegionalServicesEnabled bool   `json:"regional_services" yaml:"regional_services"`
	KeylessSSLHost          string `json:"keyless_ssl_host" yaml:"keyless_ssl_host"`
	KeylessSSLPort          int    `json:"keyless_ssl_port" yaml:"keyless_ssl_port"`

	// Tier 3 settings
	ProxyListenAddr string `json:"proxy_listen_addr" yaml:"proxy_listen_addr"`
	ProxyCertFile   string `json:"proxy_cert_file" yaml:"proxy_cert_file"`
	ProxyKeyFile    string `json:"proxy_key_file" yaml:"proxy_key_file"`
	ProxyUpstream   string `json:"proxy_upstream" yaml:"proxy_upstream"`
}

// DefaultConfig returns a config for the standard deployment tier.
func DefaultConfig() Config {
	return Config{
		Tier: TierStandard,
	}
}

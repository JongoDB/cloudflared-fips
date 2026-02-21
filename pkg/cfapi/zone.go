package cfapi

import (
	"encoding/json"
	"fmt"
)

// ZoneSetting represents a Cloudflare zone setting value.
type ZoneSetting struct {
	ID         string          `json:"id"`
	Value      json.RawMessage `json:"value"`
	ModifiedOn string          `json:"modified_on"`
	Editable   bool            `json:"editable"`
}

// SSLSetting contains SSL/TLS related zone settings.
type SSLSetting struct {
	MinTLSVersion string   `json:"min_tls_version"`
	Ciphers       []string `json:"ciphers"`
}

// SecurityHeader represents HSTS settings.
type SecurityHeader struct {
	StrictTransportSecurity struct {
		Enabled           bool `json:"enabled"`
		MaxAge            int  `json:"max_age"`
		IncludeSubdomains bool `json:"include_subdomains"`
		Preload           bool `json:"preload"`
		NoSniff           bool `json:"nosniff"`
	} `json:"strict_transport_security"`
}

// CertificatePack represents an edge SSL certificate.
type CertificatePack struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Status            string   `json:"status"`
	Hosts             []string `json:"hosts"`
	CertificateExpiry string   `json:"expires_on"`
}

// AccessApp represents a Cloudflare Access application.
type AccessApp struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Type     string `json:"type"`
	Enabled  bool   `json:"allowed_idps,omitempty"`
	AuthType string `json:"auth_type,omitempty"`
}

// TunnelInfo represents a Cloudflare Tunnel.
type TunnelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ConnectedAt string `json:"conns_active_at,omitempty"`
	Connections []struct {
		ColoName       string `json:"colo_name"`
		IsActive       bool   `json:"is_active"`
		ClientVersion  string `json:"client_version"`
		OriginIP       string `json:"origin_ip"`
	} `json:"connections"`
}

// GetMinTLSVersion returns the minimum TLS version setting for a zone.
func (c *Client) GetMinTLSVersion(zoneID string) (string, error) {
	data, err := c.get(fmt.Sprintf("/zones/%s/settings/min_tls_version", zoneID))
	if err != nil {
		return "", err
	}
	var setting ZoneSetting
	if err := json.Unmarshal(data, &setting); err != nil {
		return "", fmt.Errorf("parse min_tls_version: %w", err)
	}
	var value string
	if err := json.Unmarshal(setting.Value, &value); err != nil {
		return "", fmt.Errorf("parse value: %w", err)
	}
	return value, nil
}

// GetCiphers returns the configured cipher suites for a zone.
func (c *Client) GetCiphers(zoneID string) ([]string, error) {
	data, err := c.get(fmt.Sprintf("/zones/%s/settings/ciphers", zoneID))
	if err != nil {
		return nil, err
	}
	var setting ZoneSetting
	if err := json.Unmarshal(data, &setting); err != nil {
		return nil, fmt.Errorf("parse ciphers: %w", err)
	}
	var ciphers []string
	if err := json.Unmarshal(setting.Value, &ciphers); err != nil {
		return nil, fmt.Errorf("parse value: %w", err)
	}
	return ciphers, nil
}

// GetSecurityHeader returns the HSTS settings for a zone.
func (c *Client) GetSecurityHeader(zoneID string) (*SecurityHeader, error) {
	data, err := c.get(fmt.Sprintf("/zones/%s/settings/security_header", zoneID))
	if err != nil {
		return nil, err
	}
	var setting ZoneSetting
	if err := json.Unmarshal(data, &setting); err != nil {
		return nil, fmt.Errorf("parse security_header: %w", err)
	}
	var header SecurityHeader
	if err := json.Unmarshal(setting.Value, &header); err != nil {
		return nil, fmt.Errorf("parse value: %w", err)
	}
	return &header, nil
}

// GetCertificatePacks returns the SSL certificate packs for a zone.
func (c *Client) GetCertificatePacks(zoneID string) ([]CertificatePack, error) {
	data, err := c.get(fmt.Sprintf("/zones/%s/ssl/certificate_packs", zoneID))
	if err != nil {
		return nil, err
	}
	var packs []CertificatePack
	if err := json.Unmarshal(data, &packs); err != nil {
		return nil, fmt.Errorf("parse certificate_packs: %w", err)
	}
	return packs, nil
}

// GetAccessApps returns the Cloudflare Access applications for a zone.
func (c *Client) GetAccessApps(zoneID string) ([]AccessApp, error) {
	data, err := c.get(fmt.Sprintf("/zones/%s/access/apps", zoneID))
	if err != nil {
		return nil, err
	}
	var apps []AccessApp
	if err := json.Unmarshal(data, &apps); err != nil {
		return nil, fmt.Errorf("parse access apps: %w", err)
	}
	return apps, nil
}

// GetTunnel returns info about a specific Cloudflare Tunnel.
func (c *Client) GetTunnel(accountID, tunnelID string) (*TunnelInfo, error) {
	data, err := c.get(fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", accountID, tunnelID))
	if err != nil {
		return nil, err
	}
	var tunnel TunnelInfo
	if err := json.Unmarshal(data, &tunnel); err != nil {
		return nil, fmt.Errorf("parse tunnel: %w", err)
	}
	return &tunnel, nil
}

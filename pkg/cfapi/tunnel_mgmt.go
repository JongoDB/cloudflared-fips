package cfapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// TunnelWithToken is the result of creating a tunnel, including the locally
// generated connector token.
type TunnelWithToken struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"` // base64-encoded JSON: {"a":accountID,"t":tunnelID,"s":base64(secret)}
}

// TunnelIngressConfig is the body for PUT /accounts/{id}/cfd_tunnel/{id}/configurations.
type TunnelIngressConfig struct {
	Config TunnelConfigBody `json:"config"`
}

// TunnelConfigBody holds the ingress rules inside the config wrapper.
type TunnelConfigBody struct {
	Ingress []TunnelIngressRule `json:"ingress"`
}

// TunnelIngressRule defines a single public hostname → service mapping.
type TunnelIngressRule struct {
	Hostname      string                 `json:"hostname,omitempty"`
	Service       string                 `json:"service"`
	OriginRequest map[string]interface{} `json:"originRequest,omitempty"`
}

// DNSRecord represents a Cloudflare DNS record for creation.
type DNSRecord struct {
	Type    string `json:"type"`    // "CNAME"
	Name    string `json:"name"`    // "dashboard.jondevs.com"
	Content string `json:"content"` // "<tunnel-id>.cfargotunnel.com"
	Proxied bool   `json:"proxied"` // true
	Comment string `json:"comment,omitempty"`
}

// DNSRecordResult is the result from creating a DNS record.
type DNSRecordResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// createTunnelRequest is the body for POST /accounts/{id}/cfd_tunnel.
type createTunnelRequest struct {
	Name         string `json:"name"`
	TunnelSecret string `json:"tunnel_secret"` // base64-encoded 32-byte secret
}

// createTunnelResponse is the API result from creating a tunnel.
type createTunnelResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Secret string `json:"tunnel_secret,omitempty"` // echoed back by API (base64)
}

// CreateTunnel creates a new Cloudflare Tunnel and returns the tunnel info
// with a locally-generated connector token. The token is generated from
// the account ID, tunnel ID, and a random secret — no need to visit the
// Cloudflare dashboard.
func (c *Client) CreateTunnel(accountID, name string) (*TunnelWithToken, error) {
	// Generate 32-byte random secret for the tunnel
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate tunnel secret: %w", err)
	}
	secretB64 := base64.StdEncoding.EncodeToString(secret)

	path := fmt.Sprintf("/accounts/%s/cfd_tunnel", accountID)
	data, err := c.post(path, createTunnelRequest{
		Name:         name,
		TunnelSecret: secretB64,
	})
	if err != nil {
		return nil, fmt.Errorf("create tunnel: %w", err)
	}

	var resp createTunnelResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse tunnel response: %w", err)
	}

	token := GenerateTunnelToken(accountID, resp.ID, secret)

	return &TunnelWithToken{
		ID:    resp.ID,
		Name:  resp.Name,
		Token: token,
	}, nil
}

// GetTunnelToken retrieves the connector token for an existing tunnel.
// API: GET /accounts/{accountID}/cfd_tunnel/{tunnelID}/token
func (c *Client) GetTunnelToken(accountID, tunnelID string) (string, error) {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/token", accountID, tunnelID)
	data, err := c.get(path)
	if err != nil {
		return "", fmt.Errorf("get tunnel token: %w", err)
	}
	// The API returns the token as a JSON string
	var token string
	if err := json.Unmarshal(data, &token); err != nil {
		return "", fmt.Errorf("parse tunnel token: %w", err)
	}
	return token, nil
}

// ConfigureTunnelIngress sets the public hostname → service mapping for a tunnel.
// This replaces the entire tunnel configuration.
func (c *Client) ConfigureTunnelIngress(accountID, tunnelID string, ingress []TunnelIngressRule) error {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", accountID, tunnelID)
	_, err := c.put(path, TunnelIngressConfig{
		Config: TunnelConfigBody{
			Ingress: ingress,
		},
	})
	if err != nil {
		return fmt.Errorf("configure tunnel ingress: %w", err)
	}
	return nil
}

// CreateDNSCNAME creates a proxied CNAME record pointing to the tunnel's
// .cfargotunnel.com hostname.
func (c *Client) CreateDNSCNAME(zoneID, hostname, tunnelID string) (*DNSRecordResult, error) {
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)
	data, err := c.post(path, DNSRecord{
		Type:    "CNAME",
		Name:    hostname,
		Content: tunnelID + ".cfargotunnel.com",
		Proxied: true,
		Comment: "Created by cloudflared-fips TUI wizard",
	})
	if err != nil {
		return nil, fmt.Errorf("create DNS CNAME: %w", err)
	}

	var result DNSRecordResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse DNS record response: %w", err)
	}
	return &result, nil
}

// DeleteTunnel deletes a Cloudflare Tunnel. The tunnel must have no active
// connections (i.e., cloudflared must be stopped first).
// API: DELETE /accounts/{accountID}/cfd_tunnel/{tunnelID}
func (c *Client) DeleteTunnel(accountID, tunnelID string) error {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", accountID, tunnelID)
	_, err := c.delete(path)
	if err != nil {
		return fmt.Errorf("delete tunnel: %w", err)
	}
	return nil
}

// DeleteDNSRecord deletes a single DNS record by ID.
// API: DELETE /zones/{zoneID}/dns_records/{recordID}
func (c *Client) DeleteDNSRecord(zoneID, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)
	_, err := c.delete(path)
	if err != nil {
		return fmt.Errorf("delete DNS record: %w", err)
	}
	return nil
}

// FindDNSRecord searches for DNS records matching the given name and type.
// Returns matching records, which can then be deleted by ID.
// API: GET /zones/{zoneID}/dns_records?type={recordType}&name={name}
func (c *Client) FindDNSRecord(zoneID, name, recordType string) ([]DNSRecordResult, error) {
	path := fmt.Sprintf("/zones/%s/dns_records?type=%s&name=%s", zoneID, recordType, name)
	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("find DNS records: %w", err)
	}

	var results []DNSRecordResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse DNS records response: %w", err)
	}
	return results, nil
}

// GenerateTunnelToken generates a cloudflared connector token from account ID,
// tunnel ID, and the 32-byte secret. The token is a base64-encoded JSON object:
//
//	{"a":"<accountID>","t":"<tunnelID>","s":"<base64(secret)>"}
//
// This is the same format used by `cloudflared tunnel token <name>`.
func GenerateTunnelToken(accountID, tunnelID string, secret []byte) string {
	tokenJSON, _ := json.Marshal(map[string]string{
		"a": accountID,
		"t": tunnelID,
		"s": base64.StdEncoding.EncodeToString(secret),
	})
	return base64.StdEncoding.EncodeToString(tokenJSON)
}

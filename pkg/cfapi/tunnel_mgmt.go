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

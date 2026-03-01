package cfapi

import "fmt"

// FIPSApprovedCipherList is the set of FIPS-approved cipher suites for Cloudflare edge.
var FIPSApprovedCipherList = []string{
	"ECDHE-ECDSA-AES128-GCM-SHA256",
	"ECDHE-ECDSA-AES256-GCM-SHA384",
	"ECDHE-RSA-AES128-GCM-SHA256",
	"ECDHE-RSA-AES256-GCM-SHA384",
	"AES128-GCM-SHA256",
	"AES256-GCM-SHA384",
}

// SetCiphers updates the cipher suites for a zone via PATCH.
func (c *Client) SetCiphers(zoneID string, ciphers []string) error {
	path := fmt.Sprintf("/zones/%s/settings/ciphers", zoneID)
	_, err := c.patch(path, map[string]interface{}{
		"value": ciphers,
	})
	return err
}

// SetMinTLSVersion updates the minimum TLS version for a zone via PATCH.
// Valid values: "1.0", "1.1", "1.2", "1.3".
func (c *Client) SetMinTLSVersion(zoneID, version string) error {
	path := fmt.Sprintf("/zones/%s/settings/min_tls_version", zoneID)
	_, err := c.patch(path, map[string]interface{}{
		"value": version,
	})
	return err
}

// SetHSTS updates HSTS settings for a zone via PATCH.
func (c *Client) SetHSTS(zoneID string, enabled bool, maxAge int, includeSubdomains, preload bool) error {
	path := fmt.Sprintf("/zones/%s/settings/security_header", zoneID)
	_, err := c.patch(path, map[string]interface{}{
		"value": map[string]interface{}{
			"strict_transport_security": map[string]interface{}{
				"enabled":            enabled,
				"max_age":            maxAge,
				"include_subdomains": includeSubdomains,
				"preload":            preload,
				"nosniff":            true,
			},
		},
	})
	return err
}

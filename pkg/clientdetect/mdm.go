// MDM API integration for device posture verification.
//
// Supports Microsoft Intune and Jamf Pro for verifying device compliance
// including OS FIPS mode, disk encryption, MDM enrollment, and OS version.
//
// Both providers require API credentials:
//   - Intune: Azure AD app registration with DeviceManagementManagedDevices.Read.All
//   - Jamf: API role with "Read Computers" and "Read Mobile Devices"
package clientdetect

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MDMProvider identifies the MDM platform.
type MDMProvider string

const (
	MDMProviderIntune MDMProvider = "intune"
	MDMProviderJamf   MDMProvider = "jamf"
	MDMProviderNone   MDMProvider = "none"
)

// MDMConfig holds the configuration for MDM API integration.
type MDMConfig struct {
	Provider     MDMProvider `json:"provider"`
	APIToken     string      `json:"api_token,omitempty"`
	BaseURL      string      `json:"base_url,omitempty"`      // Jamf: https://your-instance.jamfcloud.com
	TenantID     string      `json:"tenant_id,omitempty"`     // Intune: Azure AD tenant ID
	ClientID     string      `json:"client_id,omitempty"`     // Intune: Azure AD app client ID
	ClientSecret string      `json:"client_secret,omitempty"` // Intune: Azure AD app client secret
}

// MDMDeviceStatus represents the compliance status of a managed device.
type MDMDeviceStatus struct {
	DeviceID        string    `json:"device_id"`
	DeviceName      string    `json:"device_name"`
	OSType          string    `json:"os_type"`
	OSVersion       string    `json:"os_version"`
	Compliant       bool      `json:"compliant"`
	Encrypted       bool      `json:"disk_encrypted"`
	FIPSMode        bool      `json:"fips_mode"`
	MDMEnrolled     bool      `json:"mdm_enrolled"`
	LastCheckIn     time.Time `json:"last_check_in"`
	ComplianceState string    `json:"compliance_state"` // compliant, noncompliant, unknown
	Provider        string    `json:"provider"`
}

// MDMClient provides access to MDM device compliance data.
type MDMClient struct {
	config MDMConfig
	cache  struct {
		mu      sync.RWMutex
		devices []MDMDeviceStatus
		updated time.Time
		ttl     time.Duration
	}
	httpClient *http.Client
}

// NewMDMClient creates a new MDM client.
func NewMDMClient(config MDMConfig) *MDMClient {
	client := &MDMClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	client.cache.ttl = 5 * time.Minute
	return client
}

// IsConfigured returns true if the MDM client has valid credentials.
func (c *MDMClient) IsConfigured() bool {
	switch c.config.Provider {
	case MDMProviderIntune:
		return c.config.TenantID != "" && c.config.ClientID != "" && c.config.ClientSecret != ""
	case MDMProviderJamf:
		return c.config.BaseURL != "" && c.config.APIToken != ""
	default:
		return false
	}
}

// Provider returns the configured MDM provider name.
func (c *MDMClient) Provider() MDMProvider {
	return c.config.Provider
}

// FetchDevices retrieves managed device statuses from the MDM provider.
// Results are cached for the configured TTL.
func (c *MDMClient) FetchDevices() ([]MDMDeviceStatus, error) {
	// Check cache
	c.cache.mu.RLock()
	if time.Since(c.cache.updated) < c.cache.ttl && c.cache.devices != nil {
		devices := make([]MDMDeviceStatus, len(c.cache.devices))
		copy(devices, c.cache.devices)
		c.cache.mu.RUnlock()
		return devices, nil
	}
	c.cache.mu.RUnlock()

	var devices []MDMDeviceStatus
	var err error

	switch c.config.Provider {
	case MDMProviderIntune:
		devices, err = c.fetchIntuneDevices()
	case MDMProviderJamf:
		devices, err = c.fetchJamfDevices()
	default:
		return nil, fmt.Errorf("MDM provider not configured")
	}

	if err != nil {
		return nil, err
	}

	// Update cache
	c.cache.mu.Lock()
	c.cache.devices = devices
	c.cache.updated = time.Now()
	c.cache.mu.Unlock()

	return devices, nil
}

// ComplianceSummary returns aggregate compliance stats.
func (c *MDMClient) ComplianceSummary() MDMComplianceSummary {
	devices, err := c.FetchDevices()
	if err != nil {
		return MDMComplianceSummary{Error: err.Error()}
	}

	summary := MDMComplianceSummary{
		Provider:     string(c.config.Provider),
		TotalDevices: len(devices),
	}

	for _, d := range devices {
		if d.Compliant {
			summary.Compliant++
		} else {
			summary.NonCompliant++
		}
		if d.Encrypted {
			summary.Encrypted++
		}
		if d.FIPSMode {
			summary.FIPSEnabled++
		}
		if d.MDMEnrolled {
			summary.Enrolled++
		}
	}

	return summary
}

// MDMComplianceSummary holds aggregate MDM compliance statistics.
type MDMComplianceSummary struct {
	Provider     string `json:"provider"`
	TotalDevices int    `json:"total_devices"`
	Compliant    int    `json:"compliant"`
	NonCompliant int    `json:"non_compliant"`
	Encrypted    int    `json:"disk_encrypted"`
	FIPSEnabled  int    `json:"fips_enabled"`
	Enrolled     int    `json:"enrolled"`
	Error        string `json:"error,omitempty"`
}

// ── Intune implementation ──

// fetchIntuneDevices queries Microsoft Graph API for managed device compliance.
// API: GET https://graph.microsoft.com/v1.0/deviceManagement/managedDevices
// Requires: DeviceManagementManagedDevices.Read.All permission
func (c *MDMClient) fetchIntuneDevices() ([]MDMDeviceStatus, error) {
	// Step 1: Get OAuth2 token from Azure AD
	token, err := c.getIntuneToken()
	if err != nil {
		return nil, fmt.Errorf("intune auth: %w", err)
	}

	// Step 2: Query managed devices
	req, err := http.NewRequest("GET",
		"https://graph.microsoft.com/v1.0/deviceManagement/managedDevices?$select=id,deviceName,operatingSystem,osVersion,complianceState,isEncrypted,enrolledDateTime,lastSyncDateTime",
		nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("intune API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("intune API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Value []struct {
			ID              string `json:"id"`
			DeviceName      string `json:"deviceName"`
			OperatingSystem string `json:"operatingSystem"`
			OSVersion       string `json:"osVersion"`
			ComplianceState string `json:"complianceState"`
			IsEncrypted     bool   `json:"isEncrypted"`
			LastSync        string `json:"lastSyncDateTime"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("intune parse: %w", err)
	}

	var devices []MDMDeviceStatus
	for _, d := range result.Value {
		lastSync, _ := time.Parse(time.RFC3339, d.LastSync)
		devices = append(devices, MDMDeviceStatus{
			DeviceID:        d.ID,
			DeviceName:      d.DeviceName,
			OSType:          d.OperatingSystem,
			OSVersion:       d.OSVersion,
			Compliant:       d.ComplianceState == "compliant",
			Encrypted:       d.IsEncrypted,
			FIPSMode:        false, // Intune doesn't directly report FIPS mode
			MDMEnrolled:     true,
			LastCheckIn:     lastSync,
			ComplianceState: d.ComplianceState,
			Provider:        "intune",
		})
	}

	return devices, nil
}

func (c *MDMClient) getIntuneToken() (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.config.TenantID)

	body := fmt.Sprintf("client_id=%s&client_secret=%s&scope=https%%3A%%2F%%2Fgraph.microsoft.com%%2F.default&grant_type=client_credentials",
		c.config.ClientID, c.config.ClientSecret)

	resp, err := c.httpClient.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.Description)
	}

	return tokenResp.AccessToken, nil
}

// ── Jamf implementation ──

// fetchJamfDevices queries Jamf Pro API for managed computer inventory.
// API: GET /api/v1/computers-inventory
// Requires: API role with "Read Computers" permission
func (c *MDMClient) fetchJamfDevices() ([]MDMDeviceStatus, error) {
	url := fmt.Sprintf("%s/api/v1/computers-inventory?section=GENERAL&section=HARDWARE&section=OPERATING_SYSTEM&section=SECURITY",
		strings.TrimRight(c.config.BaseURL, "/"))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jamf API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jamf API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			ID      string `json:"id"`
			General struct {
				Name        string `json:"name"`
				LastCheckIn string `json:"lastContactTime"`
				MdmCapable  bool   `json:"mdmCapable"`
			} `json:"general"`
			OperatingSystem struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"operatingSystem"`
			Security struct {
				FileVault2Status string `json:"fileVault2Status"`
			} `json:"security"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jamf parse: %w", err)
	}

	var devices []MDMDeviceStatus
	for _, d := range result.Results {
		lastCheck, _ := time.Parse(time.RFC3339, d.General.LastCheckIn)
		encrypted := d.Security.FileVault2Status == "ALL_ENCRYPTED" ||
			d.Security.FileVault2Status == "BOOT_ENCRYPTED"

		devices = append(devices, MDMDeviceStatus{
			DeviceID:        d.ID,
			DeviceName:      d.General.Name,
			OSType:          d.OperatingSystem.Name,
			OSVersion:       d.OperatingSystem.Version,
			Compliant:       d.General.MdmCapable && encrypted,
			Encrypted:       encrypted,
			FIPSMode:        false, // Jamf doesn't directly report FIPS mode
			MDMEnrolled:     d.General.MdmCapable,
			LastCheckIn:     lastCheck,
			ComplianceState: "managed",
			Provider:        "jamf",
		})
	}

	return devices, nil
}

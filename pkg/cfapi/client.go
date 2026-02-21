// Package cfapi provides a Cloudflare API client for querying zone and tunnel
// settings used in FIPS compliance checks. All requests use the v4 REST API
// with bearer token authentication.
package cfapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is a rate-limited, caching Cloudflare API client.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client

	// Simple in-memory cache with TTL
	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	data      json.RawMessage
	expiresAt time.Time
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithCacheTTL sets the cache TTL (default: 60 seconds).
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *Client) { c.ttl = ttl }
}

// NewClient creates a Cloudflare API client with the given bearer token.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		token:   token,
		baseURL: "https://api.cloudflare.com/client/v4",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]cacheEntry),
		ttl:   60 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// apiResponse is the common Cloudflare API envelope.
type apiResponse struct {
	Success  bool            `json:"success"`
	Errors   []apiError      `json:"errors"`
	Messages []interface{}   `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// get performs a cached GET request to the Cloudflare API.
func (c *Client) get(path string) (json.RawMessage, error) {
	// Check cache
	c.mu.RLock()
	if entry, ok := c.cache[path]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.data, nil
	}
	c.mu.RUnlock()

	url := c.baseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited by Cloudflare API (429)")
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse API response: %w", err)
	}

	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s (code %d)", apiResp.Errors[0].Message, apiResp.Errors[0].Code)
		}
		return nil, fmt.Errorf("API returned success=false")
	}

	// Cache the result
	c.mu.Lock()
	c.cache[path] = cacheEntry{
		data:      apiResp.Result,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return apiResp.Result, nil
}

// ClearCache removes all cached responses.
func (c *Client) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string]cacheEntry)
	c.mu.Unlock()
}

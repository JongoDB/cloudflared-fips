package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookPayload is the JSON body sent to webhook endpoints.
type WebhookPayload struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"event_type"`
	Severity  string `json:"severity"`
	Summary   string `json:"summary"`
	Detail    string `json:"detail"`
	NISTRef   string `json:"nist_ref,omitempty"`
}

// sendWebhook sends a single webhook POST request.
func sendWebhook(url string, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook POST %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook POST %s: HTTP %d", url, resp.StatusCode)
	}
	return nil
}

// sendWebhookWithRetry sends a webhook with exponential backoff retries.
func sendWebhookWithRetry(url string, payload WebhookPayload, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}
		lastErr = sendWebhook(url, payload)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

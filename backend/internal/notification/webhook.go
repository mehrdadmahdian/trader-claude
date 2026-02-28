package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookSender sends JSON payloads to a webhook URL with HMAC-SHA256 signatures.
type WebhookSender struct {
	httpClient *http.Client
}

// NewWebhookSender creates a WebhookSender.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts the payload to the URL, signed with HMAC-SHA256.
// Retries up to 3 times on 5xx errors with exponential backoff.
func (w *WebhookSender) Send(ctx context.Context, url, secret string, payload []byte) error {
	sig := computeHMAC(secret, payload)

	var lastErr error
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoffs[attempt-1]):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("webhook: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-TraderClaude-Signature", "sha256="+sig)

		resp, err := w.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("webhook: request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("webhook: status %d: %s", resp.StatusCode, string(body))

		if resp.StatusCode < 500 {
			return lastErr
		}
	}

	return fmt.Errorf("webhook: exhausted retries: %w", lastErr)
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of payload using secret.
func computeHMAC(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks an incoming webhook signature.
func VerifySignature(secret string, payload []byte, signature string) bool {
	expected := "sha256=" + computeHMAC(secret, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}

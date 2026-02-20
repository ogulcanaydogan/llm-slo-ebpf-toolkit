package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// Format selects the webhook payload format.
type Format string

const (
	FormatGeneric   Format = "generic"
	FormatPagerDuty Format = "pagerduty"
	FormatOpsgenie  Format = "opsgenie"
)

// Exporter delivers incident attribution events to an HTTP webhook endpoint.
type Exporter struct {
	URL       string
	Secret    string
	Format    Format
	TimeoutMS int
	MaxRetry  int
	client    *http.Client
}

// New creates a webhook exporter with sensible defaults.
func New(url, secret string, format Format, timeoutMS int) *Exporter {
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	if format == "" {
		format = FormatGeneric
	}
	return &Exporter{
		URL:       url,
		Secret:    secret,
		Format:    format,
		TimeoutMS: timeoutMS,
		MaxRetry:  3,
		client: &http.Client{
			Timeout: time.Duration(timeoutMS) * time.Millisecond,
		},
	}
}

// nonRetryableError wraps errors that should not be retried (e.g., 4xx).
type nonRetryableError struct{ err error }

func (e *nonRetryableError) Error() string { return e.err.Error() }
func (e *nonRetryableError) Unwrap() error { return e.err }

// Send delivers one incident attribution to the webhook endpoint.
func (e *Exporter) Send(attr schema.IncidentAttribution) error {
	payload, contentType, err := e.buildPayload(attr)
	if err != nil {
		return fmt.Errorf("build webhook payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < e.MaxRetry; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}

		lastErr = e.doPost(payload, contentType)
		if lastErr == nil {
			return nil
		}
		if _, ok := lastErr.(*nonRetryableError); ok {
			return lastErr
		}
	}
	return fmt.Errorf("webhook delivery failed after %d attempts: %w", e.MaxRetry, lastErr)
}

func (e *Exporter) buildPayload(attr schema.IncidentAttribution) ([]byte, string, error) {
	switch e.Format {
	case FormatPagerDuty:
		return BuildPagerDutyPayload(attr)
	case FormatOpsgenie:
		return BuildOpsgeniePayload(attr)
	default:
		data, err := json.Marshal(attr)
		return data, "application/json", err
	}
}

func (e *Exporter) doPost(payload []byte, contentType string) error {
	req, err := http.NewRequest(http.MethodPost, e.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "llm-slo-ebpf-toolkit/webhook")

	if e.Secret != "" {
		sig := computeHMAC(payload, e.Secret)
		req.Header.Set("X-Webhook-Signature", sig)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("drain response body: %w", err)
	}

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return &nonRetryableError{err: fmt.Errorf("client error: HTTP %d", resp.StatusCode)}
	}
	return nil
}

func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC checks an HMAC-SHA256 signature against a payload and secret.
func VerifyHMAC(payload []byte, secret, signature string) bool {
	expected := computeHMAC(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

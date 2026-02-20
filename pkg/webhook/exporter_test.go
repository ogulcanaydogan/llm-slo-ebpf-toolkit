package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func sampleAttribution() schema.IncidentAttribution {
	return schema.IncidentAttribution{
		IncidentID:           "inc-test-01",
		Timestamp:            time.Now().UTC(),
		Cluster:              "local",
		Service:              "chat",
		PredictedFaultDomain: "network_dns",
		Confidence:           0.92,
		Evidence: []schema.Evidence{
			{Signal: "dns_latency_ms", Value: 220.0, Source: "ebpf"},
		},
		SLOImpact: schema.SLOImpact{
			SLI:           "ttft_ms",
			BurnRate:      2.3,
			WindowMinutes: 5,
		},
	}
}

func TestSendGenericPayload(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := New(server.URL, "", FormatGeneric, 5000)
	if err := e.Send(sampleAttribution()); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if len(received) == 0 {
		t.Fatal("expected payload")
	}

	var attr schema.IncidentAttribution
	if err := json.Unmarshal(received, &attr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if attr.IncidentID != "inc-test-01" {
		t.Errorf("incident_id: got %s", attr.IncidentID)
	}
}

func TestSendWithHMACSignature(t *testing.T) {
	secret := "test-secret-key"
	var signature string
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature = r.Header.Get("X-Webhook-Signature")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := New(server.URL, secret, FormatGeneric, 5000)
	if err := e.Send(sampleAttribution()); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if signature == "" {
		t.Fatal("expected signature header")
	}
	if !VerifyHMAC(body, secret, signature) {
		t.Fatal("HMAC verification failed")
	}
}

func TestRetryOn5xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Fatalf("read request body: %v", err)
		}
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := New(server.URL, "", FormatGeneric, 5000)
	e.MaxRetry = 3
	if err := e.Send(sampleAttribution()); err != nil {
		t.Fatalf("send should succeed after retries: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestFailAfterMaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Fatalf("read request body: %v", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	e := New(server.URL, "", FormatGeneric, 5000)
	e.MaxRetry = 2
	if err := e.Send(sampleAttribution()); err == nil {
		t.Fatal("expected error after max retries")
	}
}

func TestPagerDutyFormat(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := New(server.URL, "", FormatPagerDuty, 5000)
	if err := e.Send(sampleAttribution()); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["event_action"] != "trigger" {
		t.Errorf("expected event_action=trigger, got %v", payload["event_action"])
	}
}

func TestOpsgenieFormat(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := New(server.URL, "", FormatOpsgenie, 5000)
	if err := e.Send(sampleAttribution()); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["alias"] != "inc-test-01" {
		t.Errorf("expected alias=inc-test-01, got %v", payload["alias"])
	}
	if payload["priority"] != "P2" {
		t.Errorf("expected priority=P2, got %v", payload["priority"])
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Fatalf("read request body: %v", err)
		}
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	e := New(server.URL, "", FormatGeneric, 5000)
	e.MaxRetry = 3
	if err := e.Send(sampleAttribution()); err == nil {
		t.Fatal("expected error on 4xx")
	}
	// 4xx is not retried (only 5xx is)
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt for 4xx, got %d", attempts)
	}
}

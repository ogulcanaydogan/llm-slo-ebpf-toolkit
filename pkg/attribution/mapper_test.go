package attribution

import (
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/semconv"
)

func TestMapFaultLabel(t *testing.T) {
	if got := MapFaultLabel("provider_throttle"); got != "provider_throttle" {
		t.Fatalf("unexpected mapping: %s", got)
	}
	if got := MapFaultLabel("network_partition"); got != "network_egress" {
		t.Fatalf("unexpected network_partition mapping: %s", got)
	}
	if got := MapFaultLabel("unknown_case"); got != "unknown" {
		t.Fatalf("unexpected unknown mapping: %s", got)
	}
}

func TestBuildAttribution(t *testing.T) {
	sample := FaultSample{
		IncidentID:    "inc-1",
		Timestamp:     time.Now().UTC(),
		Cluster:       "local",
		Namespace:     "default",
		Service:       "chat",
		FaultLabel:    "provider_throttle",
		Confidence:    0.92,
		BurnRate:      2.3,
		WindowMinutes: 5,
		RequestID:     "req-1",
		TraceID:       "trace-1",
	}
	result := BuildAttribution(sample)
	if result.PredictedFaultDomain != "provider_throttle" {
		t.Fatalf("unexpected fault domain: %s", result.PredictedFaultDomain)
	}
	if len(result.Evidence) == 0 {
		t.Fatal("expected evidence")
	}
}

func TestBuildAttributionDNSAddsDNSEvidence(t *testing.T) {
	sample := FaultSample{
		IncidentID:    "inc-2",
		Timestamp:     time.Now().UTC(),
		Cluster:       "local",
		Namespace:     "default",
		Service:       "chat",
		FaultLabel:    "dns_latency",
		Confidence:    0.88,
		BurnRate:      2.0,
		WindowMinutes: 5,
		RequestID:     "req-2",
		TraceID:       "trace-2",
	}
	result := BuildAttribution(sample)

	foundDNS := false
	foundConfidence := false
	for _, item := range result.Evidence {
		if item.Signal == semconv.AttrDNSLatencyMS {
			foundDNS = true
		}
		if item.Signal == semconv.AttrCorrelationConf {
			foundConfidence = true
		}
	}
	if !foundDNS {
		t.Fatalf("expected dns latency evidence")
	}
	if !foundConfidence {
		t.Fatalf("expected correlation confidence evidence")
	}
}

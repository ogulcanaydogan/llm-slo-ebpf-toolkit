package collector

import (
	"testing"
	"time"
)

func TestNormalizeSampleProducesEvents(t *testing.T) {
	sample := RawSample{
		Timestamp:        time.Now().UTC(),
		Cluster:          "local",
		Namespace:        "default",
		Workload:         "demo",
		Service:          "chat",
		Node:             "kind-control-plane",
		RequestID:        "req-1",
		TraceID:          "trace-1",
		TTFTMs:           300,
		RequestLatencyMs: 900,
		TokenTPS:         20,
		ErrorRate:        0.01,
		FaultLabel:       "provider_throttle",
	}

	events := NormalizeSample(sample)
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].SLIName != "ttft_ms" {
		t.Fatalf("expected first event ttft_ms, got %s", events[0].SLIName)
	}
	if events[1].Status != "warning" {
		t.Fatalf("expected warning status, got %s", events[1].Status)
	}
	if events[0].Labels["node"] != "kind-control-plane" {
		t.Fatalf("expected node label")
	}
	if events[0].Labels["fault_label"] != "provider_throttle" {
		t.Fatalf("expected fault label")
	}
}

func TestDependencyMarker(t *testing.T) {
	if DependencyMarker() == "" {
		t.Fatal("dependency marker should not be empty")
	}
}

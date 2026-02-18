package schema

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func schemaPath(t *testing.T, rel string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return filepath.Join(root, rel)
}

func TestValidateSLOEventSchema(t *testing.T) {
	event := SLOEvent{
		EventID:   "evt-1",
		Timestamp: time.Now().UTC(),
		Cluster:   "local",
		Namespace: "default",
		Workload:  "demo",
		Service:   "chat",
		RequestID: "req-1",
		SLIName:   "ttft_ms",
		SLIValue:  210,
		Unit:      "ms",
		Status:    "ok",
	}
	if err := ValidateAgainstSchema(schemaPath(t, "docs/contracts/v1/slo-event.schema.json"), event); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestValidateIncidentSchema(t *testing.T) {
	incident := IncidentAttribution{
		IncidentID:           "inc-1",
		Timestamp:            time.Now().UTC(),
		Cluster:              "local",
		Service:              "chat",
		PredictedFaultDomain: "provider_throttle",
		Confidence:           0.9,
		Evidence:             []Evidence{{Signal: "fault_label", Value: "provider_throttle", Source: "application"}},
		SLOImpact:            SLOImpact{SLI: "ttft_ms", BurnRate: 2.1, WindowMinutes: 5},
	}
	if err := ValidateAgainstSchema(schemaPath(t, "docs/contracts/v1/incident-attribution.schema.json"), incident); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestValidateProbeEventSchema(t *testing.T) {
	event := ProbeEventV1{
		TSUnixNano: time.Now().UTC().UnixNano(),
		Signal:     "dns_latency_ms",
		Node:       "kind-worker",
		Namespace:  "default",
		Pod:        "rag-service-0",
		Container:  "rag-service",
		PID:        1234,
		TID:        1234,
		ConnTuple: &ConnTuple{
			SrcIP:    "10.0.0.2",
			DstIP:    "10.0.0.53",
			SrcPort:  42424,
			DstPort:  53,
			Protocol: "udp",
		},
		Value:   23.5,
		Unit:    "ms",
		Status:  "ok",
		TraceID: "trace-123",
		SpanID:  "span-123",
	}
	if err := ValidateAgainstSchema(schemaPath(t, "docs/contracts/v1alpha1/probe-event.schema.json"), event); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

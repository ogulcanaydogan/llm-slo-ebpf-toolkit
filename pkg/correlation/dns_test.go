package correlation

import (
	"testing"
	"time"
)

func TestMatchTiers(t *testing.T) {
	baseTime := time.Now().UTC()

	span := SpanRef{
		TraceID:   "trace-1",
		Service:   "chat",
		Node:      "node-a",
		Pod:       "pod-a",
		PID:       42,
		ConnTuple: "10.0.0.1:1234->10.0.0.2:53/udp",
		Timestamp: baseTime,
	}

	tests := []struct {
		name       string
		signal     SignalRef
		confidence float64
		tier       string
	}{
		{
			name: "trace exact",
			signal: SignalRef{
				Signal:    "dns_latency_ms",
				TraceID:   "trace-1",
				Timestamp: baseTime.Add(1 * time.Second),
			},
			confidence: 1.0,
			tier:       "trace_id_exact",
		},
		{
			name: "pod pid",
			signal: SignalRef{
				Signal:    "dns_latency_ms",
				Pod:       "pod-a",
				PID:       42,
				Timestamp: baseTime.Add(50 * time.Millisecond),
			},
			confidence: 0.9,
			tier:       "pod_pid_100ms",
		},
		{
			name: "pod conn",
			signal: SignalRef{
				Signal:    "dns_latency_ms",
				Pod:       "pod-a",
				ConnTuple: "10.0.0.1:1234->10.0.0.2:53/udp",
				Timestamp: baseTime.Add(200 * time.Millisecond),
			},
			confidence: 0.8,
			tier:       "pod_conn_250ms",
		},
		{
			name: "service node debug-only",
			signal: SignalRef{
				Signal:    "dns_latency_ms",
				Service:   "chat",
				Node:      "node-a",
				Timestamp: baseTime.Add(400 * time.Millisecond),
			},
			confidence: 0.65,
			tier:       "service_node_500ms",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			decision := Match(span, tt.signal, DefaultWindow)
			if !decision.Matched {
				t.Fatalf("expected matched decision")
			}
			if decision.Confidence != tt.confidence {
				t.Fatalf("expected confidence %v got %v", tt.confidence, decision.Confidence)
			}
			if decision.Tier != tt.tier {
				t.Fatalf("expected tier %s got %s", tt.tier, decision.Tier)
			}
		})
	}
}

func TestEnrichDNSRespectsThreshold(t *testing.T) {
	baseTime := time.Now().UTC()
	span := SpanRef{
		Service:   "chat",
		Node:      "node-a",
		Timestamp: baseTime,
	}
	signal := SignalRef{
		Signal:    "dns_latency_ms",
		Service:   "chat",
		Node:      "node-a",
		Timestamp: baseTime.Add(100 * time.Millisecond),
		Value:     181.0,
	}

	out, decision := EnrichDNS(nil, span, signal, DefaultWindow, DefaultEnrichmentThreshold)
	if !decision.Matched {
		t.Fatalf("expected match")
	}
	if len(out) != 0 {
		t.Fatalf("service/node tier should not enrich because confidence is below threshold")
	}

	traceSpan := SpanRef{
		TraceID:   "trace-1",
		Timestamp: baseTime,
	}
	traceSignal := SignalRef{
		Signal:    "dns_latency_ms",
		TraceID:   "trace-1",
		Timestamp: baseTime.Add(200 * time.Millisecond),
		Value:     190.0,
	}
	out, decision = EnrichDNS(nil, traceSpan, traceSignal, DefaultWindow, DefaultEnrichmentThreshold)
	if !decision.Matched || decision.Confidence != 1.0 {
		t.Fatalf("expected exact trace-id match")
	}
	if out["llm.ebpf.dns.latency_ms"] != 190.0 {
		t.Fatalf("expected dns enrichment")
	}
	if out["llm.ebpf.correlation_confidence"] != 1.0 {
		t.Fatalf("expected confidence enrichment")
	}
}

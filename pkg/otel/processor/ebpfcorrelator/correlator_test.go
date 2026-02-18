package ebpfcorrelator

import (
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/semconv"
)

func TestEnrichDNSAttributes(t *testing.T) {
	c := New()
	now := time.Now().UTC()

	span := correlation.SpanRef{
		TraceID:   "trace-7",
		Timestamp: now,
	}
	signal := correlation.SignalRef{
		Signal:    "dns_latency_ms",
		TraceID:   "trace-7",
		Timestamp: now.Add(500 * time.Millisecond),
		Value:     165.0,
	}

	out, decision := c.EnrichDNSAttributes(nil, span, signal)
	if !decision.Matched {
		t.Fatalf("expected matched correlation")
	}
	if out[semconv.AttrDNSLatencyMS] != 165.0 {
		t.Fatalf("expected dns attribute")
	}
	if out[semconv.AttrCorrelationConf] != 1.0 {
		t.Fatalf("expected confidence attribute")
	}
}

func TestEnrichAttributesMultiSignalFanout(t *testing.T) {
	c := New()
	now := time.Now().UTC()
	span := correlation.SpanRef{
		TraceID:   "trace-9",
		Service:   "rag",
		Node:      "node-a",
		Pod:       "pod-a",
		PID:       101,
		ConnTuple: "10.0.0.2:42424->10.0.0.53:443/tcp",
		Timestamp: now,
	}

	result := c.EnrichAttributes(nil, span, []correlation.SignalRef{
		{
			Signal:    "dns_latency_ms",
			TraceID:   "trace-9",
			Service:   "rag",
			Node:      "node-a",
			Pod:       "pod-a",
			PID:       101,
			Timestamp: now.Add(10 * time.Millisecond),
			Value:     120,
		},
		{
			Signal:    "tcp_retransmits_total",
			TraceID:   "trace-9",
			Service:   "rag",
			Node:      "node-a",
			Pod:       "pod-a",
			PID:       101,
			Timestamp: now.Add(15 * time.Millisecond),
			Value:     5,
		},
		{
			Signal:    "runqueue_delay_ms",
			TraceID:   "trace-9",
			Service:   "rag",
			Node:      "node-a",
			Pod:       "pod-a",
			PID:       101,
			Timestamp: now.Add(20 * time.Millisecond),
			Value:     22,
		},
		{
			Signal:    "connect_latency_ms",
			TraceID:   "trace-9",
			Service:   "rag",
			Node:      "node-a",
			Pod:       "pod-a",
			PID:       101,
			Timestamp: now.Add(22 * time.Millisecond),
			Value:     180,
		},
	})

	if len(result.Candidates) != 3 {
		t.Fatalf("expected fanout limit 3, got %d", len(result.Candidates))
	}
	if result.Debug.FanoutDropped != 1 {
		t.Fatalf("expected one fanout drop, got %d", result.Debug.FanoutDropped)
	}
	if _, ok := result.Attributes[semconv.AttrDNSLatencyMS]; !ok {
		t.Fatal("expected dns attribute")
	}
	if _, ok := result.Attributes[semconv.AttrTCPRetransmits]; !ok {
		t.Fatal("expected retransmits attribute")
	}
	if _, ok := result.Attributes[semconv.AttrRunqueueDelayMS]; !ok {
		t.Fatal("expected runqueue attribute")
	}
	if _, ok := result.Attributes[semconv.AttrConnectLatencyMS]; ok {
		t.Fatal("expected connect latency to be dropped by fanout")
	}
}

func TestEnrichAttributesThresholdAndDebug(t *testing.T) {
	c := New()
	now := time.Now().UTC()
	span := correlation.SpanRef{
		Service:   "svc-a",
		Node:      "node-a",
		Pod:       "pod-a",
		Timestamp: now,
	}

	result := c.EnrichAttributes(nil, span, []correlation.SignalRef{
		{
			Signal:    "dns_latency_ms",
			Service:   "svc-a",
			Node:      "node-a",
			Pod:       "pod-a",
			Timestamp: now.Add(300 * time.Millisecond), // matches 0.65 tier only
			Value:     99,
		},
		{
			Signal:    "cpu_steal_pct",
			Service:   "svc-b", // unmatched
			Node:      "node-z",
			Pod:       "pod-z",
			Timestamp: now.Add(20 * time.Millisecond),
			Value:     9,
		},
		{
			Signal:    "unknown_signal",
			TraceID:   "trace-a",
			Timestamp: now,
			Value:     1,
		},
	})

	if len(result.Candidates) != 0 {
		t.Fatalf("expected no enriched candidates, got %d", len(result.Candidates))
	}
	if result.Debug.LowConfidence != 1 {
		t.Fatalf("expected one low confidence signal, got %d", result.Debug.LowConfidence)
	}
	if result.Debug.Unmatched != 1 {
		t.Fatalf("expected one unmatched signal, got %d", result.Debug.Unmatched)
	}
	if result.Debug.UnsupportedType != 1 {
		t.Fatalf("expected one unsupported signal, got %d", result.Debug.UnsupportedType)
	}
}

func TestDecomposeRetrieval(t *testing.T) {
	attrs := map[string]float64{
		semconv.AttrDNSLatencyMS:     12.5,
		semconv.AttrConnectLatencyMS: 18.0,
		semconv.AttrTLSHandshakeMS:   22.0,
		semconv.AttrCPUStealPct:      0.6,
	}

	total := DecomposeRetrieval(attrs)
	expected := 12.5 + 18.0 + 22.0
	if total != expected {
		t.Fatalf("decompose total: got %f, want %f", total, expected)
	}
	if attrs[semconv.AttrRetrievalKernelMS] != expected {
		t.Fatalf("kernel_attributed_ms: got %f, want %f", attrs[semconv.AttrRetrievalKernelMS], expected)
	}
}

func TestDecomposeRetrievalPartial(t *testing.T) {
	attrs := map[string]float64{
		semconv.AttrDNSLatencyMS: 15.0,
	}

	total := DecomposeRetrieval(attrs)
	if total != 15.0 {
		t.Fatalf("partial decompose: got %f, want 15.0", total)
	}
}

func TestDecomposeRetrievalEmpty(t *testing.T) {
	attrs := map[string]float64{
		semconv.AttrCPUStealPct: 2.0,
	}

	total := DecomposeRetrieval(attrs)
	if total != 0 {
		t.Fatalf("empty decompose: got %f, want 0", total)
	}
	if _, ok := attrs[semconv.AttrRetrievalKernelMS]; ok {
		t.Fatal("should not set kernel_attributed_ms when total is 0")
	}
}

package ebpfcorrelator

import (
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
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
	if out["llm.ebpf.dns.latency_ms"] != 165.0 {
		t.Fatalf("expected dns attribute")
	}
}

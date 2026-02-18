package correlation

import (
	"math"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/semconv"
)

const (
	// DefaultWindow is the default correlation window from config defaults.
	DefaultWindow = 2 * time.Second

	// DefaultEnrichmentThreshold is the minimum confidence to enrich spans.
	DefaultEnrichmentThreshold = 0.7
)

// SpanRef is the minimal span metadata used for correlation.
type SpanRef struct {
	TraceID   string    `json:"trace_id,omitempty"`
	Service   string    `json:"service,omitempty"`
	Node      string    `json:"node,omitempty"`
	Pod       string    `json:"pod,omitempty"`
	PID       int       `json:"pid,omitempty"`
	ConnTuple string    `json:"conn_tuple,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// SignalRef is the normalized signal metadata.
type SignalRef struct {
	Signal    string    `json:"signal"`
	TraceID   string    `json:"trace_id,omitempty"`
	Service   string    `json:"service,omitempty"`
	Node      string    `json:"node,omitempty"`
	Pod       string    `json:"pod,omitempty"`
	PID       int       `json:"pid,omitempty"`
	ConnTuple string    `json:"conn_tuple,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Decision captures one correlation result.
type Decision struct {
	Matched    bool
	Confidence float64
	Tier       string
}

// Match computes confidence/tier for one span-signal pair.
func Match(span SpanRef, signal SignalRef, window time.Duration) Decision {
	if window <= 0 {
		window = DefaultWindow
	}
	if !withinWindow(span.Timestamp, signal.Timestamp, window) {
		return Decision{}
	}

	if span.TraceID != "" && span.TraceID == signal.TraceID {
		return Decision{Matched: true, Confidence: 1.0, Tier: "trace_id_exact"}
	}
	if span.Pod != "" && span.Pod == signal.Pod && span.PID > 0 && span.PID == signal.PID &&
		withinWindow(span.Timestamp, signal.Timestamp, 100*time.Millisecond) {
		return Decision{Matched: true, Confidence: 0.9, Tier: "pod_pid_100ms"}
	}
	if span.Pod != "" && span.Pod == signal.Pod && span.ConnTuple != "" && span.ConnTuple == signal.ConnTuple &&
		withinWindow(span.Timestamp, signal.Timestamp, 250*time.Millisecond) {
		return Decision{Matched: true, Confidence: 0.8, Tier: "pod_conn_250ms"}
	}
	if span.Service != "" && span.Service == signal.Service &&
		span.Node != "" && span.Node == signal.Node &&
		withinWindow(span.Timestamp, signal.Timestamp, 500*time.Millisecond) {
		return Decision{Matched: true, Confidence: 0.65, Tier: "service_node_500ms"}
	}

	return Decision{}
}

// EnrichDNS applies DNS attributes only when confidence >= threshold.
func EnrichDNS(
	base map[string]float64,
	span SpanRef,
	signal SignalRef,
	window time.Duration,
	threshold float64,
) (map[string]float64, Decision) {
	if base == nil {
		base = map[string]float64{}
	}
	if threshold <= 0 {
		threshold = DefaultEnrichmentThreshold
	}

	decision := Match(span, signal, window)
	if !decision.Matched || decision.Confidence < threshold {
		return base, decision
	}
	if signal.Signal != "dns_latency_ms" {
		return base, Decision{}
	}

	out := clone(base)
	out[semconv.AttrDNSLatencyMS] = signal.Value
	out[semconv.AttrCorrelationConf] = decision.Confidence
	return out, decision
}

func withinWindow(a, b time.Time, window time.Duration) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	diff := math.Abs(float64(a.Sub(b)))
	return diff <= float64(window)
}

func clone(src map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

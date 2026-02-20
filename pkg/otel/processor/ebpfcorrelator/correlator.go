package ebpfcorrelator

import (
	"math"
	"sort"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/semconv"
)

// Correlator is a lightweight helper for DNS signal enrichment.
type Correlator struct {
	WindowMS            int
	EnrichmentThreshold float64
	MaxJoinFanout       int
}

// DebugStats captures non-enriched correlation outcomes for diagnostics.
type DebugStats struct {
	Unmatched       int
	LowConfidence   int
	FanoutDropped   int
	UnsupportedType int
}

// Candidate is one matched signal and its correlation decision.
type Candidate struct {
	Signal   correlation.SignalRef
	Decision correlation.Decision
}

// EnrichmentResult contains enriched attributes and diagnostic counts.
type EnrichmentResult struct {
	Attributes map[string]float64
	Candidates []Candidate
	Debug      DebugStats
}

// New returns a correlator configured for plan defaults.
func New() Correlator {
	return Correlator{
		WindowMS:            2000,
		EnrichmentThreshold: 0.7,
		MaxJoinFanout:       3,
	}
}

// EnrichAttributes enriches a span from any supported signal set.
func (c Correlator) EnrichAttributes(
	base map[string]float64,
	span correlation.SpanRef,
	signals []correlation.SignalRef,
) EnrichmentResult {
	if base == nil {
		base = map[string]float64{}
	}
	window := time.Duration(c.WindowMS) * time.Millisecond
	threshold := c.EnrichmentThreshold
	if threshold <= 0 {
		threshold = correlation.DefaultEnrichmentThreshold
	}
	fanout := c.MaxJoinFanout
	if fanout <= 0 {
		fanout = 3
	}

	candidates := make([]Candidate, 0, len(signals))
	out := cloneMap(base)
	debug := DebugStats{}

	for _, signal := range signals {
		attr, supported := signalAttrKey(signal.Signal)
		if !supported {
			debug.UnsupportedType++
			continue
		}
		decision := correlation.Match(span, signal, window)
		if !decision.Matched {
			debug.Unmatched++
			continue
		}
		if decision.Confidence < threshold {
			debug.LowConfidence++
			continue
		}

		candidates = append(candidates, Candidate{Signal: signal, Decision: decision})
		_ = attr
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Decision.Confidence != candidates[j].Decision.Confidence {
			return candidates[i].Decision.Confidence > candidates[j].Decision.Confidence
		}
		di := math.Abs(float64(span.Timestamp.Sub(candidates[i].Signal.Timestamp)))
		dj := math.Abs(float64(span.Timestamp.Sub(candidates[j].Signal.Timestamp)))
		return di < dj
	})

	if len(candidates) > fanout {
		debug.FanoutDropped = len(candidates) - fanout
		candidates = candidates[:fanout]
	}

	maxConfidence := 0.0
	for _, candidate := range candidates {
		attr, _ := signalAttrKey(candidate.Signal.Signal)
		if existing, ok := out[attr]; !ok || candidate.Signal.Value > existing {
			out[attr] = candidate.Signal.Value
		}
		if candidate.Decision.Confidence > maxConfidence {
			maxConfidence = candidate.Decision.Confidence
		}
	}
	if maxConfidence > 0 {
		out[semconv.AttrCorrelationConf] = maxConfidence
	}

	return EnrichmentResult{
		Attributes: out,
		Candidates: candidates,
		Debug:      debug,
	}
}

// EnrichDNSAttributes enriches span attributes if correlation confidence passes threshold.
func (c Correlator) EnrichDNSAttributes(
	base map[string]float64,
	span correlation.SpanRef,
	signal correlation.SignalRef,
) (map[string]float64, correlation.Decision) {
	result := c.EnrichAttributes(base, span, []correlation.SignalRef{signal})
	if len(result.Candidates) == 0 {
		if len(result.Attributes) == 0 {
			result.Attributes = map[string]float64{}
		}
		return result.Attributes, correlation.Decision{}
	}
	return result.Attributes, result.Candidates[0].Decision
}

func signalAttrKey(signal string) (string, bool) {
	switch signal {
	case "dns_latency_ms":
		return semconv.AttrDNSLatencyMS, true
	case "tcp_retransmits_total":
		return semconv.AttrTCPRetransmits, true
	case "runqueue_delay_ms":
		return semconv.AttrRunqueueDelayMS, true
	case "connect_latency_ms":
		return semconv.AttrConnectLatencyMS, true
	case "tls_handshake_ms":
		return semconv.AttrTLSHandshakeMS, true
	case "cpu_steal_pct":
		return semconv.AttrCPUStealPct, true
	case "cfs_throttled_ms":
		return semconv.AttrCFSThrottledMS, true
	case "mem_reclaim_latency_ms":
		return semconv.AttrMemReclaimLatencyMS, true
	case "disk_io_latency_ms":
		return semconv.AttrDiskIOLatencyMS, true
	case "syscall_latency_ms":
		return semconv.AttrSyscallLatencyMS, true
	default:
		return "", false
	}
}

// DecomposeRetrieval sums kernel-attributed retrieval latency components
// (DNS + connect + TLS handshake) from enriched attributes and sets the
// aggregate llm.ebpf.retrieval.kernel_attributed_ms attribute. This
// enables operators to see what fraction of retrieval latency is
// attributable to kernel-observable network operations.
func DecomposeRetrieval(attrs map[string]float64) float64 {
	var total float64
	for _, key := range []string{
		semconv.AttrDNSLatencyMS,
		semconv.AttrConnectLatencyMS,
		semconv.AttrTLSHandshakeMS,
	} {
		if v, ok := attrs[key]; ok {
			total += v
		}
	}
	if total > 0 {
		attrs[semconv.AttrRetrievalKernelMS] = total
	}
	return total
}

func cloneMap(src map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

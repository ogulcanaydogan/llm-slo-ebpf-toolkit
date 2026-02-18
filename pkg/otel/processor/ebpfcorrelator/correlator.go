package ebpfcorrelator

import (
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
)

// Correlator is a lightweight helper for DNS signal enrichment.
type Correlator struct {
	WindowMS            int
	EnrichmentThreshold float64
}

// New returns a correlator configured for plan defaults.
func New() Correlator {
	return Correlator{
		WindowMS:            2000,
		EnrichmentThreshold: 0.7,
	}
}

// EnrichDNSAttributes enriches span attributes if correlation confidence passes threshold.
func (c Correlator) EnrichDNSAttributes(
	base map[string]float64,
	span correlation.SpanRef,
	signal correlation.SignalRef,
) (map[string]float64, correlation.Decision) {
	window := time.Duration(c.WindowMS) * time.Millisecond
	return correlation.EnrichDNS(base, span, signal, window, c.EnrichmentThreshold)
}

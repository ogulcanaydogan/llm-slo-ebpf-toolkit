package ebpfcorrelator

import (
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
)

// SpanRecord is a lightweight span representation for batch correlation.
type SpanRecord struct {
	TraceID    string
	SpanID     string
	Service    string
	Node       string
	Pod        string
	PID        int
	ConnTuple  string
	Timestamp  time.Time
	Attributes map[string]float64
}

// ProcessedBatch contains enriched spans and aggregated debug diagnostics.
type ProcessedBatch struct {
	Spans []SpanRecord
	Debug DebugStats
}

// ProcessBatch applies correlation enrichment over a span batch.
func (c Correlator) ProcessBatch(spans []SpanRecord, signals []correlation.SignalRef) ProcessedBatch {
	result := ProcessedBatch{Spans: make([]SpanRecord, 0, len(spans))}
	for _, item := range spans {
		spanRef := correlation.SpanRef{
			TraceID:   item.TraceID,
			Service:   item.Service,
			Node:      item.Node,
			Pod:       item.Pod,
			PID:       item.PID,
			ConnTuple: item.ConnTuple,
			Timestamp: item.Timestamp,
		}

		enriched := c.EnrichAttributes(item.Attributes, spanRef, signals)
		DecomposeRetrieval(enriched.Attributes)
		item.Attributes = enriched.Attributes
		result.Spans = append(result.Spans, item)
		result.Debug = mergeDebug(result.Debug, enriched.Debug)
	}
	return result
}

func mergeDebug(left DebugStats, right DebugStats) DebugStats {
	return DebugStats{
		Unmatched:       left.Unmatched + right.Unmatched,
		LowConfidence:   left.LowConfidence + right.LowConfidence,
		FanoutDropped:   left.FanoutDropped + right.FanoutDropped,
		UnsupportedType: left.UnsupportedType + right.UnsupportedType,
	}
}

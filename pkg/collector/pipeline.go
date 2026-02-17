package collector

import (
	"fmt"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// RawSample represents a synthetic or collected LLM request signal.
type RawSample struct {
	Timestamp        time.Time `json:"timestamp"`
	Cluster          string    `json:"cluster"`
	Namespace        string    `json:"namespace"`
	Workload         string    `json:"workload"`
	Service          string    `json:"service"`
	RequestID        string    `json:"request_id"`
	TraceID          string    `json:"trace_id"`
	TTFTMs           float64   `json:"ttft_ms"`
	RequestLatencyMs float64   `json:"request_latency_ms"`
	TokenTPS         float64   `json:"token_throughput_tps"`
	ErrorRate        float64   `json:"error_rate"`
}

// NormalizeSample converts one raw sample into first-class SLO events.
func NormalizeSample(sample RawSample) []schema.SLOEvent {
	events := []schema.SLOEvent{
		buildEvent(sample, "ttft_ms", sample.TTFTMs, "ms", thresholdStatus(sample.TTFTMs, 500, 1000)),
		buildEvent(sample, "request_latency_ms", sample.RequestLatencyMs, "ms", thresholdStatus(sample.RequestLatencyMs, 700, 1500)),
		buildEvent(sample, "token_throughput_tps", sample.TokenTPS, "tps", inverseThresholdStatus(sample.TokenTPS, 30, 10)),
		buildEvent(sample, "error_rate", sample.ErrorRate, "ratio", thresholdStatus(sample.ErrorRate, 0.02, 0.05)),
	}
	return events
}

func buildEvent(sample RawSample, sli string, value float64, unit string, status string) schema.SLOEvent {
	return schema.SLOEvent{
		EventID:   fmt.Sprintf("%s-%s", sample.RequestID, sli),
		Timestamp: sample.Timestamp,
		Cluster:   sample.Cluster,
		Namespace: sample.Namespace,
		Workload:  sample.Workload,
		Service:   sample.Service,
		RequestID: sample.RequestID,
		TraceID:   sample.TraceID,
		SLIName:   sli,
		SLIValue:  value,
		Unit:      unit,
		Status:    status,
		Labels: map[string]string{
			"source": "synthetic",
		},
	}
}

func thresholdStatus(value float64, warning float64, breach float64) string {
	if value >= breach {
		return "breach"
	}
	if value >= warning {
		return "warning"
	}
	return "ok"
}

func inverseThresholdStatus(value float64, warning float64, breach float64) string {
	if value <= breach {
		return "breach"
	}
	if value <= warning {
		return "warning"
	}
	return "ok"
}

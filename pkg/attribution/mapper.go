package attribution

import (
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// FaultSample is the normalized benchmark input for attribution.
type FaultSample struct {
	IncidentID    string    `json:"incident_id"`
	Timestamp     time.Time `json:"timestamp"`
	Cluster       string    `json:"cluster"`
	Namespace     string    `json:"namespace"`
	Service       string    `json:"service"`
	FaultLabel    string    `json:"fault_label"`
	Confidence    float64   `json:"confidence"`
	BurnRate      float64   `json:"burn_rate"`
	WindowMinutes int       `json:"window_minutes"`
	RequestID     string    `json:"request_id"`
	TraceID       string    `json:"trace_id"`
}

// MapFaultLabel maps scenario labels into schema-constrained domains.
func MapFaultLabel(label string) string {
	switch label {
	case "dns_latency":
		return "network_dns"
	case "egress_drop":
		return "network_egress"
	case "cpu_throttle":
		return "cpu_throttle"
	case "memory_pressure":
		return "memory_pressure"
	case "provider_throttle":
		return "provider_throttle"
	case "provider_error":
		return "provider_error"
	case "retrieval_slowdown":
		return "retrieval_backend"
	default:
		return "unknown"
	}
}

// BuildAttribution converts one fault sample into an incident attribution record.
func BuildAttribution(sample FaultSample) schema.IncidentAttribution {
	domain := MapFaultLabel(sample.FaultLabel)
	return schema.IncidentAttribution{
		IncidentID:           sample.IncidentID,
		Timestamp:            sample.Timestamp,
		Cluster:              sample.Cluster,
		Namespace:            sample.Namespace,
		Service:              sample.Service,
		PredictedFaultDomain: domain,
		Confidence:           sample.Confidence,
		Evidence: []schema.Evidence{
			{
				Signal: "fault_label",
				Value:  sample.FaultLabel,
				Source: "application",
			},
			{
				Signal: "mapped_domain",
				Value:  domain,
				Source: "ebpf",
			},
		},
		SLOImpact: schema.SLOImpact{
			SLI:           "ttft_ms",
			BurnRate:      sample.BurnRate,
			WindowMinutes: sample.WindowMinutes,
		},
		TraceIDs:   []string{sample.TraceID},
		RequestIDs: []string{sample.RequestID},
	}
}

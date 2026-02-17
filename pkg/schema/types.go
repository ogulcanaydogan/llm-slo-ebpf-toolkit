package schema

import "time"

// SLOEvent is the normalized event envelope emitted by the collector.
type SLOEvent struct {
	EventID   string            `json:"event_id"`
	Timestamp time.Time         `json:"timestamp"`
	Cluster   string            `json:"cluster"`
	Namespace string            `json:"namespace"`
	Workload  string            `json:"workload"`
	Service   string            `json:"service"`
	RequestID string            `json:"request_id"`
	TraceID   string            `json:"trace_id,omitempty"`
	SLIName   string            `json:"sli_name"`
	SLIValue  float64           `json:"sli_value"`
	Unit      string            `json:"unit"`
	Status    string            `json:"status"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Evidence captures one observed signal for attribution.
type Evidence struct {
	Signal string      `json:"signal"`
	Value  interface{} `json:"value"`
	Source string      `json:"source"`
}

// SLOImpact describes burn impact for an attributed incident.
type SLOImpact struct {
	SLI           string  `json:"sli"`
	BurnRate      float64 `json:"burn_rate"`
	WindowMinutes int     `json:"window_minutes"`
}

// IncidentAttribution is the normalized attribution envelope.
type IncidentAttribution struct {
	IncidentID           string     `json:"incident_id"`
	Timestamp            time.Time  `json:"timestamp"`
	Cluster              string     `json:"cluster"`
	Namespace            string     `json:"namespace,omitempty"`
	Service              string     `json:"service"`
	PredictedFaultDomain string     `json:"predicted_fault_domain"`
	Confidence           float64    `json:"confidence"`
	Evidence             []Evidence `json:"evidence"`
	SLOImpact            SLOImpact  `json:"slo_impact"`
	TraceIDs             []string   `json:"trace_ids,omitempty"`
	RequestIDs           []string   `json:"request_ids,omitempty"`
}

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

// ConnTuple identifies one network flow tuple observed by probes.
type ConnTuple struct {
	SrcIP    string `json:"src_ip"`
	DstIP    string `json:"dst_ip"`
	SrcPort  int    `json:"src_port"`
	DstPort  int    `json:"dst_port"`
	Protocol string `json:"protocol"`
}

// ProbeEventV1 is the normalized probe envelope emitted by the node agent.
type ProbeEventV1 struct {
	TSUnixNano int64      `json:"ts_unix_nano"`
	Signal     string     `json:"signal"`
	Node       string     `json:"node"`
	Namespace  string     `json:"namespace"`
	Pod        string     `json:"pod"`
	Container  string     `json:"container"`
	PID        int        `json:"pid"`
	TID        int        `json:"tid"`
	ConnTuple  *ConnTuple `json:"conn_tuple,omitempty"`
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	Status     string     `json:"status"`
	TraceID    string     `json:"trace_id,omitempty"`
	SpanID     string     `json:"span_id,omitempty"`
	Errno      *int       `json:"errno,omitempty"`
	Confidence *float64   `json:"confidence,omitempty"`
}

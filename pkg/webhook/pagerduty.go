package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// PagerDuty Events API v2 payload.
type pagerDutyPayload struct {
	RoutingKey  string         `json:"routing_key"`
	EventAction string         `json:"event_action"`
	Payload     pdEventPayload `json:"payload"`
}

type pdEventPayload struct {
	Summary       string            `json:"summary"`
	Source        string            `json:"source"`
	Severity      string            `json:"severity"`
	Timestamp     string            `json:"timestamp"`
	Component     string            `json:"component"`
	Group         string            `json:"group"`
	CustomDetails map[string]string `json:"custom_details"`
}

// BuildPagerDutyPayload formats an IncidentAttribution as a PagerDuty Events v2 trigger.
func BuildPagerDutyPayload(attr schema.IncidentAttribution) ([]byte, string, error) {
	severity := "warning"
	if attr.Confidence >= 0.8 {
		severity = "critical"
	}

	evidenceStrs := make([]string, 0, len(attr.Evidence))
	for _, e := range attr.Evidence {
		evidenceStrs = append(evidenceStrs, fmt.Sprintf("%s=%v", e.Signal, e.Value))
	}

	payload := pagerDutyPayload{
		EventAction: "trigger",
		Payload: pdEventPayload{
			Summary:   fmt.Sprintf("[%s] %s fault detected (confidence=%.2f)", attr.Service, attr.PredictedFaultDomain, attr.Confidence),
			Source:    fmt.Sprintf("%s/%s", attr.Cluster, attr.Service),
			Severity:  severity,
			Timestamp: attr.Timestamp.Format("2006-01-02T15:04:05.000+0000"),
			Component: attr.Service,
			Group:     attr.Cluster,
			CustomDetails: map[string]string{
				"incident_id":  attr.IncidentID,
				"fault_domain": attr.PredictedFaultDomain,
				"confidence":   fmt.Sprintf("%.4f", attr.Confidence),
				"evidence":     strings.Join(evidenceStrs, "; "),
				"burn_rate":    fmt.Sprintf("%.2f", attr.SLOImpact.BurnRate),
			},
		},
	}

	data, err := json.Marshal(payload)
	return data, "application/json", err
}

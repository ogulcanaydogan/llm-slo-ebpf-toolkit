package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// Opsgenie Alert API payload.
type opsgeniePayload struct {
	Message     string            `json:"message"`
	Alias       string            `json:"alias"`
	Description string            `json:"description"`
	Priority    string            `json:"priority"`
	Source      string            `json:"source"`
	Tags        []string          `json:"tags"`
	Details     map[string]string `json:"details"`
	Entity      string            `json:"entity"`
}

// BuildOpsgeniePayload formats an IncidentAttribution as an Opsgenie alert.
func BuildOpsgeniePayload(attr schema.IncidentAttribution) ([]byte, string, error) {
	priority := "P3"
	if attr.Confidence >= 0.8 {
		priority = "P2"
	}
	if attr.SLOImpact.BurnRate >= 3.0 {
		priority = "P1"
	}

	evidenceStrs := make([]string, 0, len(attr.Evidence))
	for _, e := range attr.Evidence {
		evidenceStrs = append(evidenceStrs, fmt.Sprintf("%s=%v", e.Signal, e.Value))
	}

	payload := opsgeniePayload{
		Message:     fmt.Sprintf("[%s] %s fault detected", attr.Service, attr.PredictedFaultDomain),
		Alias:       attr.IncidentID,
		Description: fmt.Sprintf("Fault domain: %s\nConfidence: %.4f\nBurn rate: %.2f\nEvidence: %s", attr.PredictedFaultDomain, attr.Confidence, attr.SLOImpact.BurnRate, strings.Join(evidenceStrs, "; ")),
		Priority:    priority,
		Source:      "llm-slo-ebpf-toolkit",
		Tags:        []string{"llm-slo", attr.PredictedFaultDomain, attr.Cluster},
		Details: map[string]string{
			"incident_id":  attr.IncidentID,
			"cluster":      attr.Cluster,
			"service":      attr.Service,
			"fault_domain": attr.PredictedFaultDomain,
			"confidence":   fmt.Sprintf("%.4f", attr.Confidence),
			"burn_rate":    fmt.Sprintf("%.2f", attr.SLOImpact.BurnRate),
		},
		Entity: fmt.Sprintf("%s/%s", attr.Cluster, attr.Service),
	}

	data, err := json.Marshal(payload)
	return data, "application/json", err
}

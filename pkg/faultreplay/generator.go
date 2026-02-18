package faultreplay

import (
	"fmt"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/attribution"
)

var scenarioFaultLabels = map[string][]string{
	"provider_throttle": {"provider_throttle"},
	"dns_latency":       {"dns_latency"},
	"cpu_throttle":      {"cpu_throttle"},
	"memory_pressure":   {"memory_pressure"},
	"network_partition": {"network_partition"},
	"mixed":             {"provider_throttle", "dns_latency", "cpu_throttle", "memory_pressure", "network_partition"},
}

// GenerateFaultSamples creates deterministic synthetic fault samples for replay.
func GenerateFaultSamples(
	scenario string,
	count int,
	start time.Time,
) ([]attribution.FaultSample, error) {
	labels, ok := scenarioFaultLabels[scenario]
	if !ok {
		return nil, fmt.Errorf("unsupported scenario %q", scenario)
	}
	if count < 1 {
		return nil, fmt.Errorf("count must be >= 1")
	}

	samples := make([]attribution.FaultSample, 0, count)
	for idx := 0; idx < count; idx++ {
		label := labels[idx%len(labels)]
		timestamp := start.Add(time.Duration(idx) * time.Second)
		samples = append(samples, attribution.FaultSample{
			IncidentID:     fmt.Sprintf("replay-inc-%04d", idx+1),
			Timestamp:      timestamp,
			Cluster:        "local",
			Namespace:      "default",
			Service:        "chat",
			FaultLabel:     label,
			ExpectedDomain: attribution.MapFaultLabel(label),
			Confidence:     0.9,
			BurnRate:       2.0,
			WindowMinutes:  5,
			RequestID:      fmt.Sprintf("replay-req-%04d", idx+1),
			TraceID:        fmt.Sprintf("replay-trace-%04d", idx+1),
		})
	}

	return samples, nil
}

// SupportedScenarios lists accepted replay scenario names.
func SupportedScenarios() []string {
	return []string{
		"provider_throttle",
		"dns_latency",
		"cpu_throttle",
		"memory_pressure",
		"network_partition",
		"mixed",
	}
}

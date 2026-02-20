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
	if count < 1 {
		return nil, fmt.Errorf("count must be >= 1")
	}
	if scenario == "mixed_multi" {
		return generateMixedMultiSamples(count, start), nil
	}

	labels, ok := scenarioFaultLabels[scenario]
	if !ok {
		return nil, fmt.Errorf("unsupported scenario %q", scenario)
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

func generateMixedMultiSamples(count int, start time.Time) []attribution.FaultSample {
	pairs := [][2]string{
		{"provider_throttle", "dns_latency"},
		{"cpu_throttle", "memory_pressure"},
		{"network_partition", "dns_latency"},
		{"provider_throttle", "network_partition"},
	}

	samples := make([]attribution.FaultSample, 0, count)
	for idx := 0; idx < count; idx++ {
		pair := pairs[idx%len(pairs)]
		primary := pair[0]
		secondary := pair[1]
		timestamp := start.Add(time.Duration(idx) * time.Second)
		expectedDomains := uniqueDomains(
			attribution.MapFaultLabel(primary),
			attribution.MapFaultLabel(secondary),
		)
		expectedDomain := "unknown"
		if len(expectedDomains) > 0 {
			expectedDomain = expectedDomains[0]
		}

		samples = append(samples, attribution.FaultSample{
			IncidentID:      fmt.Sprintf("replay-inc-%04d", idx+1),
			Timestamp:       timestamp,
			Cluster:         "local",
			Namespace:       "default",
			Service:         "chat",
			FaultLabel:      primary,
			ExpectedDomain:  expectedDomain,
			ExpectedDomains: expectedDomains,
			Confidence:      0.9,
			BurnRate:        2.4,
			WindowMinutes:   5,
			RequestID:       fmt.Sprintf("replay-req-%04d", idx+1),
			TraceID:         fmt.Sprintf("replay-trace-%04d", idx+1),
		})
	}
	return samples
}

func uniqueDomains(domains ...string) []string {
	seen := make(map[string]bool, len(domains))
	out := make([]string, 0, len(domains))
	for _, domain := range domains {
		if domain == "" || domain == "unknown" || seen[domain] {
			continue
		}
		seen[domain] = true
		out = append(out, domain)
	}
	if len(out) == 0 {
		return []string{"unknown"}
	}
	return out
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
		"mixed_multi",
	}
}

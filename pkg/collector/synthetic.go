package collector

import (
	"fmt"
	"time"
)

// SampleMeta identifies the workload attributes attached to generated samples.
type SampleMeta struct {
	Cluster   string
	Namespace string
	Workload  string
	Service   string
	Node      string
}

var syntheticScenarioSequence = map[string][]string{
	"baseline":          {"baseline"},
	"provider_throttle": {"provider_throttle"},
	"dns_latency":       {"dns_latency"},
	"cpu_throttle":      {"cpu_throttle"},
	"memory_pressure":   {"memory_pressure"},
	"mixed":             {"provider_throttle", "dns_latency", "cpu_throttle", "memory_pressure"},
}

// SupportedSyntheticScenarios returns accepted synthetic scenario names.
func SupportedSyntheticScenarios() []string {
	return []string{
		"baseline",
		"provider_throttle",
		"dns_latency",
		"cpu_throttle",
		"memory_pressure",
		"mixed",
	}
}

// GenerateSyntheticSamples creates scenario-specific collector inputs.
func GenerateSyntheticSamples(
	scenario string,
	count int,
	start time.Time,
	meta SampleMeta,
) ([]RawSample, error) {
	if count < 1 {
		return nil, fmt.Errorf("count must be >= 1")
	}

	out := make([]RawSample, 0, count)
	for idx := 0; idx < count; idx++ {
		ts := start.Add(time.Duration(idx) * time.Second)
		sample, err := BuildSyntheticSample(scenario, idx, ts, meta)
		if err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	return out, nil
}

// BuildSyntheticSample returns one scenario-specific sample for the given index.
func BuildSyntheticSample(
	scenario string,
	idx int,
	timestamp time.Time,
	meta SampleMeta,
) (RawSample, error) {
	labels, ok := syntheticScenarioSequence[scenario]
	if !ok {
		return RawSample{}, fmt.Errorf("unsupported scenario %q", scenario)
	}
	faultLabel := labels[idx%len(labels)]
	return buildScenarioSample(meta, timestamp, idx, faultLabel), nil
}

func buildScenarioSample(meta SampleMeta, timestamp time.Time, idx int, faultLabel string) RawSample {
	requestID := fmt.Sprintf("collector-req-%04d", idx+1)
	traceID := fmt.Sprintf("collector-trace-%04d", idx+1)
	sample := RawSample{
		Timestamp:        timestamp,
		Cluster:          meta.Cluster,
		Namespace:        meta.Namespace,
		Workload:         meta.Workload,
		Service:          meta.Service,
		Node:             meta.Node,
		RequestID:        requestID,
		TraceID:          traceID,
		TTFTMs:           340,
		RequestLatencyMs: 720,
		TokenTPS:         36,
		ErrorRate:        0.005,
		FaultLabel:       faultLabel,
	}

	switch faultLabel {
	case "provider_throttle":
		sample.TTFTMs = 980
		sample.RequestLatencyMs = 2100
		sample.TokenTPS = 7
		sample.ErrorRate = 0.14
	case "dns_latency":
		sample.TTFTMs = 820
		sample.RequestLatencyMs = 1600
		sample.TokenTPS = 18
		sample.ErrorRate = 0.03
	case "cpu_throttle":
		sample.TTFTMs = 700
		sample.RequestLatencyMs = 1350
		sample.TokenTPS = 11
		sample.ErrorRate = 0.05
	case "memory_pressure":
		sample.TTFTMs = 650
		sample.RequestLatencyMs = 1250
		sample.TokenTPS = 13
		sample.ErrorRate = 0.04
	}
	return sample
}

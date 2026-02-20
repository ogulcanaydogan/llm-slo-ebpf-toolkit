package faultreplay

import (
	"testing"
	"time"
)

func TestGenerateFaultSamplesMixed(t *testing.T) {
	samples, err := GenerateFaultSamples("mixed", 6, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	if len(samples) != 6 {
		t.Fatalf("expected 6 samples, got %d", len(samples))
	}

	labels := []string{
		samples[0].FaultLabel,
		samples[1].FaultLabel,
		samples[2].FaultLabel,
		samples[4].FaultLabel,
	}
	if labels[0] != "provider_throttle" || labels[1] != "dns_latency" || labels[2] != "cpu_throttle" || labels[3] != "network_partition" {
		t.Fatalf("unexpected label order: %v", labels)
	}
}

func TestGenerateFaultSamplesRejectsUnsupportedScenario(t *testing.T) {
	if _, err := GenerateFaultSamples("unknown", 4, time.Now().UTC()); err == nil {
		t.Fatal("expected unsupported scenario error")
	}
}

func TestGenerateFaultSamplesMemoryPressure(t *testing.T) {
	samples, err := GenerateFaultSamples("memory_pressure", 3, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}
	if samples[0].FaultLabel != "memory_pressure" {
		t.Fatalf("expected memory_pressure label, got %s", samples[0].FaultLabel)
	}
}

func TestGenerateFaultSamplesNetworkPartition(t *testing.T) {
	samples, err := GenerateFaultSamples("network_partition", 2, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	if samples[0].FaultLabel != "network_partition" {
		t.Fatalf("expected network_partition label, got %s", samples[0].FaultLabel)
	}
}

func TestGenerateFaultSamplesMixedMulti(t *testing.T) {
	samples, err := GenerateFaultSamples("mixed_multi", 4, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	if len(samples) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(samples))
	}
	if len(samples[0].ExpectedDomains) < 2 {
		t.Fatalf("expected multi-fault expected_domains, got %+v", samples[0].ExpectedDomains)
	}
	if samples[0].ExpectedDomain == "" || samples[0].ExpectedDomain == "unknown" {
		t.Fatalf("expected primary expected_domain, got %q", samples[0].ExpectedDomain)
	}
}

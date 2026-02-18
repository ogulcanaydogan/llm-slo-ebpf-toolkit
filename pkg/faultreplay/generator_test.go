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
	}
	if labels[0] != "provider_throttle" || labels[1] != "dns_latency" || labels[2] != "cpu_throttle" {
		t.Fatalf("unexpected label order: %v", labels)
	}
}

func TestGenerateFaultSamplesRejectsUnsupportedScenario(t *testing.T) {
	if _, err := GenerateFaultSamples("unknown", 4, time.Now().UTC()); err == nil {
		t.Fatal("expected unsupported scenario error")
	}
}

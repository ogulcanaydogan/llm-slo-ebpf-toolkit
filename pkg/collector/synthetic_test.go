package collector

import (
	"testing"
	"time"
)

func TestGenerateSyntheticSamplesMixed(t *testing.T) {
	meta := SampleMeta{
		Cluster:   "local",
		Namespace: "default",
		Workload:  "gateway",
		Service:   "chat",
		Node:      "kind-control-plane",
	}
	samples, err := GenerateSyntheticSamples("mixed", 6, time.Unix(0, 0).UTC(), meta)
	if err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	if len(samples) != 6 {
		t.Fatalf("expected 6 samples, got %d", len(samples))
	}
	if samples[0].FaultLabel != "provider_throttle" {
		t.Fatalf("unexpected first fault label: %s", samples[0].FaultLabel)
	}
	if samples[0].Node != "kind-control-plane" {
		t.Fatalf("expected node label to propagate")
	}
}

func TestGenerateSyntheticSamplesRejectsUnknownScenario(t *testing.T) {
	meta := SampleMeta{}
	if _, err := GenerateSyntheticSamples("unknown", 2, time.Now().UTC(), meta); err == nil {
		t.Fatal("expected unsupported scenario error")
	}
}

func TestGenerateSyntheticSamplesMixedMulti(t *testing.T) {
	meta := SampleMeta{
		Cluster:   "local",
		Namespace: "default",
		Workload:  "gateway",
		Service:   "chat",
		Node:      "kind-control-plane",
	}
	samples, err := GenerateSyntheticSamples("mixed_multi", 2, time.Unix(0, 0).UTC(), meta)
	if err != nil {
		t.Fatalf("generate samples: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	if samples[0].FaultLabel != "mixed_multi" {
		t.Fatalf("expected mixed_multi fault label, got %s", samples[0].FaultLabel)
	}
	if samples[0].TTFTMs <= 1200 {
		t.Fatalf("expected elevated TTFT for mixed_multi, got %.2f", samples[0].TTFTMs)
	}
}

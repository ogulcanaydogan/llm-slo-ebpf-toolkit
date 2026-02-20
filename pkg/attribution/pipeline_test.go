package attribution

import (
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func TestBuildAttributionsCount(t *testing.T) {
	samples := []FaultSample{
		{
			IncidentID:    "inc-1",
			Timestamp:     time.Now().UTC(),
			Cluster:       "local",
			Namespace:     "default",
			Service:       "chat",
			FaultLabel:    "provider_throttle",
			Confidence:    0.9,
			BurnRate:      2.0,
			WindowMinutes: 5,
			RequestID:     "req-1",
			TraceID:       "trace-1",
		},
		{
			IncidentID:    "inc-2",
			Timestamp:     time.Now().UTC(),
			Cluster:       "local",
			Namespace:     "default",
			Service:       "chat",
			FaultLabel:    "dns_latency",
			Confidence:    0.8,
			BurnRate:      1.5,
			WindowMinutes: 5,
			RequestID:     "req-2",
			TraceID:       "trace-2",
		},
	}

	predictions := BuildAttributions(samples, AttributionModeRule)
	if len(predictions) != len(samples) {
		t.Fatalf("expected %d predictions, got %d", len(samples), len(predictions))
	}
}

func TestBuildConfusionMatrixRespectsExpectedDomain(t *testing.T) {
	samples := []FaultSample{
		{
			IncidentID:     "inc-1",
			Timestamp:      time.Now().UTC(),
			Cluster:        "local",
			Service:        "chat",
			FaultLabel:     "provider_throttle",
			ExpectedDomain: "network_dns",
		},
	}
	predictions := []schema.IncidentAttribution{
		{PredictedFaultDomain: "provider_throttle"},
	}

	matrix := BuildConfusionMatrix(samples, predictions)
	key := MatrixKey{Actual: "network_dns", Predicted: "provider_throttle"}
	if matrix[key] != 1 {
		t.Fatalf("expected matrix count 1 for %+v, got %d", key, matrix[key])
	}
}

func TestAccuracyWithMismatch(t *testing.T) {
	samples := []FaultSample{
		{FaultLabel: "provider_throttle"},
		{FaultLabel: "dns_latency"},
	}
	predictions := []schema.IncidentAttribution{
		{PredictedFaultDomain: "provider_throttle"},
		{PredictedFaultDomain: "provider_throttle"},
	}

	accuracy := Accuracy(samples, predictions)
	if accuracy != 0.5 {
		t.Fatalf("expected 0.5 accuracy, got %f", accuracy)
	}
}

func TestPartialAccuracySingleFault(t *testing.T) {
	samples := []FaultSample{
		{FaultLabel: "dns_latency", ExpectedDomains: []string{"network_dns"}},
		{FaultLabel: "cpu_throttle", ExpectedDomains: []string{"cpu_throttle"}},
	}
	predictions := []schema.IncidentAttribution{
		{PredictedFaultDomain: "network_dns"},
		{PredictedFaultDomain: "cpu_throttle"},
	}

	pa := PartialAccuracy(samples, predictions)
	if pa != 1.0 {
		t.Fatalf("expected 1.0 partial accuracy, got %f", pa)
	}
}

func TestPartialAccuracyMultiFault(t *testing.T) {
	samples := []FaultSample{
		{ExpectedDomains: []string{"network_dns", "cpu_throttle"}},
		{ExpectedDomains: []string{"provider_throttle", "memory_pressure"}},
	}
	predictions := []schema.IncidentAttribution{
		{PredictedFaultDomain: "cpu_throttle"}, // matches one of the expected
		{PredictedFaultDomain: "network_dns"},  // does not match either expected
	}

	pa := PartialAccuracy(samples, predictions)
	if pa != 0.5 {
		t.Fatalf("expected 0.5 partial accuracy, got %f", pa)
	}
}

func TestCoverageAccuracyFullCoverage(t *testing.T) {
	samples := []FaultSample{
		{ExpectedDomains: []string{"network_dns", "cpu_throttle"}},
	}
	predictions := []schema.IncidentAttribution{
		{
			PredictedFaultDomain: "network_dns",
			FaultHypotheses: []schema.FaultHypothesis{
				{Domain: "network_dns", Posterior: 0.5},
				{Domain: "cpu_throttle", Posterior: 0.3},
			},
		},
	}

	ca := CoverageAccuracy(samples, predictions, 0.1)
	if ca != 1.0 {
		t.Fatalf("expected 1.0 coverage accuracy, got %f", ca)
	}
}

func TestCoverageAccuracyPartialCoverage(t *testing.T) {
	samples := []FaultSample{
		{ExpectedDomains: []string{"network_dns", "cpu_throttle"}},
	}
	predictions := []schema.IncidentAttribution{
		{
			PredictedFaultDomain: "network_dns",
			FaultHypotheses: []schema.FaultHypothesis{
				{Domain: "network_dns", Posterior: 0.5},
				{Domain: "memory_pressure", Posterior: 0.3},
			},
		},
	}

	ca := CoverageAccuracy(samples, predictions, 0.1)
	if ca != 0.5 {
		t.Fatalf("expected 0.5 coverage accuracy, got %f", ca)
	}
}

func TestCoverageAccuracyThresholdFilters(t *testing.T) {
	samples := []FaultSample{
		{ExpectedDomains: []string{"network_dns", "cpu_throttle"}},
	}
	predictions := []schema.IncidentAttribution{
		{
			PredictedFaultDomain: "network_dns",
			FaultHypotheses: []schema.FaultHypothesis{
				{Domain: "network_dns", Posterior: 0.5},
				{Domain: "cpu_throttle", Posterior: 0.02},
			},
		},
	}

	// threshold 0.05 should filter out cpu_throttle (0.02)
	ca := CoverageAccuracy(samples, predictions, 0.05)
	if ca != 0.5 {
		t.Fatalf("expected 0.5 coverage accuracy with threshold, got %f", ca)
	}
}

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

	predictions := BuildAttributions(samples)
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

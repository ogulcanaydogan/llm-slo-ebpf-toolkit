package attribution

import "github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"

// MatrixKey represents one actual/predicted pair in confusion matrix output.
type MatrixKey struct {
	Actual    string
	Predicted string
}

// BuildAttributions converts a list of fault samples into attribution envelopes.
func BuildAttributions(samples []FaultSample) []schema.IncidentAttribution {
	out := make([]schema.IncidentAttribution, 0, len(samples))
	for _, sample := range samples {
		out = append(out, BuildAttribution(sample))
	}
	return out
}

// BuildConfusionMatrix returns matrix counts keyed by actual/predicted fault domain.
func BuildConfusionMatrix(
	samples []FaultSample,
	predictions []schema.IncidentAttribution,
) map[MatrixKey]int {
	matrix := make(map[MatrixKey]int)
	for idx, prediction := range predictions {
		if idx >= len(samples) {
			break
		}
		actual := samples[idx].ExpectedDomain
		if actual == "" {
			actual = MapFaultLabel(samples[idx].FaultLabel)
		}
		key := MatrixKey{Actual: actual, Predicted: prediction.PredictedFaultDomain}
		matrix[key]++
	}
	return matrix
}

// Accuracy returns ratio of correctly predicted fault domains in [0,1].
func Accuracy(samples []FaultSample, predictions []schema.IncidentAttribution) float64 {
	if len(predictions) == 0 {
		return 0
	}

	correct := 0
	for idx, prediction := range predictions {
		if idx >= len(samples) {
			break
		}
		actual := samples[idx].ExpectedDomain
		if actual == "" {
			actual = MapFaultLabel(samples[idx].FaultLabel)
		}
		if actual == prediction.PredictedFaultDomain {
			correct++
		}
	}
	return float64(correct) / float64(len(predictions))
}

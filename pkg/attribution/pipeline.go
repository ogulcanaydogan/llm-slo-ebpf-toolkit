package attribution

import "strings"

import "github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"

// MatrixKey represents one actual/predicted pair in confusion matrix output.
type MatrixKey struct {
	Actual    string
	Predicted string
}

const (
	AttributionModeBayes = "bayes"
	AttributionModeRule  = "rule"
)

// BuildAttributionsRule converts a list of fault samples into attribution envelopes
// using the rule-based mapper.
func BuildAttributionsRule(samples []FaultSample) []schema.IncidentAttribution {
	out := make([]schema.IncidentAttribution, 0, len(samples))
	for _, sample := range samples {
		out = append(out, BuildAttribution(sample))
	}
	return out
}

// BuildAttributionsBayes converts samples into attribution envelopes using
// Bayesian multi-fault inference. If attributor is nil, default priors and
// likelihoods are used.
func BuildAttributionsBayes(samples []FaultSample, attributor *BayesianAttributor) []schema.IncidentAttribution {
	if attributor == nil {
		attributor = NewBayesianAttributor()
	}
	out := make([]schema.IncidentAttribution, 0, len(samples))
	for _, sample := range samples {
		out = append(out, attributor.AttributeSample(sample))
	}
	return out
}

// BuildAttributions dispatches attribution mode.
// Supported values: "bayes", "rule". Unknown values fall back to "bayes".
func BuildAttributions(samples []FaultSample, mode string) []schema.IncidentAttribution {
	switch normalizeAttributionMode(mode) {
	case AttributionModeRule:
		return BuildAttributionsRule(samples)
	default:
		return BuildAttributionsBayes(samples, nil)
	}
}

func normalizeAttributionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AttributionModeRule:
		return AttributionModeRule
	case AttributionModeBayes:
		return AttributionModeBayes
	default:
		return AttributionModeBayes
	}
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

// PartialAccuracy returns the fraction of samples where the top-1 predicted
// domain appears in the expected_domains set. For single-fault samples this
// is equivalent to exact accuracy. For multi-fault samples partial credit is
// awarded if the top-1 prediction matches any expected domain.
func PartialAccuracy(samples []FaultSample, predictions []schema.IncidentAttribution) float64 {
	if len(predictions) == 0 {
		return 0
	}

	correct := 0
	for idx, prediction := range predictions {
		if idx >= len(samples) {
			break
		}
		expected := samples[idx].ExpectedDomains
		if len(expected) == 0 {
			ed := samples[idx].ExpectedDomain
			if ed == "" {
				ed = MapFaultLabel(samples[idx].FaultLabel)
			}
			expected = []string{ed}
		}
		for _, domain := range expected {
			if domain == prediction.PredictedFaultDomain {
				correct++
				break
			}
		}
	}
	return float64(correct) / float64(len(predictions))
}

// CoverageAccuracy returns the average fraction of expected domains that
// appear in the fault hypotheses above a given threshold.
func CoverageAccuracy(
	samples []FaultSample,
	predictions []schema.IncidentAttribution,
	threshold float64,
) float64 {
	if len(predictions) == 0 {
		return 0
	}

	total := 0.0
	count := 0
	for idx, prediction := range predictions {
		if idx >= len(samples) {
			break
		}
		expected := samples[idx].ExpectedDomains
		if len(expected) == 0 {
			ed := samples[idx].ExpectedDomain
			if ed == "" {
				ed = MapFaultLabel(samples[idx].FaultLabel)
			}
			expected = []string{ed}
		}

		hypothesisDomains := make(map[string]bool)
		for _, h := range prediction.FaultHypotheses {
			if h.Posterior >= threshold {
				hypothesisDomains[h.Domain] = true
			}
		}
		hypothesisDomains[prediction.PredictedFaultDomain] = true

		found := 0
		for _, domain := range expected {
			if hypothesisDomains[domain] {
				found++
			}
		}
		total += float64(found) / float64(len(expected))
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

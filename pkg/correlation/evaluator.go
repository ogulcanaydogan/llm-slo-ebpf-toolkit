package correlation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// LabeledPair is one ground-truth pair for correlation quality evaluation.
type LabeledPair struct {
	CaseID        string    `json:"case_id"`
	Span          SpanRef   `json:"span"`
	Signal        SignalRef `json:"signal"`
	ExpectedMatch bool      `json:"expected_match"`
	ExpectedTier  string    `json:"expected_tier,omitempty"`
}

// Prediction is one evaluator output row.
type Prediction struct {
	CaseID       string  `json:"case_id"`
	Expected     bool    `json:"expected"`
	Predicted    bool    `json:"predicted"`
	Confidence   float64 `json:"confidence"`
	Tier         string  `json:"tier"`
	Correct      bool    `json:"correct"`
	Signal       string  `json:"signal"`
	ExpectedTier string  `json:"expected_tier,omitempty"`
}

// EvalReport summarizes precision/recall/F1 and confusion counts.
type EvalReport struct {
	GeneratedAt     time.Time `json:"generated_at"`
	SampleSize      int       `json:"sample_size"`
	TruePositive    int       `json:"true_positive"`
	FalsePositive   int       `json:"false_positive"`
	FalseNegative   int       `json:"false_negative"`
	TrueNegative    int       `json:"true_negative"`
	Precision       float64   `json:"precision"`
	Recall          float64   `json:"recall"`
	F1              float64   `json:"f1"`
	TierAccuracy    float64   `json:"tier_accuracy"`
	MeanConfidence  float64   `json:"mean_confidence"`
	WindowMS        int       `json:"window_ms"`
	Threshold       float64   `json:"threshold"`
	MinPrecisionReq float64   `json:"min_precision_required,omitempty"`
	MinRecallReq    float64   `json:"min_recall_required,omitempty"`
	PassedGate      bool      `json:"passed_gate,omitempty"`
}

// GateResult captures gate verdict and details.
type GateResult struct {
	Pass    bool
	Message string
}

// LoadLabeledPairsFromJSONL loads evaluator inputs from a JSONL file.
func LoadLabeledPairsFromJSONL(path string) ([]LabeledPair, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open labeled pairs file: %w", err)
	}
	defer file.Close()

	out := make([]LabeledPair, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var pair LabeledPair
		if err := json.Unmarshal([]byte(line), &pair); err != nil {
			return nil, fmt.Errorf("parse labeled pair: %w", err)
		}
		out = append(out, pair)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan labeled pairs: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no labeled pairs loaded from %s", path)
	}
	return out, nil
}

// EvaluateLabeledPairs computes quality metrics at a given threshold/window.
func EvaluateLabeledPairs(
	pairs []LabeledPair,
	window time.Duration,
	threshold float64,
) (EvalReport, []Prediction) {
	if window <= 0 {
		window = DefaultWindow
	}
	if threshold <= 0 {
		threshold = DefaultEnrichmentThreshold
	}

	report := EvalReport{
		GeneratedAt: time.Now().UTC(),
		SampleSize:  len(pairs),
		WindowMS:    int(window / time.Millisecond),
		Threshold:   threshold,
	}
	predictions := make([]Prediction, 0, len(pairs))

	tierCorrect := 0
	tierComparable := 0
	confSum := 0.0
	confCount := 0

	for _, pair := range pairs {
		decision := Match(pair.Span, pair.Signal, window)
		predicted := decision.Matched && decision.Confidence >= threshold
		correct := predicted == pair.ExpectedMatch
		predictions = append(predictions, Prediction{
			CaseID:       pair.CaseID,
			Expected:     pair.ExpectedMatch,
			Predicted:    predicted,
			Confidence:   decision.Confidence,
			Tier:         decision.Tier,
			Correct:      correct,
			Signal:       pair.Signal.Signal,
			ExpectedTier: pair.ExpectedTier,
		})

		if predicted {
			confSum += decision.Confidence
			confCount++
		}

		switch {
		case pair.ExpectedMatch && predicted:
			report.TruePositive++
		case !pair.ExpectedMatch && predicted:
			report.FalsePositive++
		case pair.ExpectedMatch && !predicted:
			report.FalseNegative++
		default:
			report.TrueNegative++
		}

		if pair.ExpectedMatch && pair.ExpectedTier != "" && predicted {
			tierComparable++
			if pair.ExpectedTier == decision.Tier {
				tierCorrect++
			}
		}
	}

	report.Precision = safeDiv(report.TruePositive, report.TruePositive+report.FalsePositive)
	report.Recall = safeDiv(report.TruePositive, report.TruePositive+report.FalseNegative)
	if report.Precision+report.Recall > 0 {
		report.F1 = 2 * ((report.Precision * report.Recall) / (report.Precision + report.Recall))
	}
	if tierComparable > 0 {
		report.TierAccuracy = float64(tierCorrect) / float64(tierComparable)
	}
	if confCount > 0 {
		report.MeanConfidence = confSum / float64(confCount)
	}

	return report, predictions
}

// EvaluateGate checks whether report passes required precision/recall thresholds.
func EvaluateGate(report EvalReport, minPrecision float64, minRecall float64) GateResult {
	report.MinPrecisionReq = minPrecision
	report.MinRecallReq = minRecall

	if report.Precision < minPrecision {
		return GateResult{
			Pass:    false,
			Message: fmt.Sprintf("precision gate failed: got %.4f required %.4f", report.Precision, minPrecision),
		}
	}
	if report.Recall < minRecall {
		return GateResult{
			Pass:    false,
			Message: fmt.Sprintf("recall gate failed: got %.4f required %.4f", report.Recall, minRecall),
		}
	}
	return GateResult{Pass: true, Message: "correlation gate passed"}
}

func safeDiv(num int, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

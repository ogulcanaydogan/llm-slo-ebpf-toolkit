package correlation

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvaluateLabeledPairs(t *testing.T) {
	now := time.Now().UTC()
	pairs := []LabeledPair{
		{
			CaseID:        "tp-trace",
			ExpectedMatch: true,
			ExpectedTier:  "trace_id_exact",
			Span: SpanRef{
				TraceID:   "trace-1",
				Timestamp: now,
			},
			Signal: SignalRef{
				Signal:    "dns_latency_ms",
				TraceID:   "trace-1",
				Timestamp: now.Add(10 * time.Millisecond),
			},
		},
		{
			CaseID:        "fn-low-confidence",
			ExpectedMatch: true,
			ExpectedTier:  "service_node_500ms",
			Span: SpanRef{
				Service:   "svc-a",
				Node:      "node-a",
				Timestamp: now,
			},
			Signal: SignalRef{
				Signal:    "dns_latency_ms",
				Service:   "svc-a",
				Node:      "node-a",
				Timestamp: now.Add(100 * time.Millisecond),
			},
		},
		{
			CaseID:        "tn-unmatched",
			ExpectedMatch: false,
			Span: SpanRef{
				Service:   "svc-a",
				Node:      "node-a",
				Timestamp: now,
			},
			Signal: SignalRef{
				Signal:    "dns_latency_ms",
				Service:   "svc-b",
				Node:      "node-b",
				Timestamp: now.Add(10 * time.Millisecond),
			},
		},
	}

	report, predictions := EvaluateLabeledPairs(pairs, 2*time.Second, 0.7)
	if len(predictions) != 3 {
		t.Fatalf("expected 3 predictions, got %d", len(predictions))
	}
	if report.TruePositive != 1 || report.FalseNegative != 1 || report.TrueNegative != 1 {
		t.Fatalf("unexpected confusion counts: %+v", report)
	}
	if report.Precision != 1.0 {
		t.Fatalf("expected precision 1.0 got %.4f", report.Precision)
	}
	if report.Recall != 0.5 {
		t.Fatalf("expected recall 0.5 got %.4f", report.Recall)
	}
}

func TestEvaluateGate(t *testing.T) {
	report := EvalReport{Precision: 0.91, Recall: 0.86}
	gate := EvaluateGate(report, 0.9, 0.85)
	if !gate.Pass {
		t.Fatalf("expected pass, got: %s", gate.Message)
	}

	fail := EvaluateGate(report, 0.95, 0.85)
	if fail.Pass {
		t.Fatal("expected precision gate fail")
	}
}

func TestLoadLabeledPairsFromJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pairs.jsonl")
	content := `{"case_id":"a","span":{"trace_id":"t1","timestamp":"2026-02-18T00:00:00Z"},"signal":{"signal":"dns_latency_ms","trace_id":"t1","timestamp":"2026-02-18T00:00:00.050Z"},"expected_match":true}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	pairs, err := LoadLabeledPairsFromJSONL(path)
	if err != nil {
		t.Fatalf("load pairs: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
}

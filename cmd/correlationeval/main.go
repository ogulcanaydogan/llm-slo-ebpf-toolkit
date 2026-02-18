package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
)

func main() {
	inputPath := flag.String(
		"input",
		filepath.Join("pkg", "correlation", "testdata", "labeled_pairs.jsonl"),
		"labeled correlation dataset JSONL",
	)
	outPath := flag.String(
		"out",
		filepath.Join("artifacts", "correlation", "eval_summary.json"),
		"summary JSON output path",
	)
	predictionsPath := flag.String(
		"predictions-out",
		filepath.Join("artifacts", "correlation", "predictions.csv"),
		"predictions CSV output path",
	)
	windowMS := flag.Int("window-ms", 2000, "correlation window in milliseconds")
	threshold := flag.Float64("threshold", 0.7, "minimum confidence to count as positive correlation")
	minPrecision := flag.Float64("min-precision", 0.90, "minimum precision gate")
	minRecall := flag.Float64("min-recall", 0.85, "minimum recall gate")
	flag.Parse()

	pairs, err := correlation.LoadLabeledPairsFromJSONL(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load labeled dataset failed: %v\n", err)
		os.Exit(1)
	}

	report, predictions := correlation.EvaluateLabeledPairs(
		pairs,
		time.Duration(*windowMS)*time.Millisecond,
		*threshold,
	)
	gate := correlation.EvaluateGate(report, *minPrecision, *minRecall)
	report.MinPrecisionReq = *minPrecision
	report.MinRecallReq = *minRecall
	report.PassedGate = gate.Pass

	if err := writeSummary(*outPath, report); err != nil {
		fmt.Fprintf(os.Stderr, "write summary failed: %v\n", err)
		os.Exit(1)
	}
	if err := writePredictionsCSV(*predictionsPath, predictions); err != nil {
		fmt.Fprintf(os.Stderr, "write predictions failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(
		"correlation gate: %s | precision=%.4f recall=%.4f f1=%.4f sample_size=%d\n",
		boolWord(gate.Pass),
		report.Precision,
		report.Recall,
		report.F1,
		report.SampleSize,
	)
	fmt.Printf("summary: %s\n", *outPath)
	fmt.Printf("predictions: %s\n", *predictionsPath)
	if !gate.Pass {
		fmt.Fprintln(os.Stderr, gate.Message)
		os.Exit(1)
	}
}

func writeSummary(path string, report correlation.EvalReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}

func writePredictionsCSV(path string, predictions []correlation.Prediction) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"case_id",
		"signal",
		"expected_match",
		"predicted_match",
		"confidence",
		"tier",
		"expected_tier",
		"is_correct",
	}); err != nil {
		return err
	}
	for _, prediction := range predictions {
		if err := writer.Write([]string{
			prediction.CaseID,
			prediction.Signal,
			strconv.FormatBool(prediction.Expected),
			strconv.FormatBool(prediction.Predicted),
			fmt.Sprintf("%.4f", prediction.Confidence),
			prediction.Tier,
			prediction.ExpectedTier,
			strconv.FormatBool(prediction.Correct),
		}); err != nil {
			return err
		}
	}
	return nil
}

func boolWord(v bool) string {
	if v {
		return "PASS"
	}
	return "FAIL"
}

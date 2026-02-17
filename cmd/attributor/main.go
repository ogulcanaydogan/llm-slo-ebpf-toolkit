package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/attribution"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

type summaryPayload struct {
	GeneratedAt   string         `json:"generated_at"`
	TotalSamples  int            `json:"total_samples"`
	Accuracy      float64        `json:"accuracy"`
	DomainCounts  map[string]int `json:"predicted_domain_counts"`
	InputPath     string         `json:"input_path,omitempty"`
	OutputPath    string         `json:"output_path,omitempty"`
	ConfusionPath string         `json:"confusion_path,omitempty"`
}

func main() {
	inputPath := flag.String("input", "", "JSONL file containing fault samples")
	outPath := flag.String("out", "-", "Attribution JSONL output path ('-' for stdout)")
	summaryPath := flag.String("summary-out", "", "Optional JSON summary output path")
	confusionPath := flag.String("confusion-out", "", "Optional confusion matrix CSV output path")
	schemaPath := flag.String(
		"schema",
		filepath.Join("docs", "contracts", "v1", "incident-attribution.schema.json"),
		"Incident attribution JSON schema path",
	)
	flag.Parse()

	samples, err := loadSamples(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load samples: %v\n", err)
		os.Exit(1)
	}

	predictions := attribution.BuildAttributions(samples)
	for _, prediction := range predictions {
		if err := schema.ValidateAgainstSchema(*schemaPath, prediction); err != nil {
			fmt.Fprintf(os.Stderr, "schema validation failed: %v\n", err)
			os.Exit(1)
		}
	}

	if err := writeAttributionsJSONL(*outPath, predictions); err != nil {
		fmt.Fprintf(os.Stderr, "failed writing attributions: %v\n", err)
		os.Exit(1)
	}

	if *confusionPath != "" {
		if err := writeConfusionCSV(*confusionPath, samples, predictions); err != nil {
			fmt.Fprintf(os.Stderr, "failed writing confusion matrix: %v\n", err)
			os.Exit(1)
		}
	}

	if *summaryPath != "" {
		if err := writeSummaryJSON(*summaryPath, *inputPath, *outPath, *confusionPath, samples, predictions); err != nil {
			fmt.Fprintf(os.Stderr, "failed writing summary: %v\n", err)
			os.Exit(1)
		}
	}
}

func loadSamples(inputPath string) ([]attribution.FaultSample, error) {
	if inputPath == "" {
		return []attribution.FaultSample{
			{
				IncidentID:     "inc-1",
				Timestamp:      time.Now().UTC(),
				Cluster:        "local",
				Namespace:      "default",
				Service:        "chat",
				FaultLabel:     "provider_throttle",
				ExpectedDomain: "provider_throttle",
				Confidence:     0.9,
				BurnRate:       2.0,
				WindowMinutes:  5,
				RequestID:      "req-1",
				TraceID:        "trace-1",
			},
		}, nil
	}
	return attribution.LoadSamplesFromJSONL(inputPath)
}

func writeAttributionsJSONL(path string, predictions []schema.IncidentAttribution) error {
	writer, closeFn, err := openOutput(path)
	if err != nil {
		return err
	}
	defer closeFn()

	buffered := bufio.NewWriter(writer)
	defer buffered.Flush()

	encoder := json.NewEncoder(buffered)
	for _, prediction := range predictions {
		if err := encoder.Encode(prediction); err != nil {
			return fmt.Errorf("encode prediction: %w", err)
		}
	}
	return nil
}

func openOutput(path string) (*os.File, func() error, error) {
	if path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file: %w", err)
	}
	return file, file.Close, nil
}

func writeConfusionCSV(
	path string,
	samples []attribution.FaultSample,
	predictions []schema.IncidentAttribution,
) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create confusion output directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create confusion matrix file: %w", err)
	}
	defer file.Close()

	matrix := attribution.BuildConfusionMatrix(samples, predictions)
	keys := make([]attribution.MatrixKey, 0, len(matrix))
	for key := range matrix {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Actual == keys[j].Actual {
			return keys[i].Predicted < keys[j].Predicted
		}
		return keys[i].Actual < keys[j].Actual
	})

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"actual", "predicted", "count"}); err != nil {
		return err
	}
	for _, key := range keys {
		if err := writer.Write([]string{key.Actual, key.Predicted, fmt.Sprintf("%d", matrix[key])}); err != nil {
			return err
		}
	}
	return nil
}

func writeSummaryJSON(
	path string,
	inputPath string,
	outputPath string,
	confusionPath string,
	samples []attribution.FaultSample,
	predictions []schema.IncidentAttribution,
) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create summary output directory: %w", err)
	}

	domainCounts := make(map[string]int)
	for _, prediction := range predictions {
		domainCounts[prediction.PredictedFaultDomain]++
	}

	summary := summaryPayload{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		TotalSamples:  len(samples),
		Accuracy:      attribution.Accuracy(samples, predictions),
		DomainCounts:  domainCounts,
		InputPath:     inputPath,
		OutputPath:    outputPath,
		ConfusionPath: confusionPath,
	}

	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write summary output: %w", err)
	}
	return nil
}

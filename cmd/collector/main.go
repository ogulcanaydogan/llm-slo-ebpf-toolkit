package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func main() {
	schemaPath := filepath.Join("docs", "contracts", "v1", "slo-event.schema.json")
	samples, err := readSamples(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read samples: %v\n", err)
		os.Exit(1)
	}

	if len(samples) == 0 {
		samples = append(samples, collector.RawSample{
			Timestamp:        time.Now().UTC(),
			Cluster:          "local",
			Namespace:        "default",
			Workload:         "demo",
			Service:          "chat",
			RequestID:        "req-1",
			TraceID:          "trace-1",
			TTFTMs:           420,
			RequestLatencyMs: 850,
			TokenTPS:         24,
			ErrorRate:        0.01,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	for _, sample := range samples {
		events := collector.NormalizeSample(sample)
		for _, event := range events {
			if err := schema.ValidateAgainstSchema(schemaPath, event); err != nil {
				fmt.Fprintf(os.Stderr, "schema validation failed: %v\n", err)
				os.Exit(1)
			}
			if err := encoder.Encode(event); err != nil {
				fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

func readSamples(file *os.File) ([]collector.RawSample, error) {
	scanner := bufio.NewScanner(file)
	samples := make([]collector.RawSample, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sample collector.RawSample
		if err := json.Unmarshal(line, &sample); err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return samples, nil
}

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func main() {
	inputPath := flag.String("input", "-", "raw sample input JSONL path or '-' for stdin")
	outputMode := flag.String("output", "stdout", "output mode: stdout|jsonl")
	outputPath := flag.String("output-path", "artifacts/collector/slo-events.jsonl", "output path when output=jsonl")
	cluster := flag.String("cluster", "local", "cluster name for synthetic generation")
	namespace := flag.String("namespace", "default", "namespace for synthetic generation")
	workload := flag.String("workload", "gateway", "workload for synthetic generation")
	service := flag.String("service", "chat", "service for synthetic generation")
	node := flag.String("k8s-node", "unknown-node", "node name label")
	scenario := flag.String("scenario", "baseline", "synthetic scenario name")
	count := flag.Int("count", 1, "synthetic sample count (0 = stream mode)")
	intervalMS := flag.Int("interval-ms", 1000, "stream interval milliseconds when count=0")
	flag.Parse()

	schemaPath := filepath.Join("docs", "contracts", "v1", "slo-event.schema.json")
	samples, err := loadInputSamples(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read samples: %v\n", err)
		os.Exit(1)
	}

	writer, closeFn, err := openOutput(*outputMode, *outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open output: %v\n", err)
		os.Exit(1)
	}
	defer closeFn()

	if len(samples) > 0 {
		if err := emitSamples(writer, schemaPath, samples); err != nil {
			fmt.Fprintf(os.Stderr, "emit failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	meta := collector.SampleMeta{
		Cluster:   *cluster,
		Namespace: *namespace,
		Workload:  *workload,
		Service:   *service,
		Node:      *node,
	}

	if *count < 0 {
		fmt.Fprintln(os.Stderr, "count must be >= 0")
		os.Exit(1)
	}

	if *count > 0 {
		synthetic, genErr := collector.GenerateSyntheticSamples(*scenario, *count, time.Now().UTC(), meta)
		if genErr != nil {
			fmt.Fprintf(os.Stderr, "generate synthetic samples failed: %v\n", genErr)
			os.Exit(1)
		}
		if err := emitSamples(writer, schemaPath, synthetic); err != nil {
			fmt.Fprintf(os.Stderr, "emit failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	interval := time.Duration(*intervalMS) * time.Millisecond
	if interval <= 0 {
		fmt.Fprintln(os.Stderr, "interval-ms must be > 0")
		os.Exit(1)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for idx := 0; ; idx++ {
		sample, genErr := collector.BuildSyntheticSample(*scenario, idx, time.Now().UTC(), meta)
		if genErr != nil {
			fmt.Fprintf(os.Stderr, "generate synthetic sample failed: %v\n", genErr)
			os.Exit(1)
		}
		if err := emitSamples(writer, schemaPath, []collector.RawSample{sample}); err != nil {
			fmt.Fprintf(os.Stderr, "emit failed: %v\n", err)
			os.Exit(1)
		}
		<-ticker.C
	}
}

func loadInputSamples(path string) ([]collector.RawSample, error) {
	if path == "-" {
		return readSamples(os.Stdin)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readSamples(file)
}

func emitSamples(writer io.Writer, schemaPath string, samples []collector.RawSample) error {
	encoder := json.NewEncoder(writer)
	for _, sample := range samples {
		events := collector.NormalizeSample(sample)
		for _, event := range events {
			if err := schema.ValidateAgainstSchema(schemaPath, event); err != nil {
				return err
			}
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
	}
	return nil
}

func openOutput(mode string, path string) (io.Writer, func(), error) {
	switch mode {
	case "stdout":
		return os.Stdout, func() {}, nil
	case "jsonl":
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, func() {}, err
		}
		file, err := os.Create(path)
		if err != nil {
			return nil, func() {}, err
		}
		return file, func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "close output file failed: %v\n", closeErr)
			}
		}, nil
	default:
		return nil, func() {}, fmt.Errorf("unsupported output mode %q", mode)
	}
}

func readSamples(reader io.Reader) ([]collector.RawSample, error) {
	scanner := bufio.NewScanner(reader)
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

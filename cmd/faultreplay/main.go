package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/faultreplay"
)

func main() {
	scenario := flag.String("scenario", "mixed", "fault scenario")
	count := flag.Int("count", 30, "number of samples to emit")
	out := flag.String("out", "artifacts/fault-replay/fault_samples.jsonl", "output JSONL path")
	flag.Parse()

	samples, err := faultreplay.GenerateFaultSamples(*scenario, *count, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate replay samples: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory failed: %v\n", err)
		os.Exit(1)
	}

	file, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output file failed: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, sample := range samples {
		if err := encoder.Encode(sample); err != nil {
			fmt.Fprintf(os.Stderr, "encode sample failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("wrote %d replay samples to %s\n", len(samples), *out)
}

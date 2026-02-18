package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
)

func main() {
	scenario := flag.String("scenario", "mixed", "fault injection scenario")
	count := flag.Int("count", 24, "number of raw samples to emit")
	out := flag.String(
		"out",
		"artifacts/fault-injection/raw_samples.jsonl",
		"output JSONL file for collector input",
	)
	cluster := flag.String("cluster", "local", "cluster label")
	namespace := flag.String("namespace", "default", "namespace label")
	workload := flag.String("workload", "gateway", "workload label")
	service := flag.String("service", "chat", "service label")
	node := flag.String("node", "kind-control-plane", "node label")
	flag.Parse()

	meta := collector.SampleMeta{
		Cluster:   *cluster,
		Namespace: *namespace,
		Workload:  *workload,
		Service:   *service,
		Node:      *node,
	}
	samples, err := collector.GenerateSyntheticSamples(*scenario, *count, time.Now().UTC(), meta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate fault-injection samples failed: %v\n", err)
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

	fmt.Printf("wrote %d raw samples to %s\n", len(samples), *out)
}

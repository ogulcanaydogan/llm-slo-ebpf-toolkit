package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/attribution"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/benchmark"
)

func main() {
	out := flag.String("out", "artifacts/benchmarks", "output directory")
	scenario := flag.String("scenario", "provider_throttle", "fault scenario")
	workload := flag.String("workload", "rag_mixed", "workload profile")
	input := flag.String("input", "", "optional JSONL fault sample input")
	attributionMode := flag.String("attribution-mode", attribution.AttributionModeBayes, "attribution mode: bayes|rule")
	flag.Parse()

	if err := benchmark.GenerateArtifactsWithOptions(*out, *scenario, *workload, *input, *attributionMode); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("benchmark artifacts written to %s\n", *out)
}

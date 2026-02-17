package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/attribution"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func main() {
	schemaPath := filepath.Join("docs", "contracts", "v1", "incident-attribution.schema.json")
	sample := attribution.FaultSample{
		IncidentID:    "inc-1",
		Timestamp:     time.Now().UTC(),
		Cluster:       "local",
		Namespace:     "default",
		Service:       "chat",
		FaultLabel:    "provider_throttle",
		Confidence:    0.9,
		BurnRate:      2.0,
		WindowMinutes: 5,
		RequestID:     "req-1",
		TraceID:       "trace-1",
	}

	result := attribution.BuildAttribution(sample)
	if err := schema.ValidateAgainstSchema(schemaPath, result); err != nil {
		fmt.Fprintf(os.Stderr, "schema validation failed: %v\n", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		os.Exit(1)
	}
}

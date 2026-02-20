package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/toolkitcfg"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

type check struct {
	name string
	run  func(root string) error
}

var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	root := projectRoot()
	checks := []check{
		{name: "schema document parse", run: validateSchemaDocuments},
		{name: "contract sample payloads", run: validateContractSamples},
		{name: "toolkit config schema", run: validateToolkitConfigAgainstSchema},
		{name: "toolkit config loader", run: validateToolkitConfigLoader},
	}

	for _, c := range checks {
		if err := c.run(root); err != nil {
			fmt.Fprintf(os.Stderr, "schema validation failed (%s): %v\n", c.name, err)
			os.Exit(1)
		}
		fmt.Printf("ok: %s\n", c.name)
	}
}

func validateSchemaDocuments(root string) error {
	paths := []string{
		filepath.Join(root, "docs", "contracts", "v1", "slo-event.schema.json"),
		filepath.Join(root, "docs", "contracts", "v1", "incident-attribution.schema.json"),
		filepath.Join(root, "docs", "contracts", "v1alpha1", "probe-event.schema.json"),
		filepath.Join(root, "config", "toolkit.schema.json"),
	}

	for _, path := range paths {
		if err := validateSchemaDocument(path); err != nil {
			return err
		}
	}
	return nil
}

func validateContractSamples(root string) error {
	now := time.Now().UTC()
	sloEvent := schema.SLOEvent{
		EventID:   "evt-schema-1",
		Timestamp: now,
		Cluster:   "local",
		Namespace: "default",
		Workload:  "gateway",
		Service:   "rag-service",
		RequestID: "req-schema-1",
		TraceID:   "trace-schema-1",
		SLIName:   "ttft_ms",
		SLIValue:  220,
		Unit:      "ms",
		Status:    "ok",
	}

	incident := schema.IncidentAttribution{
		IncidentID:           "inc-schema-1",
		Timestamp:            now,
		Cluster:              "local",
		Namespace:            "default",
		Service:              "rag-service",
		PredictedFaultDomain: "provider_throttle",
		Confidence:           0.92,
		Evidence: []schema.Evidence{{
			Signal: "llm.ebpf.tcp.retransmits",
			Value:  7,
			Source: "ebpf",
		}},
		SLOImpact: schema.SLOImpact{
			SLI:           "ttft_ms",
			BurnRate:      2.4,
			WindowMinutes: 5,
		},
		TraceIDs:   []string{"trace-schema-1"},
		RequestIDs: []string{"req-schema-1"},
		FaultHypotheses: []schema.FaultHypothesis{
			{Domain: "provider_throttle", Posterior: 0.8, Evidence: []string{"tcp_retransmits_total"}},
			{Domain: "network_dns", Posterior: 0.2, Evidence: []string{"dns_latency_ms"}},
		},
	}

	probe := schema.ProbeEventV1{
		TSUnixNano: now.UnixNano(),
		Signal:     "dns_latency_ms",
		Node:       "kind-worker",
		Namespace:  "default",
		Pod:        "rag-service-0",
		Container:  "rag-service",
		PID:        101,
		TID:        101,
		ConnTuple: &schema.ConnTuple{
			SrcIP:    "10.244.0.2",
			DstIP:    "10.96.0.10",
			SrcPort:  41000,
			DstPort:  53,
			Protocol: "udp",
		},
		Value:   17.4,
		Unit:    "ms",
		Status:  "ok",
		TraceID: "trace-schema-1",
		SpanID:  "span-schema-1",
	}

	checks := []struct {
		schemaPath string
		payload    interface{}
	}{
		{schemaPath: filepath.Join(root, "docs", "contracts", "v1", "slo-event.schema.json"), payload: sloEvent},
		{schemaPath: filepath.Join(root, "docs", "contracts", "v1", "incident-attribution.schema.json"), payload: incident},
		{schemaPath: filepath.Join(root, "docs", "contracts", "v1alpha1", "probe-event.schema.json"), payload: probe},
	}

	for _, c := range checks {
		if err := schema.ValidateAgainstSchema(c.schemaPath, c.payload); err != nil {
			return err
		}
	}

	return nil
}

func validateToolkitConfigAgainstSchema(root string) error {
	schemaPath := filepath.Join(root, "config", "toolkit.schema.json")
	configPath := filepath.Join(root, "config", "toolkit.yaml")

	payloadBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read toolkit config %s: %w", configPath, err)
	}

	var yamlPayload interface{}
	if err := yaml.Unmarshal(payloadBytes, &yamlPayload); err != nil {
		return fmt.Errorf("parse toolkit yaml %s: %w", configPath, err)
	}

	return validatePayloadAgainstSchema(schemaPath, normalizeYAML(yamlPayload))
}

func validateToolkitConfigLoader(root string) error {
	configPath := filepath.Join(root, "config", "toolkit.yaml")
	_, err := toolkitcfg.Load(configPath)
	if err != nil {
		return fmt.Errorf("load toolkit config %s: %w", configPath, err)
	}
	return nil
}

func validateSchemaDocument(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", path, err)
	}
	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse schema json %s: %w", path, err)
	}

	_, err = gojsonschema.NewSchema(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return fmt.Errorf("compile schema %s: %w", path, err)
	}
	return nil
}

func validatePayloadAgainstSchema(schemaPath string, payload interface{}) error {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", schemaPath, err)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload for %s: %w", schemaPath, err)
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schemaBytes),
		gojsonschema.NewBytesLoader(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("validate payload against %s: %w", schemaPath, err)
	}
	if result.Valid() {
		return nil
	}

	errs := make([]string, 0, len(result.Errors()))
	for _, issue := range result.Errors() {
		errs = append(errs, issue.String())
	}
	return fmt.Errorf("payload failed %s: %s", schemaPath, strings.Join(errs, "; "))
}

func normalizeYAML(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, value := range x {
			out[k] = normalizeYAML(value)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, value := range x {
			out[fmt.Sprint(k)] = normalizeYAML(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i := range x {
			out[i] = normalizeYAML(x[i])
		}
		return out
	default:
		return x
	}
}

func projectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

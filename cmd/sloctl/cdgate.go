package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/cdgate"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/toolkitcfg"
)

func runCDGate(args []string) {
	if len(args) == 0 {
		printCDGateUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "check":
		runCDGateCheck(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown cdgate subcommand %q\n", args[0])
		printCDGateUsage()
		os.Exit(2)
	}
}

func runCDGateCheck(args []string) {
	defaultConfigPath := filepath.Join("config", "toolkit.yaml")
	configPathValue := resolveCDGateConfigPath(args, defaultConfigPath)
	cfg := toolkitcfg.Default()
	if loaded, err := toolkitcfg.Load(configPathValue); err == nil {
		cfg = loaded
	} else {
		log.Printf("warning: failed to load config %s: %v (using defaults)", configPathValue, err)
	}
	cg := cfg.CDGate
	if cg.PrometheusURL == "" {
		cg.PrometheusURL = toolkitcfg.Default().CDGate.PrometheusURL
	}
	if cg.TTFTp95MS <= 0 {
		cg.TTFTp95MS = toolkitcfg.Default().CDGate.TTFTp95MS
	}
	if cg.ErrorRate <= 0 {
		cg.ErrorRate = toolkitcfg.Default().CDGate.ErrorRate
	}
	if cg.BurnRate <= 0 {
		cg.BurnRate = toolkitcfg.Default().CDGate.BurnRate
	}

	fs := flag.NewFlagSet("sloctl cdgate check", flag.ExitOnError)
	configPath := fs.String("config", configPathValue, "toolkit config path")
	promURL := fs.String("prometheus-url", cg.PrometheusURL, "Prometheus base URL")
	ttftP95 := fs.Float64("ttft-p95-ms", cg.TTFTp95MS, "TTFT p95 threshold in milliseconds")
	errorRate := fs.Float64("error-rate", cg.ErrorRate, "Error rate threshold (0-1)")
	burnRate := fs.Float64("burn-rate", cg.BurnRate, "Burn rate threshold")
	failOpen := fs.Bool("fail-open", cg.FailOpen, "Pass gate if Prometheus is unreachable")
	output := fs.String("output", "text", "Output mode: text|json")
	timeoutSec := fs.Int("timeout", 10, "Query timeout in seconds")
	_ = fs.Parse(args)

	if strings.TrimSpace(*configPath) != strings.TrimSpace(configPathValue) {
		if _, err := toolkitcfg.Load(*configPath); err != nil {
			log.Printf("warning: failed to load config %s: %v", *configPath, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	querier := &cdgate.HTTPQuerier{
		BaseURL: *promURL,
		Client:  &http.Client{Timeout: time.Duration(*timeoutSec) * time.Second},
	}

	thresholds := cdgate.Thresholds{
		TTFTp95MS: *ttftP95,
		ErrorRate: *errorRate,
		BurnRate:  *burnRate,
	}

	result := cdgate.EvaluateSLOGate(ctx, querier, thresholds)

	// Apply fail-open: if there's a query error and fail-open is set, treat as pass.
	if result.Error != "" && *failOpen {
		result.Pass = true
		result.Error = result.Error + " (fail-open: passing despite query error)"
	}

	switch *output {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal result: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	case "text":
		printCDGateTextResult(result)
	default:
		fmt.Fprintf(os.Stderr, "unsupported output mode %q\n", *output)
		os.Exit(2)
	}

	if !result.Pass {
		os.Exit(1)
	}
}

func resolveCDGateConfigPath(args []string, fallback string) string {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "--config" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
		}
	}
	return fallback
}

func printCDGateTextResult(result cdgate.Result) {
	fmt.Printf("timestamp: %s\n", result.Timestamp.Format(time.RFC3339))
	if result.Error != "" {
		fmt.Printf("error: %s\n", result.Error)
	}
	fmt.Println()

	if len(result.Violations) > 0 {
		fmt.Println("violations:")
		for _, v := range result.Violations {
			fmt.Printf("  - %s: actual=%.4f threshold=%.4f\n", v.Metric, v.Actual, v.Threshold)
		}
		fmt.Println()
	}

	if result.Pass {
		fmt.Println("result: PASS (all SLO metrics within thresholds)")
	} else {
		fmt.Println("result: FAIL (one or more SLO metrics exceeded thresholds)")
	}
}

func printCDGateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  sloctl cdgate check [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --config          Toolkit config path (default: config/toolkit.yaml)")
	fmt.Println("  --prometheus-url  Prometheus base URL (default: http://prometheus:9090)")
	fmt.Println("  --ttft-p95-ms     TTFT p95 threshold in ms (default: 800)")
	fmt.Println("  --error-rate      Error rate threshold 0-1 (default: 0.05)")
	fmt.Println("  --burn-rate       Burn rate threshold (default: 2.0)")
	fmt.Println("  --fail-open       Pass if Prometheus unreachable (default: true)")
	fmt.Println("  --output          Output mode: text|json (default: text)")
	fmt.Println("  --timeout         Query timeout in seconds (default: 10)")
}

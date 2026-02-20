package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/attribution"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/toolkitcfg"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/webhook"
)

type summaryPayload struct {
	GeneratedAt           string         `json:"generated_at"`
	TotalSamples          int            `json:"total_samples"`
	Accuracy              float64        `json:"accuracy"`
	AttributionMode       string         `json:"attribution_mode"`
	DomainCounts          map[string]int `json:"predicted_domain_counts"`
	WebhookEnabled        bool           `json:"webhook_enabled"`
	WebhookStrict         bool           `json:"webhook_strict"`
	WebhookDeliveryErrors int            `json:"webhook_delivery_errors"`
	InputPath             string         `json:"input_path,omitempty"`
	OutputPath            string         `json:"output_path,omitempty"`
	ConfusionPath         string         `json:"confusion_path,omitempty"`
}

func main() {
	defaultConfigPath := filepath.Join("config", "toolkit.yaml")
	configPathValue := resolveConfigPath(os.Args[1:], defaultConfigPath)
	cfg := toolkitcfg.Default()
	if loaded, err := toolkitcfg.Load(configPathValue); err == nil {
		cfg = loaded
	} else {
		log.Printf("warning: failed to load config %s: %v (using defaults)", configPathValue, err)
	}

	inputPath := flag.String("input", "", "JSONL file containing fault samples")
	outPath := flag.String("out", "-", "Attribution JSONL output path ('-' for stdout)")
	summaryPath := flag.String("summary-out", "", "Optional JSON summary output path")
	confusionPath := flag.String("confusion-out", "", "Optional confusion matrix CSV output path")
	schemaPath := flag.String(
		"schema",
		filepath.Join("docs", "contracts", "v1", "incident-attribution.schema.json"),
		"Incident attribution JSON schema path",
	)
	configPath := flag.String("config", configPathValue, "toolkit config path")
	attributionMode := flag.String("attribution-mode", attribution.AttributionModeBayes, "attribution mode: bayes|rule")
	webhookEnabled := flag.Bool("webhook-enabled", cfg.Webhook.Enabled, "enable webhook delivery")
	webhookURL := flag.String("webhook-url", cfg.Webhook.URL, "webhook endpoint URL")
	webhookSecret := flag.String("webhook-secret", cfg.Webhook.Secret, "webhook secret for HMAC signature")
	webhookFormat := flag.String("webhook-format", cfg.Webhook.Format, "webhook format: generic|pagerduty|opsgenie")
	webhookTimeoutMS := flag.Int("webhook-timeout-ms", cfg.Webhook.TimeoutMS, "webhook timeout in milliseconds")
	webhookStrict := flag.Bool("webhook-strict", false, "fail command when webhook delivery fails")
	flag.Parse()

	// Reload if the parsed config path differs from the pre-resolved path.
	if strings.TrimSpace(*configPath) != strings.TrimSpace(configPathValue) {
		if loaded, err := toolkitcfg.Load(*configPath); err == nil {
			cfg = loaded
		} else {
			log.Printf("warning: failed to load config %s: %v (continuing with previous defaults)", *configPath, err)
		}
	}

	samples, err := loadSamples(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load samples: %v\n", err)
		os.Exit(1)
	}

	predictions := attribution.BuildAttributions(samples, *attributionMode)
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

	webhookErrCount := 0
	if *webhookEnabled {
		if strings.TrimSpace(*webhookURL) == "" {
			msg := "webhook delivery enabled but webhook-url is empty"
			if *webhookStrict {
				fmt.Fprintln(os.Stderr, msg)
				os.Exit(1)
			}
			log.Printf("warning: %s", msg)
		} else {
			format, parseErr := parseWebhookFormat(*webhookFormat)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "invalid webhook-format: %v\n", parseErr)
				os.Exit(2)
			}
			exporter := webhook.New(*webhookURL, *webhookSecret, format, *webhookTimeoutMS)
			for _, prediction := range predictions {
				if err := exporter.Send(prediction); err != nil {
					webhookErrCount++
					if *webhookStrict {
						fmt.Fprintf(os.Stderr, "webhook delivery failed: %v\n", err)
						os.Exit(1)
					}
					log.Printf("warning: webhook delivery failed for incident %s: %v", prediction.IncidentID, err)
				}
			}
		}
	}

	if *summaryPath != "" {
		if err := writeSummaryJSON(
			*summaryPath,
			*inputPath,
			*outPath,
			*confusionPath,
			*attributionMode,
			*webhookEnabled,
			*webhookStrict,
			webhookErrCount,
			samples,
			predictions,
		); err != nil {
			fmt.Fprintf(os.Stderr, "failed writing summary: %v\n", err)
			os.Exit(1)
		}
	}
}

func resolveConfigPath(args []string, fallback string) string {
	for idx := 0; idx < len(args); idx++ {
		arg := strings.TrimSpace(args[idx])
		if arg == "--config" && idx+1 < len(args) {
			return strings.TrimSpace(args[idx+1])
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
		}
	}
	return fallback
}

func parseWebhookFormat(raw string) (webhook.Format, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(webhook.FormatGeneric):
		return webhook.FormatGeneric, nil
	case string(webhook.FormatPagerDuty):
		return webhook.FormatPagerDuty, nil
	case string(webhook.FormatOpsgenie):
		return webhook.FormatOpsgenie, nil
	default:
		return "", fmt.Errorf("unsupported format %q", raw)
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

func writeAttributionsJSONL(path string, predictions []schema.IncidentAttribution) (err error) {
	writer, closeFn, err := openOutput(path)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := closeFn()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

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
	attributionMode string,
	webhookEnabled bool,
	webhookStrict bool,
	webhookErrCount int,
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
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		TotalSamples:          len(samples),
		Accuracy:              attribution.Accuracy(samples, predictions),
		AttributionMode:       attributionMode,
		DomainCounts:          domainCounts,
		WebhookEnabled:        webhookEnabled,
		WebhookStrict:         webhookStrict,
		WebhookDeliveryErrors: webhookErrCount,
		InputPath:             inputPath,
		OutputPath:            outputPath,
		ConfusionPath:         confusionPath,
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

package benchmark

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GenerateArtifacts writes benchmark skeleton outputs.
func GenerateArtifacts(outDir string, scenario string, workloadProfile string) error {
	metricsDir := filepath.Join(outDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		return fmt.Errorf("create metrics directory: %w", err)
	}

	summary := map[string]interface{}{
		"run_id":           time.Now().UTC().Format("2006-01-02T15-04-05Z"),
		"project":          "llm-slo-ebpf-toolkit",
		"scenario":         scenario,
		"workload_profile": workloadProfile,
		"environment": map[string]interface{}{
			"kubernetes_version": "1.31",
			"kernel_version":     "6.x",
			"node_count":         1,
		},
		"metrics": map[string]interface{}{
			"detection_delay_seconds_median": 2.5,
			"attribution_accuracy":           0.9,
			"false_positive_rate":            0.05,
			"false_negative_rate":            0.05,
			"burn_rate_prediction_error":     0.07,
			"collector_cpu_overhead_pct":     2.2,
			"collector_memory_overhead_mb":   120,
		},
	}

	summaryBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "attribution_summary.json"), summaryBytes, 0o644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	confusionPath := filepath.Join(outDir, "confusion-matrix.csv")
	confusionFile, err := os.Create(confusionPath)
	if err != nil {
		return fmt.Errorf("create confusion matrix: %w", err)
	}
	defer confusionFile.Close()

	writer := csv.NewWriter(confusionFile)
	defer writer.Flush()
	if err := writer.Write([]string{"actual", "predicted", "count"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"provider_throttle", "provider_throttle", "10"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"network_dns", "network_dns", "8"}); err != nil {
		return err
	}

	predictionsPath := filepath.Join(metricsDir, "incident_predictions.csv")
	predictionsFile, err := os.Create(predictionsPath)
	if err != nil {
		return fmt.Errorf("create predictions csv: %w", err)
	}
	defer predictionsFile.Close()

	predictionsWriter := csv.NewWriter(predictionsFile)
	defer predictionsWriter.Flush()
	headers := []string{
		"timestamp",
		"incident_id",
		"scenario",
		"fault_start_ts",
		"fault_end_ts",
		"predicted_fault_domain",
		"ground_truth_fault_domain",
		"confidence",
		"detection_delay_seconds",
		"is_correct",
	}
	if err := predictionsWriter.Write(headers); err != nil {
		return err
	}
	if err := predictionsWriter.Write([]string{
		time.Now().UTC().Format(time.RFC3339),
		"inc-1",
		scenario,
		time.Now().UTC().Add(-30 * time.Second).Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
		"provider_throttle",
		"provider_throttle",
		"0.93",
		"2.5",
		"true",
	}); err != nil {
		return err
	}

	return nil
}

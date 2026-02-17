package benchmark

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateArtifacts(t *testing.T) {
	tmp := t.TempDir()
	if err := GenerateArtifacts(tmp, "provider_throttle", "rag_mixed"); err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}

	required := []string{
		"attribution_summary.json",
		"confusion-matrix.csv",
		"incident_predictions.csv",
		"collector_overhead.csv",
		"provenance.json",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(tmp, name)); err != nil {
			t.Fatalf("missing artifact %s: %v", name, err)
		}
	}
}

func TestGenerateArtifactsRejectsUnsupportedScenario(t *testing.T) {
	tmp := t.TempDir()
	if err := GenerateArtifacts(tmp, "not_a_scenario", "rag_mixed"); err == nil {
		t.Fatal("expected unsupported scenario error")
	}
}

func TestGenerateArtifactsMixedFaultScenario(t *testing.T) {
	tmp := t.TempDir()
	if err := GenerateArtifacts(tmp, "mixed_faults", "rag_mixed"); err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}

	rows := readCSVRows(t, filepath.Join(tmp, "confusion-matrix.csv"))
	content := strings.Join(rows, "|")
	if !strings.Contains(content, "network_dns") {
		t.Fatal("expected network_dns in confusion matrix")
	}
	if !strings.Contains(content, "provider_throttle") {
		t.Fatal("expected provider_throttle in confusion matrix")
	}
}

func TestGenerateArtifactsWithInputFixture(t *testing.T) {
	tmp := t.TempDir()
	inputPath := filepath.Join("testdata", "mixed_fault_samples.jsonl")
	if err := GenerateArtifactsWithInput(tmp, "provider_throttle", "rag_mixed", inputPath); err != nil {
		t.Fatalf("generate artifacts with input: %v", err)
	}

	rows := readCSVRows(t, filepath.Join(tmp, "incident_predictions.csv"))
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows including header, got %d", len(rows))
	}
}

func readCSVRows(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	rows := make([]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, strings.Join(record, ","))
	}
	return rows
}

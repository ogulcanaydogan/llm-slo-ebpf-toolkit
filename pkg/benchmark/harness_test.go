package benchmark

import (
	"os"
	"path/filepath"
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
		filepath.Join("metrics", "incident_predictions.csv"),
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(tmp, name)); err != nil {
			t.Fatalf("missing artifact %s: %v", name, err)
		}
	}
}

package toolkitcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolkit.yaml")
	content := `
apiVersion: toolkit.llm-slo.dev/v1alpha1
kind: ToolkitConfig
signal_set:
  - dns_latency_ms
  - tcp_retransmits_total
sampling:
  events_per_second_limit: 500
  burst_limit: 1000
correlation:
  window_ms: 1500
otlp:
  endpoint: http://localhost:4317
safety:
  max_overhead_pct: 4
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Sampling.EventsPerSecondLimit != 500 {
		t.Fatalf("unexpected rate limit: %d", cfg.Sampling.EventsPerSecondLimit)
	}
	if cfg.Safety.MaxOverheadPct != 4 {
		t.Fatalf("unexpected overhead: %f", cfg.Safety.MaxOverheadPct)
	}
	if len(cfg.SignalSet) != 2 {
		t.Fatalf("unexpected signal count: %d", len(cfg.SignalSet))
	}
}

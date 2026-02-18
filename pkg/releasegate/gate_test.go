package releasegate

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
)

func TestEvaluatePass(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")

	for i := 1; i <= 3; i++ {
		ttft := []float64{100, 110, 120, 130, 140, 150}
		if i == 2 {
			ttft = []float64{102, 112, 122, 132, 142, 152}
		}
		if i == 3 {
			ttft = []float64{98, 108, 118, 128, 138, 148}
		}
		writeRun(t, candidateRoot, "dns_latency", i, ttft, []float64{30, 29, 31, 30, 30, 29}, 2.2)
	}
	writeRun(t, baselineRoot, "dns_latency", 1, []float64{100, 110, 120, 130, 140, 150}, []float64{30, 30, 30, 30, 30, 30}, 2.1)

	summary, err := Evaluate(Config{
		CandidateRoot:       candidateRoot,
		BaselineRoot:        baselineRoot,
		Scenarios:           []string{"dns_latency"},
		MaxOverheadPct:      3,
		MaxVariancePct:      10,
		MinRunsPerScenario:  3,
		RegressionPctLimit:  5,
		SignificanceAlpha:   0.05,
		BootstrapIterations: 200,
		BootstrapSeed:       42,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !summary.Pass {
		t.Fatalf("expected pass, failures: %v", summary.Failures)
	}
}

func TestEvaluateOverheadFail(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")

	for i := 1; i <= 3; i++ {
		cpu := 2.0
		if i == 2 {
			cpu = 3.4
		}
		writeRun(t, candidateRoot, "dns_latency", i, []float64{100, 110, 120, 130, 140, 150}, []float64{30, 30, 30, 30, 30, 30}, cpu)
	}
	writeRun(t, baselineRoot, "dns_latency", 1, []float64{100, 110, 120, 130, 140, 150}, []float64{30, 30, 30, 30, 30, 30}, 2.0)

	summary, err := Evaluate(Config{
		CandidateRoot:       candidateRoot,
		BaselineRoot:        baselineRoot,
		Scenarios:           []string{"dns_latency"},
		MaxOverheadPct:      3,
		MaxVariancePct:      10,
		MinRunsPerScenario:  3,
		RegressionPctLimit:  5,
		SignificanceAlpha:   0.05,
		BootstrapIterations: 200,
		BootstrapSeed:       42,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if summary.Pass {
		t.Fatal("expected overhead gate fail")
	}
	if summary.Overhead.Pass {
		t.Fatal("expected overhead sub-gate fail")
	}
}

func TestEvaluateSignificanceFail(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")

	for i := 1; i <= 3; i++ {
		writeRun(t, candidateRoot, "dns_latency", i,
			[]float64{210, 220, 230, 240, 250, 260, 270, 280, 290, 300},
			[]float64{20, 19, 18, 21, 20, 19, 18, 21, 20, 19},
			2.0,
		)
	}
	writeRun(t, baselineRoot, "dns_latency", 1,
		[]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190},
		[]float64{30, 31, 29, 30, 30, 31, 29, 30, 30, 31},
		2.0,
	)

	summary, err := Evaluate(Config{
		CandidateRoot:       candidateRoot,
		BaselineRoot:        baselineRoot,
		Scenarios:           []string{"dns_latency"},
		MaxOverheadPct:      3,
		MaxVariancePct:      30,
		MinRunsPerScenario:  3,
		RegressionPctLimit:  5,
		SignificanceAlpha:   0.05,
		BootstrapIterations: 400,
		BootstrapSeed:       42,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if summary.Significance.Pass {
		t.Fatal("expected significance gate fail")
	}
}

func writeRun(t *testing.T, root string, scenario string, run int, ttft []float64, tps []float64, cpu float64) {
	t.Helper()
	if len(ttft) != len(tps) {
		t.Fatalf("ttft/tps length mismatch")
	}

	runDir := filepath.Join(root, scenario, "run-"+strconv.Itoa(run))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	rawFile, err := os.Create(filepath.Join(runDir, "raw_samples.jsonl"))
	if err != nil {
		t.Fatalf("create raw file: %v", err)
	}
	enc := json.NewEncoder(rawFile)
	for i := range ttft {
		sample := collector.RawSample{
			Timestamp:        time.Unix(int64(i+1), 0).UTC(),
			Cluster:          "local",
			Namespace:        "default",
			Workload:         "w",
			Service:          "s",
			Node:             "n",
			RequestID:        "req-" + strconv.Itoa(i),
			TraceID:          "trace-" + strconv.Itoa(i),
			TTFTMs:           ttft[i],
			RequestLatencyMs: ttft[i] * 2,
			TokenTPS:         tps[i],
			ErrorRate:        0,
			FaultLabel:       scenario,
		}
		if err := enc.Encode(sample); err != nil {
			t.Fatalf("encode raw sample: %v", err)
		}
	}
	if err := rawFile.Close(); err != nil {
		t.Fatalf("close raw file: %v", err)
	}

	overheadFile, err := os.Create(filepath.Join(runDir, "collector_overhead.csv"))
	if err != nil {
		t.Fatalf("create overhead file: %v", err)
	}
	writer := csv.NewWriter(overheadFile)
	if err := writer.Write([]string{"timestamp", "node", "collector_cpu_pct", "collector_memory_mb", "events_per_second", "dropped_events"}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := writer.Write([]string{"2026-02-18T00:00:00Z", "node-a", strconv.FormatFloat(cpu, 'f', 2, 64), "120", "900", "0"}); err != nil {
		t.Fatalf("write row: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
	if err := overheadFile.Close(); err != nil {
		t.Fatalf("close overhead file: %v", err)
	}
}

package releasegate

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
)

type cpuRow struct {
	node string
	cpu  float64
}

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
		writeRunWithOptions(
			t,
			candidateRoot,
			"dns_latency",
			i,
			ttft,
			[]float64{30, 29, 31, 30, 30, 29},
			[]float64{0.01, 0.01, 0.009, 0.01, 0.011, 0.01},
			[]cpuRow{{node: "node-a", cpu: 2.2}, {node: "node-b", cpu: 2.4}},
		)
	}
	writeRunWithOptions(
		t,
		baselineRoot,
		"dns_latency",
		1,
		[]float64{100, 110, 120, 130, 140, 150},
		[]float64{30, 30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.1}, {node: "node-b", cpu: 2.3}},
	)

	summary, err := Evaluate(Config{
		CandidateRoot:         candidateRoot,
		BaselineRoot:          baselineRoot,
		Scenarios:             []string{"dns_latency"},
		MaxOverheadPct:        3,
		MaxVariancePct:        10,
		MinRunsPerScenario:    3,
		RegressionPctLimit:    5,
		SignificanceAlpha:     0.05,
		BootstrapIterations:   200,
		BootstrapSeed:         42,
		MinSamplesPerScenario: 6,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !summary.Pass {
		t.Fatalf("expected pass, failures: %v", summary.Failures)
	}
	if summary.Overhead.MaxNodeP95Node == "" {
		t.Fatal("expected max node p95 metadata")
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
		writeRunWithOptions(
			t,
			candidateRoot,
			"dns_latency",
			i,
			[]float64{100, 110, 120, 130, 140, 150},
			[]float64{30, 30, 30, 30, 30, 30},
			[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
			[]cpuRow{{node: "node-a", cpu: cpu}},
		)
	}
	writeRunWithOptions(
		t,
		baselineRoot,
		"dns_latency",
		1,
		[]float64{100, 110, 120, 130, 140, 150},
		[]float64{30, 30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.0}},
	)

	summary, err := Evaluate(Config{
		CandidateRoot:         candidateRoot,
		BaselineRoot:          baselineRoot,
		Scenarios:             []string{"dns_latency"},
		MaxOverheadPct:        3,
		MaxVariancePct:        10,
		MinRunsPerScenario:    3,
		RegressionPctLimit:    5,
		SignificanceAlpha:     0.05,
		BootstrapIterations:   200,
		BootstrapSeed:         42,
		MinSamplesPerScenario: 6,
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
	if !strings.Contains(summary.Overhead.FailureReason, "node") {
		t.Fatalf("expected node-aware overhead reason, got %q", summary.Overhead.FailureReason)
	}
}

func TestEvaluateVarianceTokenFail(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")

	writeRunWithOptions(
		t,
		candidateRoot,
		"dns_latency",
		1,
		[]float64{100, 100, 100, 100, 100, 100},
		[]float64{30, 30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.1}},
	)
	writeRunWithOptions(
		t,
		candidateRoot,
		"dns_latency",
		2,
		[]float64{100, 100, 100, 100, 100, 100},
		[]float64{10, 10, 10, 10, 10, 10},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.1}},
	)
	writeRunWithOptions(
		t,
		candidateRoot,
		"dns_latency",
		3,
		[]float64{100, 100, 100, 100, 100, 100},
		[]float64{45, 45, 45, 45, 45, 45},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.1}},
	)

	writeRunWithOptions(
		t,
		baselineRoot,
		"dns_latency",
		1,
		[]float64{100, 100, 100, 100, 100, 100},
		[]float64{30, 30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.0}},
	)

	summary, err := Evaluate(Config{
		CandidateRoot:         candidateRoot,
		BaselineRoot:          baselineRoot,
		Scenarios:             []string{"dns_latency"},
		MaxOverheadPct:        3,
		MaxVariancePct:        10,
		MinRunsPerScenario:    3,
		RegressionPctLimit:    100,
		SignificanceAlpha:     0.05,
		BootstrapIterations:   200,
		BootstrapSeed:         42,
		MinSamplesPerScenario: 6,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if summary.Variance.Pass {
		t.Fatal("expected variance gate fail")
	}
	if !strings.Contains(summary.Variance.Scenarios[0].FailureReason, "tokens variance") {
		t.Fatalf("expected token variance failure, got %q", summary.Variance.Scenarios[0].FailureReason)
	}
}

func TestEvaluateSignificanceFail(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")

	for i := 1; i <= 3; i++ {
		writeRunWithOptions(
			t,
			candidateRoot,
			"dns_latency",
			i,
			[]float64{210, 220, 230, 240, 250, 260, 270, 280, 290, 300},
			[]float64{20, 19, 18, 21, 20, 19, 18, 21, 20, 19},
			[]float64{0.03, 0.03, 0.03, 0.03, 0.03, 0.03, 0.03, 0.03, 0.03, 0.03},
			[]cpuRow{{node: "node-a", cpu: 2.0}},
		)
		writeRunWithOptions(
			t,
			baselineRoot,
			"dns_latency",
			i,
			[]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190},
			[]float64{30, 31, 29, 30, 30, 31, 29, 30, 30, 31},
			[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
			[]cpuRow{{node: "node-a", cpu: 2.0}},
		)
	}

	summary, err := Evaluate(Config{
		CandidateRoot:         candidateRoot,
		BaselineRoot:          baselineRoot,
		Scenarios:             []string{"dns_latency"},
		MaxOverheadPct:        3,
		MaxVariancePct:        30,
		MinRunsPerScenario:    3,
		RegressionPctLimit:    5,
		SignificanceAlpha:     0.05,
		BootstrapIterations:   400,
		BootstrapSeed:         42,
		MinSamplesPerScenario: 20,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if summary.Significance.Pass {
		t.Fatal("expected significance gate fail")
	}
	if summary.Significance.Scenarios[0].CliffsDelta <= 0 {
		t.Fatal("expected positive Cliff's delta for regression")
	}
}

func TestEvaluateSignificanceMinSamplesFail(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")

	for i := 1; i <= 3; i++ {
		writeRunWithOptions(
			t,
			candidateRoot,
			"dns_latency",
			i,
			[]float64{110, 120, 130, 140, 150},
			[]float64{30, 29, 31, 30, 29},
			[]float64{0.01, 0.01, 0.01, 0.01, 0.01},
			[]cpuRow{{node: "node-a", cpu: 2.0}},
		)
	}
	writeRunWithOptions(
		t,
		baselineRoot,
		"dns_latency",
		1,
		[]float64{100, 110, 120, 130, 140},
		[]float64{30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.0}},
	)

	summary, err := Evaluate(Config{
		CandidateRoot:         candidateRoot,
		BaselineRoot:          baselineRoot,
		Scenarios:             []string{"dns_latency"},
		MaxOverheadPct:        3,
		MaxVariancePct:        30,
		MinRunsPerScenario:    3,
		RegressionPctLimit:    5,
		SignificanceAlpha:     0.05,
		BootstrapIterations:   200,
		BootstrapSeed:         42,
		MinSamplesPerScenario: 30,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if summary.Significance.Pass {
		t.Fatal("expected significance gate fail due to sample size")
	}
	if !strings.Contains(summary.Significance.Scenarios[0].FailureReason, "insufficient samples") {
		t.Fatalf("expected insufficient samples reason, got %q", summary.Significance.Scenarios[0].FailureReason)
	}
}

func TestEvaluateBaselineManifestRequired(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")
	manifestPath := filepath.Join(baselineRoot, "manifest.json")

	for i := 1; i <= 3; i++ {
		writeRunWithOptions(
			t,
			candidateRoot,
			"dns_latency",
			i,
			[]float64{100, 110, 120, 130, 140, 150},
			[]float64{30, 30, 30, 30, 30, 30},
			[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
			[]cpuRow{{node: "node-a", cpu: 2.0}},
		)
	}
	writeRunWithOptions(
		t,
		baselineRoot,
		"dns_latency",
		1,
		[]float64{100, 110, 120, 130, 140, 150},
		[]float64{30, 30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.0}},
	)

	summary, err := Evaluate(Config{
		CandidateRoot:            candidateRoot,
		BaselineRoot:             baselineRoot,
		BaselineManifestPath:     manifestPath,
		RequireBaselineManifest:  true,
		CandidateRef:             "main",
		CandidateCommit:          "abc123",
		Scenarios:                []string{"dns_latency"},
		MaxOverheadPct:           3,
		MaxVariancePct:           10,
		MinRunsPerScenario:       3,
		RegressionPctLimit:       5,
		SignificanceAlpha:        0.05,
		BootstrapIterations:      100,
		BootstrapSeed:            42,
		MinSamplesPerScenario:    6,
		MinCliffsDeltaForFailure: 0.147,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if summary.Baseline.Pass {
		t.Fatal("expected baseline gate fail when manifest is missing")
	}
	if summary.Pass {
		t.Fatal("expected overall fail when baseline gate fails")
	}
}

func TestEvaluateBaselineSameSourcePassesGracefully(t *testing.T) {
	candidateRoot := filepath.Join(t.TempDir(), "candidate")
	baselineRoot := filepath.Join(t.TempDir(), "baseline")
	manifestPath := filepath.Join(baselineRoot, "manifest.json")

	for i := 1; i <= 3; i++ {
		writeRunWithOptions(
			t,
			candidateRoot,
			"dns_latency",
			i,
			[]float64{100, 110, 120, 130, 140, 150},
			[]float64{30, 30, 30, 30, 30, 30},
			[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
			[]cpuRow{{node: "node-a", cpu: 2.0}},
		)
	}
	writeRunWithOptions(
		t,
		baselineRoot,
		"dns_latency",
		1,
		[]float64{100, 110, 120, 130, 140, 150},
		[]float64{30, 30, 30, 30, 30, 30},
		[]float64{0.01, 0.01, 0.01, 0.01, 0.01, 0.01},
		[]cpuRow{{node: "node-a", cpu: 2.0}},
	)
	writeBaselineManifest(t, manifestPath, "main", "abc123")

	summary, err := Evaluate(Config{
		CandidateRoot:            candidateRoot,
		BaselineRoot:             baselineRoot,
		BaselineManifestPath:     manifestPath,
		RequireBaselineManifest:  true,
		CandidateRef:             "main",
		CandidateCommit:          "abc123",
		Scenarios:                []string{"dns_latency"},
		MaxOverheadPct:           3,
		MaxVariancePct:           10,
		MinRunsPerScenario:       3,
		RegressionPctLimit:       5,
		SignificanceAlpha:        0.05,
		BootstrapIterations:      100,
		BootstrapSeed:            42,
		MinSamplesPerScenario:    6,
		MinCliffsDeltaForFailure: 0.147,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !summary.Baseline.SameSource {
		t.Fatal("expected same source detection")
	}
	if !summary.Baseline.Pass {
		t.Fatal("same-source baseline should pass gracefully, not fail")
	}
	if !summary.Pass {
		t.Fatalf("overall gate should pass for same-source comparison, failures: %v", summary.Failures)
	}
}

func writeRunWithOptions(
	t *testing.T,
	root string,
	scenario string,
	run int,
	ttft []float64,
	tps []float64,
	errs []float64,
	cpuRows []cpuRow,
) {
	t.Helper()
	if len(ttft) != len(tps) || len(ttft) != len(errs) {
		t.Fatalf("ttft/tps/error length mismatch")
	}
	if len(cpuRows) == 0 {
		cpuRows = []cpuRow{{node: "node-a", cpu: 2.0}}
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
			RequestID:        "run-" + strconv.Itoa(run) + "-req-" + strconv.Itoa(i),
			TraceID:          "run-" + strconv.Itoa(run) + "-trace-" + strconv.Itoa(i),
			TTFTMs:           ttft[i],
			RequestLatencyMs: ttft[i] * 2,
			TokenTPS:         tps[i],
			ErrorRate:        errs[i],
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
	for i, row := range cpuRows {
		if err := writer.Write([]string{time.Unix(int64(i+1), 0).UTC().Format(time.RFC3339), row.node, strconv.FormatFloat(row.cpu, 'f', 3, 64), "120", "900", "0"}); err != nil {
			t.Fatalf("write overhead row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
	if err := overheadFile.Close(); err != nil {
		t.Fatalf("close overhead file: %v", err)
	}
}

func writeBaselineManifest(t *testing.T, path string, sourceRef string, sourceCommit string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	manifest := map[string]string{
		"source_ref":    sourceRef,
		"source_commit": sourceCommit,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

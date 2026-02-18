package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/releasegate"
)

func main() {
	candidateRoot := flag.String("candidate-root", filepath.Join("artifacts", "weekly-benchmark"), "candidate benchmark root")
	baselineRoot := flag.String("baseline-root", filepath.Join("artifacts", "weekly-benchmark", "baseline"), "baseline benchmark root")
	scenariosCSV := flag.String("scenarios", "dns_latency,cpu_throttle,provider_throttle,memory_pressure,network_partition,mixed", "comma-separated scenario list")
	maxOverhead := flag.Float64("max-overhead-pct", 3.0, "B5 max collector CPU overhead percent")
	maxVariance := flag.Float64("max-variance-pct", 10.0, "D3 max rerun variance percent")
	minRuns := flag.Int("min-runs", 3, "D3 minimum reruns per scenario")
	regressionLimit := flag.Float64("ttft-regression-pct", 5.0, "E3 max p95 TTFT regression percent")
	alpha := flag.Float64("alpha", 0.05, "E3 significance alpha")
	bootstrapIters := flag.Int("bootstrap-iters", 1000, "E3 bootstrap iterations")
	seed := flag.Int64("seed", 42, "bootstrap RNG seed")
	outJSON := flag.String("out-json", filepath.Join("artifacts", "weekly-benchmark", "m5_gate_summary.json"), "output JSON summary path")
	outMD := flag.String("out-md", filepath.Join("artifacts", "weekly-benchmark", "m5_gate_summary.md"), "output markdown summary path")
	flag.Parse()

	summary, err := releasegate.Evaluate(releasegate.Config{
		CandidateRoot:       *candidateRoot,
		BaselineRoot:        *baselineRoot,
		Scenarios:           splitCSV(*scenariosCSV),
		MaxOverheadPct:      *maxOverhead,
		MaxVariancePct:      *maxVariance,
		MinRunsPerScenario:  *minRuns,
		RegressionPctLimit:  *regressionLimit,
		SignificanceAlpha:   *alpha,
		BootstrapIterations: *bootstrapIters,
		BootstrapSeed:       *seed,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "m5 gate evaluation failed: %v\n", err)
		os.Exit(1)
	}

	if err := writeJSON(*outJSON, summary); err != nil {
		fmt.Fprintf(os.Stderr, "write json summary failed: %v\n", err)
		os.Exit(1)
	}
	if err := writeMarkdown(*outMD, summary); err != nil {
		fmt.Fprintf(os.Stderr, "write markdown summary failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("m5 gate: %s\n", word(summary.Pass))
	fmt.Printf("summary json: %s\n", *outJSON)
	fmt.Printf("summary md: %s\n", *outMD)
	fmt.Printf("B5 overhead max: %.4f%% (limit %.4f%%)\n", summary.Overhead.MaxObservedPct, summary.Overhead.ThresholdPct)
	if !summary.Pass {
		for _, failure := range summary.Failures {
			fmt.Printf("- %s\n", failure)
		}
		os.Exit(1)
	}
}

func writeJSON(path string, payload interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}

func writeMarkdown(path string, summary releasegate.Summary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	body := fmt.Sprintf(
		"# M5 Gate Summary\n\n"+
			"- Overall: `%s`\n"+
			"- Generated at: `%s`\n"+
			"- Candidate root: `%s`\n"+
			"- Baseline root: `%s`\n\n"+
			"## B5 Overhead\n\n"+
			"- Status: `%s`\n"+
			"- Max observed CPU overhead (%%): `%.4f`\n"+
			"- Mean observed CPU overhead (%%): `%.4f`\n"+
			"- Threshold (%%): `%.4f`\n"+
			"- Samples: `%d`\n\n"+
			"## D3 Rerun Variance\n\n"+
			"- Status: `%s`\n"+
			"- Threshold (%%): `%.4f`\n\n",
		word(summary.Pass),
		summary.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
		summary.CandidateRoot,
		summary.BaselineRoot,
		word(summary.Overhead.Pass),
		summary.Overhead.MaxObservedPct,
		summary.Overhead.MeanObservedPct,
		summary.Overhead.ThresholdPct,
		summary.Overhead.SampleCount,
		word(summary.Variance.Pass),
		summary.Variance.ThresholdPct,
	)

	for _, scenario := range summary.Variance.Scenarios {
		body += fmt.Sprintf(
			"- `%s`: status=`%s`, runs=`%d`, variance_pct=`%.4f`, mean_ttft_p95=`%.4f`\n",
			scenario.Scenario,
			word(scenario.Pass),
			scenario.RunCount,
			scenario.VariancePct,
			scenario.MeanTTFTP95,
		)
		if scenario.FailureReason != "" {
			body += fmt.Sprintf("  failure: `%s`\n", scenario.FailureReason)
		}
	}

	body += "\n## E3 Significance\n\n"
	body += fmt.Sprintf("- Status: `%s`\n", word(summary.Significance.Pass))
	body += fmt.Sprintf("- TTFT regression threshold (%%): `%.4f`\n", summary.Significance.RegressionPctLimit)
	body += fmt.Sprintf("- Alpha: `%.4f`\n", summary.Significance.Alpha)
	body += fmt.Sprintf("- Bootstrap iterations: `%d`\n\n", summary.Significance.BootstrapIterations)

	for _, scenario := range summary.Significance.Scenarios {
		body += fmt.Sprintf(
			"- `%s`: status=`%s`, candidate_p95=`%.4f`, baseline_p95=`%.4f`, regression_pct=`%.4f`, p=`%.6f`, ci95_delta=[`%.4f`,`%.4f`]\n",
			scenario.Scenario,
			word(scenario.Pass),
			scenario.CandidateTTFTP95,
			scenario.BaselineTTFTP95,
			scenario.TTFTRegressionPct,
			scenario.MannWhitneyPValue,
			scenario.BootstrapDeltaCI95[0],
			scenario.BootstrapDeltaCI95[1],
		)
		if scenario.FailureReason != "" {
			body += fmt.Sprintf("  failure: `%s`\n", scenario.FailureReason)
		}
	}

	if len(summary.Failures) > 0 {
		body += "\n## Failures\n\n"
		for _, failure := range summary.Failures {
			body += fmt.Sprintf("- %s\n", failure)
		}
	}

	return os.WriteFile(path, []byte(body), 0o644)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func word(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

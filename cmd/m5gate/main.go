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
	baselineManifest := flag.String("baseline-manifest", filepath.Join("artifacts", "weekly-benchmark", "baseline", "manifest.json"), "baseline manifest path")
	candidateRef := flag.String("candidate-ref", firstNonEmpty(os.Getenv("GITHUB_REF_NAME"), os.Getenv("GITHUB_REF"), "local"), "candidate git ref")
	candidateCommit := flag.String("candidate-commit", firstNonEmpty(os.Getenv("GITHUB_SHA"), "local"), "candidate git commit")
	requireBaselineManifest := flag.Bool("require-baseline-manifest", false, "require baseline manifest and source independence check")
	scenariosCSV := flag.String("scenarios", "dns_latency,cpu_throttle,provider_throttle,memory_pressure,network_partition,mixed,mixed_multi", "comma-separated scenario list")
	maxOverhead := flag.Float64("max-overhead-pct", 3.0, "B5 max collector CPU overhead percent")
	maxVariance := flag.Float64("max-variance-pct", 10.0, "D3 max rerun variance percent")
	minRuns := flag.Int("min-runs", 3, "D3 minimum reruns per scenario")
	regressionLimit := flag.Float64("ttft-regression-pct", 5.0, "E3 max p95 TTFT regression percent")
	alpha := flag.Float64("alpha", 0.05, "E3 significance alpha")
	bootstrapIters := flag.Int("bootstrap-iters", 1000, "E3 bootstrap iterations")
	seed := flag.Int64("seed", 42, "bootstrap RNG seed")
	minSamples := flag.Int("min-samples", 30, "E3 minimum samples required per scenario for both candidate and baseline")
	minCliffsDelta := flag.Float64("min-cliffs-delta", 0.147, "E3 minimum absolute Cliff's delta to treat significant regression as practically meaningful")
	outJSON := flag.String("out-json", filepath.Join("artifacts", "weekly-benchmark", "m5_gate_summary.json"), "output JSON summary path")
	outMD := flag.String("out-md", filepath.Join("artifacts", "weekly-benchmark", "m5_gate_summary.md"), "output markdown summary path")
	flag.Parse()

	summary, err := releasegate.Evaluate(releasegate.Config{
		CandidateRoot:            *candidateRoot,
		BaselineRoot:             *baselineRoot,
		BaselineManifestPath:     *baselineManifest,
		CandidateRef:             *candidateRef,
		CandidateCommit:          *candidateCommit,
		RequireBaselineManifest:  *requireBaselineManifest,
		Scenarios:                splitCSV(*scenariosCSV),
		MaxOverheadPct:           *maxOverhead,
		MaxVariancePct:           *maxVariance,
		MinRunsPerScenario:       *minRuns,
		RegressionPctLimit:       *regressionLimit,
		SignificanceAlpha:        *alpha,
		BootstrapIterations:      *bootstrapIters,
		BootstrapSeed:            *seed,
		MinSamplesPerScenario:    *minSamples,
		MinCliffsDeltaForFailure: *minCliffsDelta,
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
	fmt.Printf("B5 overhead node p95 max: %.4f%% on %s (limit %.4f%%)\n", summary.Overhead.MaxNodeP95Pct, summary.Overhead.MaxNodeP95Node, summary.Overhead.ThresholdPct)
	if summary.Baseline.SameSource {
		fmt.Printf("note: %s\n", summary.Baseline.FailureReason)
	}
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
			"## Baseline Provenance\n\n"+
			"- Status: `%s`\n"+
			"- Manifest required: `%t`\n"+
			"- Manifest path: `%s`\n"+
			"- Baseline source ref: `%s`\n"+
			"- Baseline source commit: `%s`\n"+
			"- Candidate ref: `%s`\n"+
			"- Candidate commit: `%s`\n\n"+
			"## B5 Overhead\n\n"+
			"- Status: `%s`\n"+
			"- Max observed CPU overhead (%%): `%.4f`\n"+
			"- Mean observed CPU overhead (%%): `%.4f`\n"+
			"- Max node p95 CPU overhead (%%): `%.4f` (`%s`)\n"+
			"- Threshold (%%): `%.4f`\n"+
			"- Samples: `%d`\n\n"+
			"## D3 Rerun Variance\n\n"+
			"- Status: `%s`\n"+
			"- Threshold (%%): `%.4f`\n\n",
		word(summary.Pass),
		summary.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
		summary.CandidateRoot,
		summary.BaselineRoot,
		word(summary.Baseline.Pass),
		summary.Baseline.ManifestRequired,
		summary.Baseline.ManifestPath,
		nonEmpty(summary.Baseline.SourceRef, "-"),
		nonEmpty(summary.Baseline.SourceCommit, "-"),
		nonEmpty(summary.Baseline.CandidateRef, "-"),
		nonEmpty(summary.Baseline.CandidateCommit, "-"),
		word(summary.Overhead.Pass),
		summary.Overhead.MaxObservedPct,
		summary.Overhead.MeanObservedPct,
		summary.Overhead.MaxNodeP95Pct,
		nonEmpty(summary.Overhead.MaxNodeP95Node, "-"),
		summary.Overhead.ThresholdPct,
		summary.Overhead.SampleCount,
		word(summary.Variance.Pass),
		summary.Variance.ThresholdPct,
	)

	for _, scenario := range summary.Variance.Scenarios {
		body += fmt.Sprintf(
			"- `%s`: status=`%s`, runs=`%d`, ttft_variance_pct=`%.4f`, tokens_variance_pct=`%.4f`, error_rate_variance_pct=`%.4f`, mean_ttft_p95=`%.4f`\n",
			scenario.Scenario,
			word(scenario.Pass),
			scenario.RunCount,
			scenario.VariancePct,
			scenario.TokensVariancePct,
			scenario.ErrorRateVariancePct,
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
	body += fmt.Sprintf("- Minimum samples per scenario: `%d`\n", summary.Significance.MinSamplesPerScenario)
	body += fmt.Sprintf("- Minimum |Cliff's delta| for failure: `%.4f`\n\n", summary.Significance.MinCliffsDeltaForFailure)

	for _, scenario := range summary.Significance.Scenarios {
		body += fmt.Sprintf(
			"- `%s`: status=`%s`, candidate_n=`%d`, baseline_n=`%d`, candidate_p95=`%.4f`, baseline_p95=`%.4f`, regression_pct=`%.4f`, p=`%.6f`, ci95_delta=[`%.4f`,`%.4f`], cliffs_delta=`%.4f`\n",
			scenario.Scenario,
			word(scenario.Pass),
			scenario.CandidateN,
			scenario.BaselineN,
			scenario.CandidateTTFTP95,
			scenario.BaselineTTFTP95,
			scenario.TTFTRegressionPct,
			scenario.MannWhitneyPValue,
			scenario.BootstrapDeltaCI95[0],
			scenario.BootstrapDeltaCI95[1],
			scenario.CliffsDelta,
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

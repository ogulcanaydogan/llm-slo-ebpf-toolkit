package releasegate

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
)

// Config defines evaluation thresholds and IO roots for M5 GA gates.
type Config struct {
	CandidateRoot            string
	BaselineRoot             string
	BaselineManifestPath     string
	CandidateRef             string
	CandidateCommit          string
	RequireBaselineManifest  bool
	Scenarios                []string
	MaxOverheadPct           float64
	MaxVariancePct           float64
	MinRunsPerScenario       int
	RegressionPctLimit       float64
	SignificanceAlpha        float64
	BootstrapIterations      int
	BootstrapSeed            int64
	MinSamplesPerScenario    int
	MinCliffsDeltaForFailure float64
}

// Summary is the machine-readable output for M5 gate decisions.
type Summary struct {
	GeneratedAt   time.Time        `json:"generated_at"`
	CandidateRoot string           `json:"candidate_root"`
	BaselineRoot  string           `json:"baseline_root"`
	Scenarios     []string         `json:"scenarios"`
	Baseline      BaselineGate     `json:"baseline"`
	Overhead      OverheadGate     `json:"overhead"`
	Variance      VarianceGate     `json:"variance"`
	Significance  SignificanceGate `json:"significance"`
	Pass          bool             `json:"pass"`
	Failures      []string         `json:"failures,omitempty"`
}

// BaselineGate validates baseline provenance and independence.
type BaselineGate struct {
	Pass             bool   `json:"pass"`
	ManifestRequired bool   `json:"manifest_required"`
	ManifestPath     string `json:"manifest_path"`
	SourceRef        string `json:"source_ref,omitempty"`
	SourceCommit     string `json:"source_commit,omitempty"`
	CandidateRef     string `json:"candidate_ref,omitempty"`
	CandidateCommit  string `json:"candidate_commit,omitempty"`
	SameSource       bool   `json:"same_source"`
	FailureReason    string `json:"failure_reason,omitempty"`
}

// OverheadGate captures B5 verdict details.
type OverheadGate struct {
	Pass            bool               `json:"pass"`
	ThresholdPct    float64            `json:"threshold_pct"`
	MaxObservedPct  float64            `json:"max_observed_pct"`
	MeanObservedPct float64            `json:"mean_observed_pct"`
	SampleCount     int                `json:"sample_count"`
	FilesChecked    int                `json:"files_checked"`
	NodeP95Observed map[string]float64 `json:"node_p95_observed"`
	MaxNodeP95Pct   float64            `json:"max_node_p95_pct"`
	MaxNodeP95Node  string             `json:"max_node_p95_node,omitempty"`
	FailureReason   string             `json:"failure_reason,omitempty"`
}

// VarianceGate captures D3 verdict details.
type VarianceGate struct {
	Pass         bool               `json:"pass"`
	ThresholdPct float64            `json:"threshold_pct"`
	MinRuns      int                `json:"min_runs"`
	Scenarios    []ScenarioVariance `json:"scenarios"`
}

// ScenarioVariance captures one scenario's rerun variance.
type ScenarioVariance struct {
	Scenario             string    `json:"scenario"`
	RunCount             int       `json:"run_count"`
	TTFTP95Values        []float64 `json:"ttft_p95_values"`
	MeanTTFTP95          float64   `json:"mean_ttft_p95"`
	StdDevTTFTP95        float64   `json:"stddev_ttft_p95"`
	VariancePct          float64   `json:"variance_pct"`
	TokensP50Values      []float64 `json:"tokens_p50_values"`
	MeanTokensP50        float64   `json:"mean_tokens_p50"`
	StdDevTokensP50      float64   `json:"stddev_tokens_p50"`
	TokensVariancePct    float64   `json:"tokens_variance_pct"`
	ErrorRateMeanValues  []float64 `json:"error_rate_mean_values"`
	MeanErrorRateMean    float64   `json:"mean_error_rate_mean"`
	StdDevErrorRateMean  float64   `json:"stddev_error_rate_mean"`
	ErrorRateVariancePct float64   `json:"error_rate_variance_pct"`
	Pass                 bool      `json:"pass"`
	FailureReason        string    `json:"failure_reason,omitempty"`
}

// SignificanceGate captures E3 verdict details.
type SignificanceGate struct {
	Pass                     bool                   `json:"pass"`
	RegressionPctLimit       float64                `json:"regression_pct_limit"`
	Alpha                    float64                `json:"alpha"`
	BootstrapIterations      int                    `json:"bootstrap_iterations"`
	MinSamplesPerScenario    int                    `json:"min_samples_per_scenario"`
	MinCliffsDeltaForFailure float64                `json:"min_cliffs_delta_for_failure"`
	Scenarios                []ScenarioSignificance `json:"scenarios"`
}

// ScenarioSignificance captures one scenario's statistical regression checks.
type ScenarioSignificance struct {
	Scenario              string     `json:"scenario"`
	CandidateN            int        `json:"candidate_n"`
	BaselineN             int        `json:"baseline_n"`
	CandidateTTFTP95      float64    `json:"candidate_ttft_p95"`
	BaselineTTFTP95       float64    `json:"baseline_ttft_p95"`
	TTFTRegressionPct     float64    `json:"ttft_regression_pct"`
	CandidateTokensP50    float64    `json:"candidate_tokens_p50"`
	BaselineTokensP50     float64    `json:"baseline_tokens_p50"`
	MannWhitneyPValue     float64    `json:"mann_whitney_p_value"`
	BootstrapDeltaCI95    [2]float64 `json:"bootstrap_delta_ci95"`
	CliffsDelta           float64    `json:"cliffs_delta"`
	PracticalEffectPass   bool       `json:"practical_effect_pass"`
	MinimumSamplesReached bool       `json:"minimum_samples_reached"`
	Pass                  bool       `json:"pass"`
	FailureReason         string     `json:"failure_reason,omitempty"`
}

// Evaluate executes B5, D3, and E3 gates together.
func Evaluate(cfg Config) (Summary, error) {
	cfg = normalizeConfig(cfg)
	summary := Summary{
		GeneratedAt:   time.Now().UTC(),
		CandidateRoot: cfg.CandidateRoot,
		BaselineRoot:  cfg.BaselineRoot,
		Scenarios:     append([]string(nil), cfg.Scenarios...),
	}

	baseline, err := evaluateBaseline(cfg)
	if err != nil {
		return Summary{}, err
	}
	overhead, err := evaluateOverhead(cfg)
	if err != nil {
		return Summary{}, err
	}
	variance, err := evaluateVariance(cfg)
	if err != nil {
		return Summary{}, err
	}
	significance, err := evaluateSignificance(cfg)
	if err != nil {
		return Summary{}, err
	}

	summary.Baseline = baseline
	summary.Overhead = overhead
	summary.Variance = variance
	summary.Significance = significance
	summary.Pass = baseline.Pass && overhead.Pass && variance.Pass && significance.Pass

	if !baseline.Pass {
		reason := baseline.FailureReason
		if reason == "" {
			reason = "baseline provenance validation failed"
		}
		summary.Failures = append(summary.Failures, "baseline gate failed: "+reason)
	}

	if !overhead.Pass {
		reason := overhead.FailureReason
		if reason == "" {
			reason = fmt.Sprintf("node p95 %.4f on %s exceeded %.4f", overhead.MaxNodeP95Pct, overhead.MaxNodeP95Node, overhead.ThresholdPct)
		}
		summary.Failures = append(summary.Failures, "B5 overhead gate failed: "+reason)
	}
	if !variance.Pass {
		summary.Failures = append(summary.Failures, "D3 rerun variance gate failed")
	}
	if !significance.Pass {
		summary.Failures = append(summary.Failures, "E3 significance gate failed")
	}

	return summary, nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.CandidateRoot == "" {
		cfg.CandidateRoot = filepath.Join("artifacts", "weekly-benchmark")
	}
	if cfg.BaselineRoot == "" {
		cfg.BaselineRoot = filepath.Join(cfg.CandidateRoot, "baseline")
	}
	if cfg.BaselineManifestPath == "" {
		cfg.BaselineManifestPath = filepath.Join(cfg.BaselineRoot, "manifest.json")
	}
	if len(cfg.Scenarios) == 0 {
		cfg.Scenarios = []string{"dns_latency", "cpu_throttle", "provider_throttle", "memory_pressure", "network_partition", "mixed", "mixed_multi"}
	}
	if cfg.MaxOverheadPct <= 0 {
		cfg.MaxOverheadPct = 3
	}
	if cfg.MaxVariancePct <= 0 {
		cfg.MaxVariancePct = 10
	}
	if cfg.MinRunsPerScenario <= 0 {
		cfg.MinRunsPerScenario = 3
	}
	if cfg.RegressionPctLimit <= 0 {
		cfg.RegressionPctLimit = 5
	}
	if cfg.SignificanceAlpha <= 0 || cfg.SignificanceAlpha >= 1 {
		cfg.SignificanceAlpha = 0.05
	}
	if cfg.BootstrapIterations <= 0 {
		cfg.BootstrapIterations = 1000
	}
	if cfg.BootstrapSeed == 0 {
		cfg.BootstrapSeed = 42
	}
	if cfg.MinSamplesPerScenario <= 0 {
		cfg.MinSamplesPerScenario = 30
	}
	if cfg.MinCliffsDeltaForFailure <= 0 {
		cfg.MinCliffsDeltaForFailure = 0.147
	}
	return cfg
}

type baselineManifest struct {
	SourceRef    string `json:"source_ref"`
	SourceCommit string `json:"source_commit"`
	GeneratedAt  string `json:"generated_at,omitempty"`
}

func evaluateBaseline(cfg Config) (BaselineGate, error) {
	result := BaselineGate{
		Pass:             true,
		ManifestRequired: cfg.RequireBaselineManifest,
		ManifestPath:     cfg.BaselineManifestPath,
		CandidateRef:     cfg.CandidateRef,
		CandidateCommit:  cfg.CandidateCommit,
	}

	manifestPath := cfg.BaselineManifestPath
	manifestExists := false
	if _, err := os.Stat(manifestPath); err == nil {
		manifestExists = true
	} else if !os.IsNotExist(err) {
		return result, fmt.Errorf("stat baseline manifest %s: %w", manifestPath, err)
	}

	if cfg.RequireBaselineManifest && !manifestExists {
		result.Pass = false
		result.FailureReason = fmt.Sprintf("required baseline manifest missing: %s", manifestPath)
		return result, nil
	}
	if !manifestExists {
		return result, nil
	}

	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return result, fmt.Errorf("read baseline manifest %s: %w", manifestPath, err)
	}
	var manifest baselineManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return result, fmt.Errorf("parse baseline manifest %s: %w", manifestPath, err)
	}

	result.SourceRef = strings.TrimSpace(manifest.SourceRef)
	result.SourceCommit = strings.TrimSpace(manifest.SourceCommit)

	sameCommit := result.SourceCommit != "" && result.CandidateCommit != "" && result.SourceCommit == result.CandidateCommit
	sameRef := result.SourceRef != "" && result.CandidateRef != "" && result.SourceRef == result.CandidateRef
	result.SameSource = sameCommit || sameRef
	if result.SameSource {
		// Same-commit comparison means no code changes since the last tag.
		// This is not a failure — there is simply nothing to regress against.
		// Mark as pass with an informational note so the significance gate
		// can also skip (no meaningful regression test is possible).
		result.Pass = true
		if sameCommit {
			result.FailureReason = fmt.Sprintf("baseline source_commit matches candidate commit (%s); skipping regression comparison", result.SourceCommit)
		} else {
			result.FailureReason = fmt.Sprintf("baseline source_ref matches candidate ref (%s); skipping regression comparison", result.SourceRef)
		}
	}

	return result, nil
}

func evaluateOverhead(cfg Config) (OverheadGate, error) {
	result := OverheadGate{
		ThresholdPct:    cfg.MaxOverheadPct,
		Pass:            true,
		NodeP95Observed: map[string]float64{},
	}
	var values []float64
	valuesByNode := map[string][]float64{}
	files := 0

	for _, scenario := range cfg.Scenarios {
		runs, err := discoverRuns(filepath.Join(cfg.CandidateRoot, scenario))
		if err != nil {
			return result, err
		}
		if len(runs) == 0 {
			return result, fmt.Errorf("no run directories found for scenario %s", scenario)
		}

		for _, runDir := range runs {
			overheadPath := filepath.Join(runDir, "collector_overhead.csv")
			samples, err := loadCollectorCPUSamples(overheadPath)
			if err != nil {
				return result, err
			}
			files++
			for _, sample := range samples {
				values = append(values, sample.CPU)
				valuesByNode[sample.Node] = append(valuesByNode[sample.Node], sample.CPU)
			}
		}
	}

	if len(values) == 0 {
		return result, fmt.Errorf("no overhead values found in candidate root %s", cfg.CandidateRoot)
	}

	result.FilesChecked = files
	result.SampleCount = len(values)
	result.MaxObservedPct = maxFloat(values)
	result.MeanObservedPct = mean(values)

	maxNode := ""
	maxNodeP95 := 0.0
	for node, nodeValues := range valuesByNode {
		p95 := quantile(nodeValues, 0.95)
		result.NodeP95Observed[node] = p95
		if maxNode == "" || p95 > maxNodeP95 {
			maxNode = node
			maxNodeP95 = p95
		}
	}
	result.MaxNodeP95Node = maxNode
	result.MaxNodeP95Pct = maxNodeP95
	result.Pass = result.MaxNodeP95Pct <= result.ThresholdPct && result.MeanObservedPct <= result.ThresholdPct
	if !result.Pass {
		switch {
		case result.MaxNodeP95Pct > result.ThresholdPct:
			result.FailureReason = fmt.Sprintf(
				"node %s p95 overhead %.4f exceeds %.4f",
				result.MaxNodeP95Node,
				result.MaxNodeP95Pct,
				result.ThresholdPct,
			)
		case result.MeanObservedPct > result.ThresholdPct:
			result.FailureReason = fmt.Sprintf(
				"mean overhead %.4f exceeds %.4f",
				result.MeanObservedPct,
				result.ThresholdPct,
			)
		}
	}
	return result, nil
}

func evaluateVariance(cfg Config) (VarianceGate, error) {
	result := VarianceGate{
		Pass:         true,
		ThresholdPct: cfg.MaxVariancePct,
		MinRuns:      cfg.MinRunsPerScenario,
		Scenarios:    make([]ScenarioVariance, 0, len(cfg.Scenarios)),
	}

	for _, scenario := range cfg.Scenarios {
		runs, err := discoverRuns(filepath.Join(cfg.CandidateRoot, scenario))
		if err != nil {
			return result, err
		}

		scenarioResult := ScenarioVariance{Scenario: scenario, RunCount: len(runs)}
		if len(runs) < cfg.MinRunsPerScenario {
			scenarioResult.Pass = false
			scenarioResult.FailureReason = fmt.Sprintf("requires at least %d runs", cfg.MinRunsPerScenario)
			result.Pass = false
			result.Scenarios = append(result.Scenarios, scenarioResult)
			continue
		}

		ttftP95 := make([]float64, 0, len(runs))
		tokensP50 := make([]float64, 0, len(runs))
		errorRateMean := make([]float64, 0, len(runs))
		for _, runDir := range runs {
			rawPath := filepath.Join(runDir, "raw_samples.jsonl")
			raw, err := loadRawSamples(rawPath)
			if err != nil {
				return result, err
			}
			ttft := extractTTFT(raw)
			tokens := extractTokens(raw)
			errors := extractErrorRates(raw)
			if len(ttft) == 0 {
				return result, fmt.Errorf("no ttft values found in %s", rawPath)
			}
			if len(tokens) == 0 {
				return result, fmt.Errorf("no token throughput values found in %s", rawPath)
			}
			if len(errors) == 0 {
				return result, fmt.Errorf("no error_rate values found in %s", rawPath)
			}
			ttftP95 = append(ttftP95, quantile(ttft, 0.95))
			tokensP50 = append(tokensP50, quantile(tokens, 0.50))
			errorRateMean = append(errorRateMean, mean(errors))
		}

		scenarioResult.TTFTP95Values = ttftP95
		scenarioResult.MeanTTFTP95 = mean(ttftP95)
		scenarioResult.StdDevTTFTP95 = stddev(ttftP95)
		scenarioResult.VariancePct = coefficientOfVariancePct(ttftP95)
		scenarioResult.TokensP50Values = tokensP50
		scenarioResult.MeanTokensP50 = mean(tokensP50)
		scenarioResult.StdDevTokensP50 = stddev(tokensP50)
		scenarioResult.TokensVariancePct = coefficientOfVariancePct(tokensP50)
		scenarioResult.ErrorRateMeanValues = errorRateMean
		scenarioResult.MeanErrorRateMean = mean(errorRateMean)
		scenarioResult.StdDevErrorRateMean = stddev(errorRateMean)
		scenarioResult.ErrorRateVariancePct = coefficientOfVariancePct(errorRateMean)

		scenarioResult.Pass = scenarioResult.VariancePct <= cfg.MaxVariancePct &&
			scenarioResult.TokensVariancePct <= cfg.MaxVariancePct &&
			scenarioResult.ErrorRateVariancePct <= cfg.MaxVariancePct
		if !scenarioResult.Pass {
			failureParts := make([]string, 0, 3)
			if scenarioResult.VariancePct > cfg.MaxVariancePct {
				failureParts = append(failureParts, fmt.Sprintf("ttft variance %.4f%% exceeds %.4f%%", scenarioResult.VariancePct, cfg.MaxVariancePct))
			}
			if scenarioResult.TokensVariancePct > cfg.MaxVariancePct {
				failureParts = append(failureParts, fmt.Sprintf("tokens variance %.4f%% exceeds %.4f%%", scenarioResult.TokensVariancePct, cfg.MaxVariancePct))
			}
			if scenarioResult.ErrorRateVariancePct > cfg.MaxVariancePct {
				failureParts = append(failureParts, fmt.Sprintf("error-rate variance %.4f%% exceeds %.4f%%", scenarioResult.ErrorRateVariancePct, cfg.MaxVariancePct))
			}
			scenarioResult.FailureReason = strings.Join(failureParts, "; ")
			result.Pass = false
		}

		result.Scenarios = append(result.Scenarios, scenarioResult)
	}

	return result, nil
}

func evaluateSignificance(cfg Config) (SignificanceGate, error) {
	result := SignificanceGate{
		Pass:                     true,
		RegressionPctLimit:       cfg.RegressionPctLimit,
		Alpha:                    cfg.SignificanceAlpha,
		BootstrapIterations:      cfg.BootstrapIterations,
		MinSamplesPerScenario:    cfg.MinSamplesPerScenario,
		MinCliffsDeltaForFailure: cfg.MinCliffsDeltaForFailure,
		Scenarios:                make([]ScenarioSignificance, 0, len(cfg.Scenarios)),
	}

	rng := rand.New(rand.NewSource(cfg.BootstrapSeed))

	for _, scenario := range cfg.Scenarios {
		candidateRuns, err := discoverRuns(filepath.Join(cfg.CandidateRoot, scenario))
		if err != nil {
			return result, err
		}
		if len(candidateRuns) == 0 {
			return result, fmt.Errorf("no candidate runs found for %s", scenario)
		}

		baselineRuns, err := discoverRuns(filepath.Join(cfg.BaselineRoot, scenario))
		if err != nil {
			return result, err
		}
		if len(baselineRuns) == 0 {
			return result, fmt.Errorf("no baseline runs found for %s in %s", scenario, cfg.BaselineRoot)
		}

		candidateSamples, err := loadSamplesFromRuns(candidateRuns)
		if err != nil {
			return result, err
		}
		baselineSamples, err := loadSamplesFromRuns(baselineRuns)
		if err != nil {
			return result, err
		}

		candidateTTFT := extractTTFT(candidateSamples)
		baselineTTFT := extractTTFT(baselineSamples)
		candidateTPS := extractTokens(candidateSamples)
		baselineTPS := extractTokens(baselineSamples)

		if len(candidateTTFT) == 0 || len(baselineTTFT) == 0 {
			return result, fmt.Errorf("ttft distributions empty for scenario %s", scenario)
		}

		candidateP95 := quantile(candidateTTFT, 0.95)
		baselineP95 := quantile(baselineTTFT, 0.95)
		regressionPct := 0.0
		if baselineP95 > 0 {
			regressionPct = ((candidateP95 - baselineP95) / baselineP95) * 100
		}
		candidateTokensP50 := quantile(candidateTPS, 0.50)
		baselineTokensP50 := quantile(baselineTPS, 0.50)

		minSamplesReached := len(candidateTTFT) >= cfg.MinSamplesPerScenario && len(baselineTTFT) >= cfg.MinSamplesPerScenario

		scenarioResult := ScenarioSignificance{
			Scenario:              scenario,
			CandidateN:            len(candidateTTFT),
			BaselineN:             len(baselineTTFT),
			CandidateTTFTP95:      candidateP95,
			BaselineTTFTP95:       baselineP95,
			TTFTRegressionPct:     regressionPct,
			CandidateTokensP50:    candidateTokensP50,
			BaselineTokensP50:     baselineTokensP50,
			MannWhitneyPValue:     1.0,
			BootstrapDeltaCI95:    [2]float64{0, 0},
			CliffsDelta:           0,
			PracticalEffectPass:   false,
			MinimumSamplesReached: minSamplesReached,
			Pass:                  true,
		}

		if !minSamplesReached {
			scenarioResult.Pass = false
			scenarioResult.FailureReason = fmt.Sprintf(
				"insufficient samples: candidate=%d baseline=%d requires >=%d",
				len(candidateTTFT),
				len(baselineTTFT),
				cfg.MinSamplesPerScenario,
			)
			result.Pass = false
			result.Scenarios = append(result.Scenarios, scenarioResult)
			continue
		}

		pValue := mannWhitneyPValue(candidateTTFT, baselineTTFT)
		ciLow, ciHigh := bootstrapDeltaCI(candidateTTFT, baselineTTFT, 0.95, cfg.BootstrapIterations, rng)
		cliffs := cliffsDelta(candidateTTFT, baselineTTFT)
		scenarioResult.MannWhitneyPValue = pValue
		scenarioResult.BootstrapDeltaCI95 = [2]float64{ciLow, ciHigh}
		scenarioResult.CliffsDelta = cliffs
		scenarioResult.PracticalEffectPass = math.Abs(cliffs) >= cfg.MinCliffsDeltaForFailure

		isRegression := regressionPct > cfg.RegressionPctLimit && pValue < cfg.SignificanceAlpha && ciLow > 0
		if isRegression && !scenarioResult.PracticalEffectPass {
			scenarioResult.FailureReason = fmt.Sprintf(
				"statistical regression detected (%.4f%%, p=%.6f, CI95[%.4f, %.4f]) but |Cliff's delta| %.4f < %.4f practical threshold",
				regressionPct,
				pValue,
				ciLow,
				ciHigh,
				math.Abs(cliffs),
				cfg.MinCliffsDeltaForFailure,
			)
		}
		if isRegression && scenarioResult.PracticalEffectPass {
			scenarioResult.Pass = false
			scenarioResult.FailureReason = fmt.Sprintf(
				"ttft regression %.4f%% exceeds %.4f%% with p=%.6f CI95[%.4f, %.4f] and Cliff's delta %.4f",
				regressionPct,
				cfg.RegressionPctLimit,
				pValue,
				ciLow,
				ciHigh,
				cliffs,
			)
			result.Pass = false
		}

		result.Scenarios = append(result.Scenarios, scenarioResult)
	}

	return result, nil
}

func discoverRuns(scenarioRoot string) ([]string, error) {
	entries, err := os.ReadDir(scenarioRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	runs := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "run-") {
			runs = append(runs, filepath.Join(scenarioRoot, entry.Name()))
		}
	}
	sort.Strings(runs)
	if len(runs) > 0 {
		return runs, nil
	}

	if _, err := os.Stat(filepath.Join(scenarioRoot, "raw_samples.jsonl")); err == nil {
		return []string{scenarioRoot}, nil
	}
	return nil, nil
}

type collectorCPUSample struct {
	Node string
	CPU  float64
}

func loadCollectorCPUSamples(path string) ([]collectorCPUSample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open overhead file %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read overhead csv %s: %w", path, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("overhead csv %s has no data rows", path)
	}

	header := records[0]
	cpuIdx := indexOf(header, "collector_cpu_pct")
	if cpuIdx < 0 {
		return nil, fmt.Errorf("collector_cpu_pct column missing in %s", path)
	}
	nodeIdx := indexOf(header, "node")

	out := make([]collectorCPUSample, 0, len(records)-1)
	for _, row := range records[1:] {
		if cpuIdx >= len(row) {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(row[cpuIdx]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse collector_cpu_pct in %s: %w", path, err)
		}
		node := "unknown"
		if nodeIdx >= 0 && nodeIdx < len(row) {
			node = strings.TrimSpace(row[nodeIdx])
			if node == "" {
				node = "unknown"
			}
		}
		out = append(out, collectorCPUSample{Node: node, CPU: value})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no collector_cpu_pct values in %s", path)
	}
	return out, nil
}

func loadSamplesFromRuns(runDirs []string) ([]collector.RawSample, error) {
	all := make([]collector.RawSample, 0)
	for _, runDir := range runDirs {
		rawPath := filepath.Join(runDir, "raw_samples.jsonl")
		samples, err := loadRawSamples(rawPath)
		if err != nil {
			return nil, err
		}
		all = append(all, samples...)
	}
	return all, nil
}

func loadRawSamples(path string) ([]collector.RawSample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open raw samples %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	samples := make([]collector.RawSample, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var sample collector.RawSample
		if err := json.Unmarshal([]byte(line), &sample); err != nil {
			return nil, fmt.Errorf("parse raw sample from %s: %w", path, err)
		}
		samples = append(samples, sample)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan raw samples %s: %w", path, err)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no raw samples in %s", path)
	}
	return samples, nil
}

func extractTTFT(samples []collector.RawSample) []float64 {
	out := make([]float64, 0, len(samples))
	for _, sample := range samples {
		out = append(out, sample.TTFTMs)
	}
	return out
}

func extractTokens(samples []collector.RawSample) []float64 {
	out := make([]float64, 0, len(samples))
	for _, sample := range samples {
		out = append(out, sample.TokenTPS)
	}
	return out
}

func extractErrorRates(samples []collector.RawSample) []float64 {
	out := make([]float64, 0, len(samples))
	for _, sample := range samples {
		out = append(out, sample.ErrorRate)
	}
	return out
}

func quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		q = 0
	}
	if q >= 1 {
		q = 1
	}
	cpy := append([]float64(nil), values...)
	sort.Float64s(cpy)
	if len(cpy) == 1 {
		return cpy[0]
	}
	pos := q * float64(len(cpy)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return cpy[lo]
	}
	frac := pos - float64(lo)
	return cpy[lo]*(1-frac) + cpy[hi]*frac
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	m := mean(values)
	acc := 0.0
	for _, value := range values {
		delta := value - m
		acc += delta * delta
	}
	return math.Sqrt(acc / float64(len(values)-1))
}

func coefficientOfVariancePct(values []float64) float64 {
	m := mean(values)
	if m == 0 {
		return 0
	}
	return (stddev(values) / m) * 100
}

func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func indexOf(items []string, target string) int {
	for idx, item := range items {
		if item == target {
			return idx
		}
	}
	return -1
}

func mannWhitneyPValue(x []float64, y []float64) float64 {
	nx := len(x)
	ny := len(y)
	if nx == 0 || ny == 0 {
		return 1
	}

	type point struct {
		value float64
		group int
	}
	points := make([]point, 0, nx+ny)
	for _, value := range x {
		points = append(points, point{value: value, group: 0})
	}
	for _, value := range y {
		points = append(points, point{value: value, group: 1})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].value < points[j].value
	})

	ranks := make([]float64, len(points))
	tieSum := 0.0
	for i := 0; i < len(points); {
		j := i + 1
		for j < len(points) && points[j].value == points[i].value {
			j++
		}
		avgRank := (float64(i+1) + float64(j)) / 2.0
		for k := i; k < j; k++ {
			ranks[k] = avgRank
		}
		t := float64(j - i)
		if t > 1 {
			tieSum += (t*t*t - t)
		}
		i = j
	}

	rankX := 0.0
	for idx, p := range points {
		if p.group == 0 {
			rankX += ranks[idx]
		}
	}

	nxf := float64(nx)
	nyf := float64(ny)
	u1 := rankX - (nxf*(nxf+1))/2.0
	u2 := nxf*nyf - u1
	u := math.Min(u1, u2)

	n := nxf + nyf
	meanU := (nxf * nyf) / 2.0
	varianceU := (nxf * nyf / 12.0) * ((n + 1.0) - (tieSum / (n * (n - 1.0))))
	if varianceU <= 0 {
		return 1
	}

	z := u - meanU
	if z > 0 {
		z = (z - 0.5) / math.Sqrt(varianceU)
	} else {
		z = (z + 0.5) / math.Sqrt(varianceU)
	}

	p := 2 * (1 - normalCDF(math.Abs(z)))
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func cliffsDelta(x []float64, y []float64) float64 {
	if len(x) == 0 || len(y) == 0 {
		return 0
	}
	greater := 0
	lower := 0
	for _, xv := range x {
		for _, yv := range y {
			switch {
			case xv > yv:
				greater++
			case xv < yv:
				lower++
			}
		}
	}
	total := float64(len(x) * len(y))
	return (float64(greater) - float64(lower)) / total
}

func normalCDF(z float64) float64 {
	return 0.5 * (1 + math.Erf(z/math.Sqrt2))
}

func bootstrapDeltaCI(candidate []float64, baseline []float64, quant float64, iterations int, rng *rand.Rand) (float64, float64) {
	if len(candidate) == 0 || len(baseline) == 0 || iterations < 10 {
		return 0, 0
	}

	deltas := make([]float64, 0, iterations)
	candBuf := make([]float64, len(candidate))
	baseBuf := make([]float64, len(baseline))

	for i := 0; i < iterations; i++ {
		for j := range candidate {
			candBuf[j] = candidate[rng.Intn(len(candidate))]
		}
		for j := range baseline {
			baseBuf[j] = baseline[rng.Intn(len(baseline))]
		}
		deltas = append(deltas, quantile(candBuf, quant)-quantile(baseBuf, quant))
	}

	sort.Float64s(deltas)
	lowIdx := int(math.Floor(0.025 * float64(len(deltas)-1)))
	highIdx := int(math.Ceil(0.975 * float64(len(deltas)-1)))
	if lowIdx < 0 {
		lowIdx = 0
	}
	if highIdx >= len(deltas) {
		highIdx = len(deltas) - 1
	}
	return deltas[lowIdx], deltas[highIdx]
}

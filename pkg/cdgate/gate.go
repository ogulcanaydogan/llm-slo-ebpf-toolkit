package cdgate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Metric names queried from Prometheus.
const (
	MetricTTFTp95   = "ttft_p95_ms"
	MetricErrorRate = "error_rate"
	MetricBurnRate  = "burn_rate"
)

// Thresholds defines the SLO gate pass criteria.
type Thresholds struct {
	TTFTp95MS float64
	ErrorRate float64
	BurnRate  float64
}

// Violation describes a single SLO metric breach.
type Violation struct {
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
}

// Result is the output of an SLO gate evaluation.
type Result struct {
	Pass       bool        `json:"pass"`
	Violations []Violation `json:"violations"`
	Timestamp  time.Time   `json:"timestamp"`
	Error      string      `json:"error,omitempty"`
}

// PrometheusQuerier abstracts Prometheus instant query API.
type PrometheusQuerier interface {
	Query(ctx context.Context, query string) (float64, error)
}

// HTTPQuerier queries Prometheus via its HTTP API.
type HTTPQuerier struct {
	BaseURL string
	Client  *http.Client
}

// promResponse is the Prometheus instant query JSON envelope.
type promResponse struct {
	Status string   `json:"status"`
	Data   promData `json:"data"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promResult `json:"result"`
}

type promResult struct {
	Value [2]interface{} `json:"value"` // [timestamp, "value_string"]
}

// Query executes a Prometheus instant query and returns the scalar result.
func (q *HTTPQuerier) Query(ctx context.Context, query string) (float64, error) {
	u, err := url.Parse(q.BaseURL)
	if err != nil {
		return 0, fmt.Errorf("parse prometheus url: %w", err)
	}
	u.Path = "/api/v1/query"
	u.RawQuery = url.Values{"query": {query}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	client := q.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, fmt.Errorf("unmarshal prometheus response: %w", err)
	}

	if pr.Status != "success" {
		return 0, fmt.Errorf("prometheus query status: %s", pr.Status)
	}

	if len(pr.Data.Result) == 0 {
		return 0, fmt.Errorf("prometheus query returned no results for: %s", query)
	}

	valueStr, ok := pr.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value type in prometheus result")
	}

	val, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse prometheus value %q: %w", valueStr, err)
	}

	return val, nil
}

// DefaultQueries returns the Prometheus PromQL queries for each SLO metric.
func DefaultQueries() map[string]string {
	return map[string]string{
		MetricTTFTp95:   `histogram_quantile(0.95, sum(rate(llm_slo_ttft_ms_bucket[5m])) by (le))`,
		MetricErrorRate: `sum(rate(llm_slo_errors_total[5m])) / sum(rate(llm_slo_requests_total[5m]))`,
		MetricBurnRate:  `llm_slo_burn_rate`,
	}
}

// EvaluateSLOGate queries Prometheus for SLO metrics and evaluates thresholds.
func EvaluateSLOGate(ctx context.Context, querier PrometheusQuerier, thresholds Thresholds) Result {
	result := Result{
		Pass:      true,
		Timestamp: time.Now().UTC(),
	}

	queries := DefaultQueries()
	checks := []struct {
		metric    string
		threshold float64
		exceeds   func(actual, threshold float64) bool
	}{
		{MetricTTFTp95, thresholds.TTFTp95MS, func(a, t float64) bool { return a > t }},
		{MetricErrorRate, thresholds.ErrorRate, func(a, t float64) bool { return a > t }},
		{MetricBurnRate, thresholds.BurnRate, func(a, t float64) bool { return a > t }},
	}

	for _, check := range checks {
		val, err := querier.Query(ctx, queries[check.metric])
		if err != nil {
			result.Error = fmt.Sprintf("query %s failed: %v", check.metric, err)
			result.Pass = false
			return result
		}

		if check.exceeds(val, check.threshold) {
			result.Pass = false
			result.Violations = append(result.Violations, Violation{
				Metric:    check.metric,
				Threshold: check.threshold,
				Actual:    val,
			})
		}
	}

	return result
}

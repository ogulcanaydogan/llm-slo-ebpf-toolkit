package cdgate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockQuerier implements PrometheusQuerier for testing.
type mockQuerier struct {
	values map[string]float64
	err    error
}

func (m *mockQuerier) Query(_ context.Context, query string) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	val, ok := m.values[query]
	if !ok {
		return 0, fmt.Errorf("no mock value for query: %s", query)
	}
	return val, nil
}

func TestEvaluateSLOGateAllPass(t *testing.T) {
	queries := DefaultQueries()
	q := &mockQuerier{values: map[string]float64{
		queries[MetricTTFTp95]:   500,
		queries[MetricErrorRate]: 0.01,
		queries[MetricBurnRate]:  1.0,
	}}

	result := EvaluateSLOGate(context.Background(), q, Thresholds{
		TTFTp95MS: 800,
		ErrorRate: 0.05,
		BurnRate:  2.0,
	})

	if !result.Pass {
		t.Fatalf("expected pass, got fail: %+v", result)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(result.Violations))
	}
}

func TestEvaluateSLOGateTTFTViolation(t *testing.T) {
	queries := DefaultQueries()
	q := &mockQuerier{values: map[string]float64{
		queries[MetricTTFTp95]:   900,
		queries[MetricErrorRate]: 0.02,
		queries[MetricBurnRate]:  1.5,
	}}

	result := EvaluateSLOGate(context.Background(), q, Thresholds{
		TTFTp95MS: 800,
		ErrorRate: 0.05,
		BurnRate:  2.0,
	})

	if result.Pass {
		t.Fatal("expected fail due to TTFT violation")
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].Metric != MetricTTFTp95 {
		t.Errorf("expected metric %s, got %s", MetricTTFTp95, result.Violations[0].Metric)
	}
}

func TestEvaluateSLOGateMultipleViolations(t *testing.T) {
	queries := DefaultQueries()
	q := &mockQuerier{values: map[string]float64{
		queries[MetricTTFTp95]:   900,
		queries[MetricErrorRate]: 0.1,
		queries[MetricBurnRate]:  3.0,
	}}

	result := EvaluateSLOGate(context.Background(), q, Thresholds{
		TTFTp95MS: 800,
		ErrorRate: 0.05,
		BurnRate:  2.0,
	})

	if result.Pass {
		t.Fatal("expected fail")
	}
	if len(result.Violations) != 3 {
		t.Fatalf("expected 3 violations, got %d", len(result.Violations))
	}
}

func TestEvaluateSLOGateQueryError(t *testing.T) {
	q := &mockQuerier{err: fmt.Errorf("connection refused")}

	result := EvaluateSLOGate(context.Background(), q, Thresholds{
		TTFTp95MS: 800,
		ErrorRate: 0.05,
		BurnRate:  2.0,
	})

	if result.Pass {
		t.Fatal("expected fail on query error")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestHTTPQuerierSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := promResponse{
			Status: "success",
			Data: promData{
				ResultType: "vector",
				Result: []promResult{
					{Value: [2]interface{}{1.0, "42.5"}},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	q := &HTTPQuerier{BaseURL: server.URL, Client: server.Client()}
	val, err := q.Query(context.Background(), "test_query")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if val != 42.5 {
		t.Errorf("expected 42.5, got %f", val)
	}
}

func TestHTTPQuerierEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := promResponse{
			Status: "success",
			Data: promData{
				ResultType: "vector",
				Result:     []promResult{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	q := &HTTPQuerier{BaseURL: server.URL, Client: server.Client()}
	_, err := q.Query(context.Background(), "test_query")
	if err == nil {
		t.Fatal("expected error for empty result")
	}
}

func TestHTTPQuerierServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	q := &HTTPQuerier{BaseURL: server.URL, Client: server.Client()}
	_, err := q.Query(context.Background(), "test_query")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestDefaultQueriesHaveAllMetrics(t *testing.T) {
	queries := DefaultQueries()
	for _, metric := range []string{MetricTTFTp95, MetricErrorRate, MetricBurnRate} {
		if _, ok := queries[metric]; !ok {
			t.Errorf("missing query for metric %s", metric)
		}
	}
}

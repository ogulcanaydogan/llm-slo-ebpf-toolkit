package slo

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Timing captures one request generation timeline.
type Timing struct {
	RequestStart time.Time
	FirstTokenAt time.Time
	LastTokenAt  time.Time
	TokenCount   int
}

// RetrievalBreakdown captures retrieval latency components.
type RetrievalBreakdown struct {
	VectorDBMS float64
	NetworkMS  float64
	DNSMS      float64
}

// Snapshot is one request-level SLO observation.
type Snapshot struct {
	TTFTMs     float64
	TokensPerS float64
	Retrieval  RetrievalBreakdown
}

// Percentiles represents distribution summary over SLO snapshots.
type Percentiles struct {
	TTFTP50        float64
	TTFTP95        float64
	TTFTP99        float64
	TokensPerSP50  float64
	TokensPerSP95  float64
	RetrievalP95MS float64
}

// Calculate returns one request-level SLO snapshot.
func Calculate(t Timing, retrieval RetrievalBreakdown) (Snapshot, error) {
	ttft, err := TTFTMs(t.RequestStart, t.FirstTokenAt)
	if err != nil {
		return Snapshot{}, err
	}
	tps, err := TokensPerSecond(t.FirstTokenAt, t.LastTokenAt, t.TokenCount)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		TTFTMs:     ttft,
		TokensPerS: tps,
		Retrieval:  retrieval,
	}, nil
}

// TTFTMs returns time-to-first-token in milliseconds.
func TTFTMs(requestStart, firstTokenAt time.Time) (float64, error) {
	if requestStart.IsZero() || firstTokenAt.IsZero() {
		return 0, fmt.Errorf("requestStart and firstTokenAt are required")
	}
	if firstTokenAt.Before(requestStart) {
		return 0, fmt.Errorf("firstTokenAt must be after requestStart")
	}
	return float64(firstTokenAt.Sub(requestStart).Milliseconds()), nil
}

// TokensPerSecond returns generation throughput from first to last token.
func TokensPerSecond(firstTokenAt, lastTokenAt time.Time, tokenCount int) (float64, error) {
	if firstTokenAt.IsZero() || lastTokenAt.IsZero() {
		return 0, fmt.Errorf("firstTokenAt and lastTokenAt are required")
	}
	if tokenCount < 1 {
		return 0, fmt.Errorf("tokenCount must be >= 1")
	}
	if lastTokenAt.Before(firstTokenAt) {
		return 0, fmt.Errorf("lastTokenAt must be after firstTokenAt")
	}

	windowSeconds := lastTokenAt.Sub(firstTokenAt).Seconds()
	if windowSeconds == 0 {
		return float64(tokenCount), nil
	}
	return float64(tokenCount) / windowSeconds, nil
}

// TotalRetrievalMS returns summed retrieval latency components.
func TotalRetrievalMS(b RetrievalBreakdown) float64 {
	return nonNegative(b.VectorDBMS) + nonNegative(b.NetworkMS) + nonNegative(b.DNSMS)
}

// Aggregate computes percentile summaries over snapshots.
func Aggregate(items []Snapshot) Percentiles {
	if len(items) == 0 {
		return Percentiles{}
	}

	ttft := make([]float64, 0, len(items))
	tps := make([]float64, 0, len(items))
	retrieval := make([]float64, 0, len(items))

	for _, item := range items {
		ttft = append(ttft, nonNegative(item.TTFTMs))
		tps = append(tps, nonNegative(item.TokensPerS))
		retrieval = append(retrieval, TotalRetrievalMS(item.Retrieval))
	}

	return Percentiles{
		TTFTP50:        quantile(ttft, 0.50),
		TTFTP95:        quantile(ttft, 0.95),
		TTFTP99:        quantile(ttft, 0.99),
		TokensPerSP50:  quantile(tps, 0.50),
		TokensPerSP95:  quantile(tps, 0.95),
		RetrievalP95MS: quantile(retrieval, 0.95),
	}
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

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	if len(sorted) == 1 {
		return sorted[0]
	}

	pos := q * float64(len(sorted)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return sorted[lower]
	}

	frac := pos - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func nonNegative(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

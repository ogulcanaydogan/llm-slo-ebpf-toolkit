package slo

import (
	"math"
	"testing"
	"time"
)

func TestTTFTMs(t *testing.T) {
	start := time.Unix(0, 0).UTC()
	first := start.Add(175 * time.Millisecond)

	val, err := TTFTMs(start, first)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 175 {
		t.Fatalf("expected 175, got %v", val)
	}
}

func TestTokensPerSecond(t *testing.T) {
	first := time.Unix(0, 0).UTC()
	last := first.Add(2 * time.Second)

	val, err := TokensPerSecond(first, last, 40)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(val-20) > 0.001 {
		t.Fatalf("expected 20 tps, got %f", val)
	}
}

func TestAggregate(t *testing.T) {
	items := []Snapshot{
		{TTFTMs: 100, TokensPerS: 50, Retrieval: RetrievalBreakdown{VectorDBMS: 20, NetworkMS: 10, DNSMS: 5}},
		{TTFTMs: 200, TokensPerS: 30, Retrieval: RetrievalBreakdown{VectorDBMS: 40, NetworkMS: 15, DNSMS: 10}},
		{TTFTMs: 300, TokensPerS: 10, Retrieval: RetrievalBreakdown{VectorDBMS: 60, NetworkMS: 25, DNSMS: 15}},
	}

	out := Aggregate(items)
	if out.TTFTP50 != 200 {
		t.Fatalf("expected ttft p50=200, got %f", out.TTFTP50)
	}
	if out.TokensPerSP50 != 30 {
		t.Fatalf("expected tps p50=30, got %f", out.TokensPerSP50)
	}
	if out.RetrievalP95MS <= 0 {
		t.Fatalf("expected retrieval p95 > 0")
	}
}

func TestCalculateValidation(t *testing.T) {
	_, err := Calculate(Timing{}, RetrievalBreakdown{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

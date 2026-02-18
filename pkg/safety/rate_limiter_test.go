package safety

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	limiter := NewRateLimiter(2)
	base := time.Unix(100, 0).UTC()

	if !limiter.Allow(base) {
		t.Fatal("first event should pass")
	}
	if !limiter.Allow(base.Add(100 * time.Millisecond)) {
		t.Fatal("second event should pass")
	}
	if limiter.Allow(base.Add(200 * time.Millisecond)) {
		t.Fatal("third event in same second should be blocked")
	}
	if !limiter.Allow(base.Add(1200 * time.Millisecond)) {
		t.Fatal("window should reset on next second")
	}
}

type fakeSampler struct {
	idx     int
	samples []CPUSample
}

func (s *fakeSampler) Sample() (CPUSample, error) {
	if s.idx >= len(s.samples) {
		return s.samples[len(s.samples)-1], nil
	}
	out := s.samples[s.idx]
	s.idx++
	return out, nil
}

func TestOverheadGuardEvaluate(t *testing.T) {
	sampler := &fakeSampler{
		samples: []CPUSample{
			{ProcessTicks: 100, TotalTicks: 10_000},
			{ProcessTicks: 220, TotalTicks: 10_800},
		},
	}
	guard := NewOverheadGuardWithSampler(5, sampler)

	if pct, triggered, err := guard.Evaluate(); err != nil || triggered || pct != 0 {
		t.Fatalf("expected bootstrap sample only, pct=%f triggered=%v err=%v", pct, triggered, err)
	}

	pct, triggered, err := guard.Evaluate()
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if pct <= 0 {
		t.Fatalf("expected positive pct, got %f", pct)
	}
	if !triggered {
		t.Fatalf("expected trigger when pct=%f > threshold", pct)
	}
}

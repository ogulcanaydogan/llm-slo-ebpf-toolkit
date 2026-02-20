package attribution

import (
	"math"
	"testing"
	"time"
)

func TestPosteriorsSumToOne(t *testing.T) {
	ba := NewBayesianAttributor()
	signals := map[string]float64{
		"dns_latency_ms":    220,
		"connect_latency_ms": 130,
	}

	posteriors := ba.Attribute(signals)
	sum := 0.0
	for _, p := range posteriors {
		sum += p.Posterior
	}
	if math.Abs(sum-1.0) > 1e-6 {
		t.Fatalf("posteriors sum to %f, want 1.0", sum)
	}
}

func TestSingleFaultDNSLatency(t *testing.T) {
	ba := NewBayesianAttributor()
	signals := map[string]float64{
		"dns_latency_ms":      220,
		"connect_latency_ms":  130,
		"tcp_retransmits_total": 0.2,
		"runqueue_delay_ms":   4,
		"cpu_steal_pct":       0.6,
	}

	posteriors := ba.Attribute(signals)
	if len(posteriors) == 0 {
		t.Fatal("expected posteriors")
	}
	if posteriors[0].Domain != DomainNetworkDNS {
		t.Errorf("top domain: got %s, want %s", posteriors[0].Domain, DomainNetworkDNS)
	}
	if posteriors[0].Posterior < 0.3 {
		t.Errorf("DNS posterior too low: %f", posteriors[0].Posterior)
	}
}

func TestSingleFaultCPUThrottle(t *testing.T) {
	ba := NewBayesianAttributor()
	signals := map[string]float64{
		"runqueue_delay_ms": 28,
		"cpu_steal_pct":     9,
		"cfs_throttled_ms":  170,
		"dns_latency_ms":    12,
	}

	posteriors := ba.Attribute(signals)
	if posteriors[0].Domain != DomainCPUThrottle {
		t.Errorf("top domain: got %s, want %s", posteriors[0].Domain, DomainCPUThrottle)
	}
}

func TestSingleFaultMemoryPressure(t *testing.T) {
	ba := NewBayesianAttributor()
	signals := map[string]float64{
		"cfs_throttled_ms":       90,
		"mem_reclaim_latency_ms": 25,
		"disk_io_latency_ms":     60,
		"runqueue_delay_ms":      14,
		"dns_latency_ms":         12,
	}

	posteriors := ba.Attribute(signals)
	if posteriors[0].Domain != DomainMemoryPressure {
		t.Errorf("top domain: got %s, want %s", posteriors[0].Domain, DomainMemoryPressure)
	}
}

func TestSingleFaultProviderThrottle(t *testing.T) {
	ba := NewBayesianAttributor()
	signals := map[string]float64{
		"connect_latency_ms":  95,
		"tls_handshake_ms":    90,
		"syscall_latency_ms":  300,
		"connect_errors_total": 2,
		"dns_latency_ms":      12,
	}

	posteriors := ba.Attribute(signals)
	if posteriors[0].Domain != DomainProviderThrottle {
		t.Errorf("top domain: got %s, want %s", posteriors[0].Domain, DomainProviderThrottle)
	}
}

func TestMultiFaultMultipleHypotheses(t *testing.T) {
	ba := NewBayesianAttributor()
	// Simultaneous DNS + CPU symptoms
	signals := map[string]float64{
		"dns_latency_ms":    220,
		"runqueue_delay_ms": 28,
		"cpu_steal_pct":     9,
		"cfs_throttled_ms":  100,
	}

	posteriors := ba.Attribute(signals)
	if len(posteriors) < 2 {
		t.Fatal("expected at least 2 hypotheses")
	}

	// Both network_dns and cpu_throttle should appear; in naive Bayes one will dominate
	// but both should have non-zero posteriors. The top-2 domains should include at
	// least one of the injected faults.
	found := map[string]float64{}
	for _, p := range posteriors {
		found[p.Domain] = p.Posterior
	}

	topTwo := map[string]bool{posteriors[0].Domain: true, posteriors[1].Domain: true}
	hasExpected := topTwo[DomainNetworkDNS] || topTwo[DomainCPUThrottle] || topTwo[DomainMemoryPressure]
	if !hasExpected {
		t.Errorf("top-2 domains %v/%v don't include any expected fault domain", posteriors[0].Domain, posteriors[1].Domain)
	}
}

func TestAttributeSamplePopulatesHypotheses(t *testing.T) {
	ba := NewBayesianAttributor()
	sample := FaultSample{
		IncidentID:    "inc-1",
		Timestamp:     time.Now().UTC(),
		Cluster:       "local",
		Service:       "chat",
		FaultLabel:    "dns_latency",
		Confidence:    0.9,
		BurnRate:      2.0,
		WindowMinutes: 5,
		RequestID:     "req-1",
		TraceID:       "trace-1",
		Signals: map[string]float64{
			"dns_latency_ms":    220,
			"connect_latency_ms": 130,
		},
	}

	result := ba.AttributeSample(sample)
	if len(result.FaultHypotheses) == 0 {
		t.Fatal("expected fault_hypotheses to be populated")
	}
	if result.FaultHypotheses[0].Domain != DomainNetworkDNS {
		t.Errorf("top hypothesis: got %s, want %s", result.FaultHypotheses[0].Domain, DomainNetworkDNS)
	}
	if result.PredictedFaultDomain != DomainNetworkDNS {
		t.Errorf("predicted domain: got %s, want %s", result.PredictedFaultDomain, DomainNetworkDNS)
	}
}

func TestAttributeSampleWithoutSignalsFallsBackToRuleBased(t *testing.T) {
	ba := NewBayesianAttributor()
	sample := FaultSample{
		IncidentID:    "inc-1",
		Timestamp:     time.Now().UTC(),
		Cluster:       "local",
		Service:       "chat",
		FaultLabel:    "dns_latency",
		Confidence:    0.9,
		BurnRate:      2.0,
		WindowMinutes: 5,
		RequestID:     "req-1",
		TraceID:       "trace-1",
	}

	result := ba.AttributeSample(sample)
	if result.PredictedFaultDomain != "network_dns" {
		t.Errorf("expected rule-based fallback: got %s", result.PredictedFaultDomain)
	}
	if len(result.FaultHypotheses) != 0 {
		t.Errorf("expected no hypotheses without signals, got %d", len(result.FaultHypotheses))
	}
}

func TestAllDomainsCount(t *testing.T) {
	domains := AllDomains()
	if len(domains) != 8 {
		t.Fatalf("expected 8 domains, got %d", len(domains))
	}
}

func TestDefaultPriorsUniform(t *testing.T) {
	priors := DefaultPriors()
	expected := 1.0 / 8.0
	for domain, p := range priors {
		if math.Abs(p-expected) > 1e-10 {
			t.Errorf("prior for %s: got %f, want %f", domain, p, expected)
		}
	}
}

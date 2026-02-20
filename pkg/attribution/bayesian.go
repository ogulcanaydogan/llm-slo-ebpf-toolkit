package attribution

import (
	"math"
	"sort"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// FaultDomain enumerates the recognized fault domains for Bayesian attribution.
const (
	DomainNetworkDNS       = "network_dns"
	DomainNetworkEgress    = "network_egress"
	DomainCPUThrottle      = "cpu_throttle"
	DomainMemoryPressure   = "memory_pressure"
	DomainProviderThrottle = "provider_throttle"
	DomainProviderError    = "provider_error"
	DomainRetrievalBackend = "retrieval_backend"
	DomainUnknown          = "unknown"
)

// AllDomains returns the full set of fault domains used by the Bayesian engine.
func AllDomains() []string {
	return []string{
		DomainNetworkDNS,
		DomainNetworkEgress,
		DomainCPUThrottle,
		DomainMemoryPressure,
		DomainProviderThrottle,
		DomainProviderError,
		DomainRetrievalBackend,
		DomainUnknown,
	}
}

// BayesianAttributor computes posterior probabilities over fault domains
// given observed signal values. Uses naive Bayes: P(fault|signals) proportional
// to P(fault) * product(P(signal_i|fault)).
type BayesianAttributor struct {
	Priors      map[string]float64
	Likelihoods map[string]map[string]float64 // signal -> domain -> P(signal_elevated|domain)
}

// NewBayesianAttributor returns an attributor with default uniform priors
// and likelihoods derived from the signal generator fault profiles.
func NewBayesianAttributor() *BayesianAttributor {
	return &BayesianAttributor{
		Priors:      DefaultPriors(),
		Likelihoods: DefaultLikelihoods(),
	}
}

// DefaultPriors returns uniform priors across all domains.
func DefaultPriors() map[string]float64 {
	domains := AllDomains()
	priors := make(map[string]float64, len(domains))
	p := 1.0 / float64(len(domains))
	for _, d := range domains {
		priors[d] = p
	}
	return priors
}

// DefaultLikelihoods returns P(signal_elevated|fault_domain) derived from
// the generator.go profileForFault thresholds. High values mean the signal
// is very likely elevated under that fault domain.
func DefaultLikelihoods() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"dns_latency_ms": {
			DomainNetworkDNS:       0.95,
			DomainNetworkEgress:    0.70,
			DomainCPUThrottle:      0.10,
			DomainMemoryPressure:   0.10,
			DomainProviderThrottle: 0.10,
			DomainProviderError:    0.10,
			DomainRetrievalBackend: 0.15,
			DomainUnknown:          0.10,
		},
		"tcp_retransmits_total": {
			DomainNetworkDNS:       0.15,
			DomainNetworkEgress:    0.90,
			DomainCPUThrottle:      0.10,
			DomainMemoryPressure:   0.10,
			DomainProviderThrottle: 0.10,
			DomainProviderError:    0.15,
			DomainRetrievalBackend: 0.10,
			DomainUnknown:          0.10,
		},
		"runqueue_delay_ms": {
			DomainNetworkDNS:       0.10,
			DomainNetworkEgress:    0.10,
			DomainCPUThrottle:      0.90,
			DomainMemoryPressure:   0.60,
			DomainProviderThrottle: 0.10,
			DomainProviderError:    0.10,
			DomainRetrievalBackend: 0.10,
			DomainUnknown:          0.10,
		},
		"connect_latency_ms": {
			DomainNetworkDNS:       0.50,
			DomainNetworkEgress:    0.85,
			DomainCPUThrottle:      0.10,
			DomainMemoryPressure:   0.10,
			DomainProviderThrottle: 0.75,
			DomainProviderError:    0.40,
			DomainRetrievalBackend: 0.30,
			DomainUnknown:          0.10,
		},
		"tls_handshake_ms": {
			DomainNetworkDNS:       0.10,
			DomainNetworkEgress:    0.30,
			DomainCPUThrottle:      0.10,
			DomainMemoryPressure:   0.10,
			DomainProviderThrottle: 0.80,
			DomainProviderError:    0.50,
			DomainRetrievalBackend: 0.20,
			DomainUnknown:          0.10,
		},
		"cpu_steal_pct": {
			DomainNetworkDNS:       0.10,
			DomainNetworkEgress:    0.10,
			DomainCPUThrottle:      0.90,
			DomainMemoryPressure:   0.20,
			DomainProviderThrottle: 0.10,
			DomainProviderError:    0.10,
			DomainRetrievalBackend: 0.10,
			DomainUnknown:          0.10,
		},
		"cfs_throttled_ms": {
			DomainNetworkDNS:       0.10,
			DomainNetworkEgress:    0.10,
			DomainCPUThrottle:      0.85,
			DomainMemoryPressure:   0.75,
			DomainProviderThrottle: 0.10,
			DomainProviderError:    0.10,
			DomainRetrievalBackend: 0.10,
			DomainUnknown:          0.10,
		},
		"mem_reclaim_latency_ms": {
			DomainNetworkDNS:       0.05,
			DomainNetworkEgress:    0.05,
			DomainCPUThrottle:      0.15,
			DomainMemoryPressure:   0.95,
			DomainProviderThrottle: 0.05,
			DomainProviderError:    0.05,
			DomainRetrievalBackend: 0.05,
			DomainUnknown:          0.05,
		},
		"disk_io_latency_ms": {
			DomainNetworkDNS:       0.05,
			DomainNetworkEgress:    0.05,
			DomainCPUThrottle:      0.10,
			DomainMemoryPressure:   0.85,
			DomainProviderThrottle: 0.05,
			DomainProviderError:    0.05,
			DomainRetrievalBackend: 0.30,
			DomainUnknown:          0.05,
		},
		"syscall_latency_ms": {
			DomainNetworkDNS:       0.10,
			DomainNetworkEgress:    0.20,
			DomainCPUThrottle:      0.15,
			DomainMemoryPressure:   0.10,
			DomainProviderThrottle: 0.90,
			DomainProviderError:    0.60,
			DomainRetrievalBackend: 0.40,
			DomainUnknown:          0.10,
		},
		"connect_errors_total": {
			DomainNetworkDNS:       0.10,
			DomainNetworkEgress:    0.80,
			DomainCPUThrottle:      0.05,
			DomainMemoryPressure:   0.05,
			DomainProviderThrottle: 0.60,
			DomainProviderError:    0.85,
			DomainRetrievalBackend: 0.15,
			DomainUnknown:          0.10,
		},
		"tls_handshake_fail_total": {
			DomainNetworkDNS:       0.05,
			DomainNetworkEgress:    0.70,
			DomainCPUThrottle:      0.05,
			DomainMemoryPressure:   0.05,
			DomainProviderThrottle: 0.30,
			DomainProviderError:    0.60,
			DomainRetrievalBackend: 0.10,
			DomainUnknown:          0.05,
		},
	}
}

// signalThresholds maps signal names to their "elevated" thresholds.
// A signal is considered evidence when its value exceeds this threshold.
var signalThresholds = map[string]float64{
	"dns_latency_ms":           40,
	"tcp_retransmits_total":    2,
	"runqueue_delay_ms":        10,
	"connect_latency_ms":       80,
	"tls_handshake_ms":         60,
	"cpu_steal_pct":            2,
	"cfs_throttled_ms":         40,
	"mem_reclaim_latency_ms":   5,
	"disk_io_latency_ms":       10,
	"syscall_latency_ms":       50,
	"connect_errors_total":     1,
	"tls_handshake_fail_total": 1,
}

// Posterior holds one domain's posterior probability.
type Posterior struct {
	Domain    string
	Posterior float64
	Evidence  []string
}

// Attribute computes Bayesian posteriors over fault domains given observed signals.
// Returns posteriors sorted by probability descending.
func (b *BayesianAttributor) Attribute(signals map[string]float64) []Posterior {
	// Determine which signals are elevated (above threshold).
	elevated := make(map[string]bool)
	for signal, value := range signals {
		if thresh, ok := signalThresholds[signal]; ok && value >= thresh {
			elevated[signal] = true
		}
	}

	domains := AllDomains()
	logPosteriors := make(map[string]float64, len(domains))

	for _, domain := range domains {
		prior := b.Priors[domain]
		if prior <= 0 {
			prior = 1e-10
		}
		logP := math.Log(prior)

		for signal := range b.Likelihoods {
			likelihood := b.likelihoodFor(signal, domain, elevated[signal])
			logP += math.Log(likelihood)
		}
		logPosteriors[domain] = logP
	}

	// Log-sum-exp normalization for numerical stability.
	maxLog := math.Inf(-1)
	for _, lp := range logPosteriors {
		if lp > maxLog {
			maxLog = lp
		}
	}

	sumExp := 0.0
	for _, lp := range logPosteriors {
		sumExp += math.Exp(lp - maxLog)
	}
	logZ := maxLog + math.Log(sumExp)

	result := make([]Posterior, 0, len(domains))
	for _, domain := range domains {
		posterior := math.Exp(logPosteriors[domain] - logZ)

		evidence := make([]string, 0)
		for signal := range elevated {
			if ll, ok := b.Likelihoods[signal]; ok {
				if ll[domain] >= 0.5 {
					evidence = append(evidence, signal)
				}
			}
		}
		sort.Strings(evidence)

		result = append(result, Posterior{
			Domain:    domain,
			Posterior: posterior,
			Evidence:  evidence,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Posterior > result[j].Posterior
	})

	return result
}

// likelihoodFor returns P(signal_state|domain). When the signal is elevated
// it returns the configured likelihood, otherwise (1 - likelihood).
func (b *BayesianAttributor) likelihoodFor(signal, domain string, isElevated bool) float64 {
	ll, ok := b.Likelihoods[signal]
	if !ok {
		return 0.5 // uninformative
	}
	p, ok := ll[domain]
	if !ok {
		return 0.5
	}
	if isElevated {
		return clampLikelihood(p)
	}
	return clampLikelihood(1 - p)
}

func clampLikelihood(p float64) float64 {
	if p < 0.01 {
		return 0.01
	}
	if p > 0.99 {
		return 0.99
	}
	return p
}

// AttributeSample runs Bayesian attribution on a FaultSample and returns
// an IncidentAttribution with populated FaultHypotheses.
func (b *BayesianAttributor) AttributeSample(sample FaultSample) schema.IncidentAttribution {
	base := BuildAttribution(sample)

	if len(sample.Signals) == 0 {
		return base
	}

	posteriors := b.Attribute(sample.Signals)
	hypotheses := make([]schema.FaultHypothesis, 0, len(posteriors))
	for _, p := range posteriors {
		if p.Posterior < 0.01 {
			continue
		}
		hypotheses = append(hypotheses, schema.FaultHypothesis{
			Domain:    p.Domain,
			Posterior: p.Posterior,
			Evidence:  p.Evidence,
		})
	}
	base.FaultHypotheses = hypotheses

	// Override top-1 as primary prediction.
	if len(posteriors) > 0 {
		base.PredictedFaultDomain = posteriors[0].Domain
		base.Confidence = posteriors[0].Posterior
	}

	return base
}

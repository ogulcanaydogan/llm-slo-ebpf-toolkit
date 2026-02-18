package collector

import (
	"fmt"
	"log"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/safety"
)

// ProbeSpec describes a single eBPF probe to be managed.
type ProbeSpec struct {
	Signal     string
	Collection *ebpf.Collection
	Links      []link.Link
	RingBuf    *ringbuf.Reader
}

// ProbeManager loads, attaches, and controls the lifecycle of eBPF probes.
// It integrates with the safety package to disable probes when overhead
// exceeds budget.
type ProbeManager struct {
	mu           sync.Mutex
	mode         string // capability mode label (e.g. "core_full", "bcc_degraded")
	probes       map[string]*ProbeSpec
	allowed      map[string]struct{} // allowed signal set for the mode
	disableOrder []string            // preferred disable order for overhead shedding
	guard        *safety.OverheadGuard
	limiter      *safety.RateLimiter
}

// NewProbeManager creates a manager for the given capability mode. The caller
// provides the allowed signal set and disable order to avoid import cycles
// with the signals package.
func NewProbeManager(mode string, allowed []string, disableOrder []string, guard *safety.OverheadGuard, limiter *safety.RateLimiter) *ProbeManager {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, s := range allowed {
		allowedSet[s] = struct{}{}
	}
	return &ProbeManager{
		mode:         mode,
		probes:       make(map[string]*ProbeSpec),
		allowed:      allowedSet,
		disableOrder: disableOrder,
		guard:        guard,
		limiter:      limiter,
	}
}

// Mode returns the active capability mode label.
func (pm *ProbeManager) Mode() string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.mode
}

// Register adds a probe spec to the manager. The probe is not yet attached.
func (pm *ProbeManager) Register(spec *ProbeSpec) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.allowed[spec.Signal]; !ok {
		return fmt.Errorf("signal %q not supported in mode %s", spec.Signal, pm.mode)
	}

	pm.probes[spec.Signal] = spec
	return nil
}

// AttachAll attaches all registered probes.
func (pm *ProbeManager) AttachAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for sig, spec := range pm.probes {
		if len(spec.Links) > 0 {
			log.Printf("probe %s: already attached, skipping", sig)
			continue
		}
		log.Printf("probe %s: attached", sig)
	}
	return nil
}

// DetachAll detaches all probes and closes resources.
func (pm *ProbeManager) DetachAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for sig, spec := range pm.probes {
		pm.closeProbe(sig, spec)
	}
	pm.probes = make(map[string]*ProbeSpec)
}

// DisableProbe detaches and removes a single probe by signal name.
func (pm *ProbeManager) DisableProbe(signal string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	spec, ok := pm.probes[signal]
	if !ok {
		return false
	}

	pm.closeProbe(signal, spec)
	delete(pm.probes, signal)
	return true
}

// EnabledSignals returns the list of currently attached signal names.
func (pm *ProbeManager) EnabledSignals() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	out := make([]string, 0, len(pm.probes))
	for sig := range pm.probes {
		out = append(out, sig)
	}
	return out
}

// CheckOverhead evaluates current CPU overhead and disables the highest-cost
// probe if the budget is exceeded. Returns the disabled signal name (if any).
func (pm *ProbeManager) CheckOverhead() (string, bool) {
	if pm.guard == nil {
		return "", false
	}

	pct, exceeded, err := pm.guard.Evaluate()
	if err != nil {
		log.Printf("overhead check error: %v", err)
		return "", false
	}

	if !exceeded {
		return "", false
	}

	log.Printf("overhead %.2f%% exceeds budget, disabling highest-cost probe", pct)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, signal := range pm.disableOrder {
		if spec, ok := pm.probes[signal]; ok {
			pm.closeProbe(signal, spec)
			delete(pm.probes, signal)
			return signal, true
		}
	}
	return "", false
}

// RingBufReaders returns all active ring buffer readers for the consumer.
func (pm *ProbeManager) RingBufReaders() []*ringbuf.Reader {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var readers []*ringbuf.Reader
	for _, spec := range pm.probes {
		if spec.RingBuf != nil {
			readers = append(readers, spec.RingBuf)
		}
	}
	return readers
}

func (pm *ProbeManager) closeProbe(signal string, spec *ProbeSpec) {
	for _, l := range spec.Links {
		if err := l.Close(); err != nil {
			log.Printf("probe %s: link close error: %v", signal, err)
		}
	}
	if spec.RingBuf != nil {
		spec.RingBuf.Close()
	}
	if spec.Collection != nil {
		spec.Collection.Close()
	}
	log.Printf("probe %s: detached", signal)
}

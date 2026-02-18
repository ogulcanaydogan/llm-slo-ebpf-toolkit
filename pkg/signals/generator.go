package signals

import (
	"sort"
	"sync"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

type signalProfile struct {
	dnsLatencyMS      float64
	tcpRetransmits    float64
	runqueueDelayMS   float64
	connectLatencyMS  float64
	connectErrors     float64
	connectErrno      int
	tlsHandshakeMS    float64
	tlsHandshakeFails float64
	cpuStealPct       float64
	cfsThrottledMS    float64
}

// Generator emits normalized probe events for the configured signal set.
type Generator struct {
	mu       sync.RWMutex
	mode     CapabilityMode
	enabled  map[string]struct{}
	enricher MetadataEnricher
}

// NewGenerator builds a probe generator with capability filtering.
func NewGenerator(mode CapabilityMode, signalSet []string, enricher MetadataEnricher) *Generator {
	g := &Generator{
		mode:     mode,
		enabled:  make(map[string]struct{}),
		enricher: enricher,
	}
	g.SetSignals(signalSet)
	return g
}

// SetSignals replaces enabled probes at runtime.
func (g *Generator) SetSignals(signalSet []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.enabled = make(map[string]struct{})
	allowed := make(map[string]struct{})
	for _, signal := range SupportedSignalsForMode(g.mode) {
		allowed[signal] = struct{}{}
	}

	if len(signalSet) == 0 {
		for signal := range allowed {
			g.enabled[signal] = struct{}{}
		}
		return
	}

	for _, signal := range signalSet {
		if _, ok := allowed[signal]; ok {
			g.enabled[signal] = struct{}{}
		}
	}
}

// EnabledSignals returns a stable list of enabled signal names.
func (g *Generator) EnabledSignals() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]string, 0, len(g.enabled))
	for signal := range g.enabled {
		out = append(out, signal)
	}
	sort.Strings(out)
	return out
}

// Mode returns current capability mode.
func (g *Generator) Mode() CapabilityMode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.mode
}

// Disable disables one signal if it is currently enabled.
func (g *Generator) Disable(signal string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.enabled[signal]; !ok {
		return false
	}
	delete(g.enabled, signal)
	return true
}

// DisableHighestCost disables the next preferred high-cost enabled signal.
func (g *Generator) DisableHighestCost() (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, signal := range DisableOrder() {
		if _, ok := g.enabled[signal]; ok {
			delete(g.enabled, signal)
			return signal, true
		}
	}
	return "", false
}

// Generate turns one sample into normalized probe events.
func (g *Generator) Generate(sample collector.RawSample, meta Metadata) []schema.ProbeEventV1 {
	g.mu.RLock()
	enabled := make(map[string]struct{}, len(g.enabled))
	for k := range g.enabled {
		enabled[k] = struct{}{}
	}
	g.mu.RUnlock()

	if len(enabled) == 0 {
		return nil
	}

	if g.enricher != nil {
		meta = g.enricher.Enrich(meta)
	}
	profile := profileForFault(sample.FaultLabel)
	tuple := defaultConnTuple(sample)

	out := make([]schema.ProbeEventV1, 0, len(enabled))
	appendIfEnabled := func(signal string, event schema.ProbeEventV1) {
		if _, ok := enabled[signal]; ok {
			out = append(out, event)
		}
	}

	appendIfEnabled(SignalDNSLatencyMS, newEvent(sample.Timestamp, SignalDNSLatencyMS, profile.dnsLatencyMS, "ms", meta, &tuple, 0, 0))
	appendIfEnabled(SignalTCPRetransmits, newEvent(sample.Timestamp, SignalTCPRetransmits, profile.tcpRetransmits, "count", meta, &tuple, 0, 0))
	appendIfEnabled(SignalRunqueueDelayMS, newEvent(sample.Timestamp, SignalRunqueueDelayMS, profile.runqueueDelayMS, "ms", meta, nil, 0, 0))
	appendIfEnabled(SignalConnectLatencyMS, newEvent(sample.Timestamp, SignalConnectLatencyMS, profile.connectLatencyMS, "ms", meta, &tuple, profile.connectErrno, 0))
	appendIfEnabled(SignalConnectErrors, newEvent(sample.Timestamp, SignalConnectErrors, profile.connectErrors, "count", meta, &tuple, profile.connectErrno, 0))
	appendIfEnabled(SignalTLSHandshakeMS, newEvent(sample.Timestamp, SignalTLSHandshakeMS, profile.tlsHandshakeMS, "ms", meta, &tuple, 0, 0))
	appendIfEnabled(SignalTLSHandshakeFails, newEvent(sample.Timestamp, SignalTLSHandshakeFails, profile.tlsHandshakeFails, "count", meta, &tuple, 0, 0))
	appendIfEnabled(SignalCPUStealPct, newEvent(sample.Timestamp, SignalCPUStealPct, profile.cpuStealPct, "pct", meta, nil, 0, 0))
	appendIfEnabled(SignalCFSThrottledMS, newEvent(sample.Timestamp, SignalCFSThrottledMS, profile.cfsThrottledMS, "ms", meta, nil, 0, 0))

	return out
}

func defaultConnTuple(sample collector.RawSample) schema.ConnTuple {
	return schema.ConnTuple{
		SrcIP:    "10.244.0.10",
		DstIP:    "10.244.0.53",
		SrcPort:  42424,
		DstPort:  443,
		Protocol: "tcp",
	}
}

func newEvent(
	ts time.Time,
	signal string,
	value float64,
	unit string,
	meta Metadata,
	tuple *schema.ConnTuple,
	errno int,
	confidence float64,
) schema.ProbeEventV1 {
	event := schema.ProbeEventV1{
		TSUnixNano: ts.UTC().UnixNano(),
		Signal:     signal,
		Node:       meta.Node,
		Namespace:  meta.Namespace,
		Pod:        meta.Pod,
		Container:  meta.Container,
		PID:        meta.PID,
		TID:        meta.TID,
		ConnTuple:  tuple,
		Value:      value,
		Unit:       unit,
		Status:     signalStatus(signal, value),
		TraceID:    meta.TraceID,
		SpanID:     meta.SpanID,
	}
	if errno != 0 {
		event.Errno = &errno
	}
	if confidence > 0 {
		c := confidence
		event.Confidence = &c
	}
	return event
}

func signalStatus(signal string, value float64) string {
	switch signal {
	case SignalDNSLatencyMS:
		return threshold(value, 40, 120)
	case SignalTCPRetransmits:
		return threshold(value, 2, 5)
	case SignalRunqueueDelayMS:
		return threshold(value, 10, 25)
	case SignalConnectLatencyMS:
		return threshold(value, 80, 180)
	case SignalConnectErrors:
		return threshold(value, 1, 3)
	case SignalTLSHandshakeMS:
		return threshold(value, 60, 160)
	case SignalTLSHandshakeFails:
		return threshold(value, 1, 3)
	case SignalCPUStealPct:
		return threshold(value, 2, 8)
	case SignalCFSThrottledMS:
		return threshold(value, 40, 120)
	default:
		return "ok"
	}
}

func threshold(value float64, warning float64, errorCutoff float64) string {
	if value >= errorCutoff {
		return "error"
	}
	if value >= warning {
		return "warning"
	}
	return "ok"
}

func profileForFault(faultLabel string) signalProfile {
	base := signalProfile{
		dnsLatencyMS:      12,
		tcpRetransmits:    0.2,
		runqueueDelayMS:   4,
		connectLatencyMS:  18,
		connectErrors:     0,
		connectErrno:      0,
		tlsHandshakeMS:    22,
		tlsHandshakeFails: 0,
		cpuStealPct:       0.6,
		cfsThrottledMS:    5,
	}

	switch faultLabel {
	case "dns_latency":
		base.dnsLatencyMS = 220
		base.connectLatencyMS = 130
	case "cpu_throttle":
		base.runqueueDelayMS = 28
		base.cpuStealPct = 9
		base.cfsThrottledMS = 170
	case "memory_pressure":
		base.runqueueDelayMS = 14
		base.cfsThrottledMS = 90
	case "provider_throttle":
		base.connectLatencyMS = 45
		base.tlsHandshakeMS = 55
		base.connectErrors = 1
		base.connectErrno = 110
	}
	return base
}

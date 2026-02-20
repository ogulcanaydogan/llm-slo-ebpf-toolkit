package signals

import (
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
)

func TestGeneratorCoreFullEmitsRequiredSignals(t *testing.T) {
	sample := collector.RawSample{
		Timestamp:  time.Unix(1710000000, 0).UTC(),
		FaultLabel: "dns_latency",
	}
	g := NewGenerator(CapabilityCoreFull, nil, StaticMetadataEnricher{
		Defaults: Metadata{
			Node:      "kind-worker",
			Namespace: "default",
			Pod:       "rag-0",
			Container: "rag",
			PID:       100,
			TID:       100,
		},
	})

	events := g.Generate(sample, Metadata{})
	if len(events) < 9 {
		t.Fatalf("expected at least 9 events, got %d", len(events))
	}

	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Signal] = true
		if event.Pod == "" || event.Node == "" || event.Namespace == "" {
			t.Fatalf("expected enriched metadata for %s", event.Signal)
		}
	}

	for _, signal := range RequiredMinimumSignals() {
		if !seen[signal] {
			t.Fatalf("required signal missing: %s", signal)
		}
	}
}

func TestGeneratorBCCModeFiltersSignalSet(t *testing.T) {
	sample := collector.RawSample{
		Timestamp:  time.Unix(1710000000, 0).UTC(),
		FaultLabel: "cpu_throttle",
	}
	g := NewGenerator(CapabilityBCCDegraded, nil, StaticMetadataEnricher{
		Defaults: Metadata{
			Node:      "kind-worker",
			Namespace: "default",
			Pod:       "rag-0",
			Container: "rag",
			PID:       100,
			TID:       100,
		},
	})

	events := g.Generate(sample, Metadata{})
	if len(events) != 2 {
		t.Fatalf("expected 2 events in bcc mode, got %d", len(events))
	}
	for _, event := range events {
		if event.Signal != SignalDNSLatencyMS && event.Signal != SignalTCPRetransmits {
			t.Fatalf("unexpected signal in bcc mode: %s", event.Signal)
		}
	}
}

func TestGeneratorEmitsNewV03Signals(t *testing.T) {
	sample := collector.RawSample{
		Timestamp:  time.Unix(1710000000, 0).UTC(),
		FaultLabel: "memory_pressure",
	}
	g := NewGenerator(CapabilityCoreFull, nil, StaticMetadataEnricher{
		Defaults: Metadata{
			Node:      "kind-worker",
			Namespace: "default",
			Pod:       "rag-0",
			Container: "rag",
			PID:       100,
			TID:       100,
		},
	})

	events := g.Generate(sample, Metadata{})
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Signal] = true
	}

	for _, signal := range []string{
		SignalMemReclaimLatencyMS,
		SignalDiskIOLatencyMS,
		SignalSyscallLatencyMS,
	} {
		if !seen[signal] {
			t.Errorf("v0.3 signal missing from Generate output: %s", signal)
		}
	}
}

func TestGeneratorMemoryPressureProfile(t *testing.T) {
	sample := collector.RawSample{
		Timestamp:  time.Unix(1710000000, 0).UTC(),
		FaultLabel: "memory_pressure",
	}
	g := NewGenerator(CapabilityCoreFull, nil, StaticMetadataEnricher{
		Defaults: Metadata{Node: "n", Namespace: "ns", Pod: "p", Container: "c", PID: 1, TID: 1},
	})

	events := g.Generate(sample, Metadata{})
	for _, event := range events {
		if event.Signal == SignalMemReclaimLatencyMS && event.Value < 20 {
			t.Errorf("mem_reclaim_latency_ms under memory_pressure should be elevated, got %f", event.Value)
		}
		if event.Signal == SignalDiskIOLatencyMS && event.Value < 50 {
			t.Errorf("disk_io_latency_ms under memory_pressure should be elevated, got %f", event.Value)
		}
	}
}

func TestGeneratorDisableHighestCost(t *testing.T) {
	g := NewGenerator(CapabilityCoreFull, nil, nil)
	first, ok := g.DisableHighestCost()
	if !ok {
		t.Fatal("expected disable candidate")
	}
	if first != SignalTLSHandshakeMS {
		t.Fatalf("expected highest-cost tls signal, got %s", first)
	}
}

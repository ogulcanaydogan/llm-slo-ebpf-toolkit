package collector

import (
	"testing"
)

var (
	testCoreSignals = []string{
		"dns_latency_ms", "tcp_retransmits_total", "runqueue_delay_ms",
		"connect_latency_ms", "connect_errors_total", "tls_handshake_ms",
		"tls_handshake_fail_total", "cpu_steal_pct", "cfs_throttled_ms",
	}
	testBCCSignals   = []string{"dns_latency_ms", "tcp_retransmits_total"}
	testDisableOrder = []string{
		"tls_handshake_ms", "runqueue_delay_ms", "connect_latency_ms",
		"cpu_steal_pct", "dns_latency_ms", "tcp_retransmits_total",
	}
)

func TestProbeManagerRegister(t *testing.T) {
	pm := NewProbeManager("core_full", testCoreSignals, testDisableOrder, nil, nil)

	err := pm.Register(&ProbeSpec{Signal: "dns_latency_ms"})
	if err != nil {
		t.Fatalf("register dns: %v", err)
	}

	err = pm.Register(&ProbeSpec{Signal: "tcp_retransmits_total"})
	if err != nil {
		t.Fatalf("register tcp: %v", err)
	}

	enabled := pm.EnabledSignals()
	if len(enabled) != 2 {
		t.Errorf("enabled count: got %d, want 2", len(enabled))
	}
}

func TestProbeManagerRegisterBCCRejects(t *testing.T) {
	pm := NewProbeManager("bcc_degraded", testBCCSignals, nil, nil, nil)

	// DNS is allowed in BCC mode
	err := pm.Register(&ProbeSpec{Signal: "dns_latency_ms"})
	if err != nil {
		t.Fatalf("register dns in BCC: %v", err)
	}

	// Runqueue delay is NOT allowed in BCC mode
	err = pm.Register(&ProbeSpec{Signal: "runqueue_delay_ms"})
	if err == nil {
		t.Error("expected error registering runqueue in BCC mode")
	}
}

func TestProbeManagerDisableProbe(t *testing.T) {
	pm := NewProbeManager("core_full", testCoreSignals, testDisableOrder, nil, nil)
	if err := pm.Register(&ProbeSpec{Signal: "dns_latency_ms"}); err != nil {
		t.Fatalf("register dns: %v", err)
	}
	if err := pm.Register(&ProbeSpec{Signal: "tcp_retransmits_total"}); err != nil {
		t.Fatalf("register tcp: %v", err)
	}

	ok := pm.DisableProbe("dns_latency_ms")
	if !ok {
		t.Error("expected DisableProbe to return true")
	}

	enabled := pm.EnabledSignals()
	if len(enabled) != 1 {
		t.Errorf("enabled count after disable: got %d, want 1", len(enabled))
	}

	ok = pm.DisableProbe("nonexistent")
	if ok {
		t.Error("expected DisableProbe for nonexistent to return false")
	}
}

func TestProbeManagerDetachAll(t *testing.T) {
	pm := NewProbeManager("core_full", testCoreSignals, testDisableOrder, nil, nil)
	if err := pm.Register(&ProbeSpec{Signal: "dns_latency_ms"}); err != nil {
		t.Fatalf("register dns: %v", err)
	}
	if err := pm.Register(&ProbeSpec{Signal: "tcp_retransmits_total"}); err != nil {
		t.Fatalf("register tcp: %v", err)
	}
	if err := pm.Register(&ProbeSpec{Signal: "runqueue_delay_ms"}); err != nil {
		t.Fatalf("register runqueue: %v", err)
	}

	pm.DetachAll()

	enabled := pm.EnabledSignals()
	if len(enabled) != 0 {
		t.Errorf("enabled count after detach: got %d, want 0", len(enabled))
	}
}

func TestProbeManagerMode(t *testing.T) {
	pm := NewProbeManager("bcc_degraded", testBCCSignals, nil, nil, nil)
	if pm.Mode() != "bcc_degraded" {
		t.Errorf("mode: got %q, want %q", pm.Mode(), "bcc_degraded")
	}
}

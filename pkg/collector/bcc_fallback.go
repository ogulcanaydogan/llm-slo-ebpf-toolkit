package collector

import (
	"fmt"
	"log"
)

// bccSignals are the only signals supported in BCC degraded mode.
var bccSignals = []string{"dns_latency_ms", "tcp_retransmits_total"}

// BCCFallback provides degraded-mode signal collection using BCC when
// CO-RE / BTF is unavailable. Only DNS latency and TCP retransmits
// are supported in this mode.
type BCCFallback struct {
	enabled map[string]struct{}
	active  bool
}

// NewBCCFallback creates a BCC fallback collector.
func NewBCCFallback() *BCCFallback {
	enabled := make(map[string]struct{})
	for _, sig := range bccSignals {
		enabled[sig] = struct{}{}
	}
	return &BCCFallback{
		enabled: enabled,
	}
}

// SupportedSignals returns the signals available in BCC degraded mode.
func (b *BCCFallback) SupportedSignals() []string {
	out := make([]string, len(bccSignals))
	copy(out, bccSignals)
	return out
}

// Start initializes BCC-based tracing for DNS and TCP retransmit signals.
// This is a stub: actual BCC integration requires the bcc-tools package
// and Python/C interop which is deferred to a later milestone. The
// capability flag is surfaced so reports correctly indicate degraded mode.
func (b *BCCFallback) Start() error {
	if b.active {
		return fmt.Errorf("bcc fallback already active")
	}

	log.Printf("bcc fallback: starting degraded mode with %d signals", len(b.enabled))
	b.active = true
	return nil
}

// Stop tears down BCC tracing.
func (b *BCCFallback) Stop() {
	if !b.active {
		return
	}
	log.Printf("bcc fallback: stopping degraded mode")
	b.active = false
}

// IsActive returns whether BCC fallback is currently running.
func (b *BCCFallback) IsActive() bool {
	return b.active
}

// CapabilityFlags returns metadata for benchmark reports indicating
// degraded mode and which signals are available.
func (b *BCCFallback) CapabilityFlags() map[string]interface{} {
	return map[string]interface{}{
		"mode":              "bcc_degraded",
		"supported_signals": b.SupportedSignals(),
		"degraded":          true,
		"note":              "BCC fallback: DNS and TCP retransmits only; CO-RE unavailable",
	}
}

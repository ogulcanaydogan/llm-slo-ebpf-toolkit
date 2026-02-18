package signals

// Signal keys exposed by the agent.
const (
	SignalDNSLatencyMS      = "dns_latency_ms"
	SignalTCPRetransmits    = "tcp_retransmits_total"
	SignalRunqueueDelayMS   = "runqueue_delay_ms"
	SignalConnectLatencyMS  = "connect_latency_ms"
	SignalConnectErrors     = "connect_errors_total"
	SignalTLSHandshakeMS    = "tls_handshake_ms"
	SignalTLSHandshakeFails = "tls_handshake_fail_total"
	SignalCPUStealPct       = "cpu_steal_pct"
	SignalCFSThrottledMS    = "cfs_throttled_ms"
)

// CapabilityMode defines probe coverage level.
type CapabilityMode string

const (
	CapabilityCoreFull    CapabilityMode = "core_full"
	CapabilityBCCDegraded CapabilityMode = "bcc_degraded"
)

var (
	coreSignalSet = []string{
		SignalDNSLatencyMS,
		SignalTCPRetransmits,
		SignalRunqueueDelayMS,
		SignalConnectLatencyMS,
		SignalConnectErrors,
		SignalTLSHandshakeMS,
		SignalTLSHandshakeFails,
		SignalCPUStealPct,
		SignalCFSThrottledMS,
	}
	bccSignalSet = []string{
		SignalDNSLatencyMS,
		SignalTCPRetransmits,
	}
	highCostDisableOrder = []string{
		SignalTLSHandshakeMS,
		SignalRunqueueDelayMS,
		SignalConnectLatencyMS,
		SignalCPUStealPct,
		SignalDNSLatencyMS,
		SignalTCPRetransmits,
		SignalCFSThrottledMS,
		SignalConnectErrors,
		SignalTLSHandshakeFails,
	}
)

// RequiredMinimumSignals returns the six required v0.2 signal names.
func RequiredMinimumSignals() []string {
	return []string{
		SignalDNSLatencyMS,
		SignalTCPRetransmits,
		SignalRunqueueDelayMS,
		SignalConnectLatencyMS,
		SignalTLSHandshakeMS,
		SignalCPUStealPct,
	}
}

// SupportedSignalsForMode returns the supported signal set for a capability mode.
func SupportedSignalsForMode(mode CapabilityMode) []string {
	switch mode {
	case CapabilityBCCDegraded:
		return cloneStrings(bccSignalSet)
	default:
		return cloneStrings(coreSignalSet)
	}
}

// DisableOrder returns preferred signal disable order when overhead exceeds budget.
func DisableOrder() []string {
	return cloneStrings(highCostDisableOrder)
}

func cloneStrings(src []string) []string {
	out := make([]string, len(src))
	copy(out, src)
	return out
}

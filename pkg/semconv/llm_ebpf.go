package semconv

const (
	AttrDNSLatencyMS       = "llm.ebpf.dns.latency_ms"
	AttrTCPRetransmits     = "llm.ebpf.tcp.retransmits"
	AttrRunqueueDelayMS    = "llm.ebpf.sched.runqueue_delay_ms"
	AttrCPUStealPct        = "llm.ebpf.cpu.steal_pct"
	AttrConnectLatencyMS   = "llm.ebpf.net.connect_latency_ms"
	AttrTLSHandshakeMS     = "llm.ebpf.tls.handshake_ms"
	AttrCorrelationConf    = "llm.ebpf.correlation_confidence"
	AttrSLOTTFTMS          = "llm.slo.ttft_ms"
	AttrSLOTokensPerSec    = "llm.slo.tokens_per_sec"
	AttrRetrievalVectorDB  = "llm.slo.retrieval.vectordb_ms"
	AttrRetrievalNetworkMS = "llm.slo.retrieval.network_ms"
	AttrRetrievalDNSMS     = "llm.slo.retrieval.dns_ms"
)

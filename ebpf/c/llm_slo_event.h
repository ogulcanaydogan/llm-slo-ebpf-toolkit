#ifndef __LLM_SLO_EVENT_H
#define __LLM_SLO_EVENT_H

/* Signal type identifiers for ring buffer event discrimination. */
enum llm_slo_signal_type {
    LLM_SLO_DNS_LATENCY     = 1,
    LLM_SLO_TCP_RETRANSMIT  = 2,
    LLM_SLO_RUNQUEUE_DELAY  = 3,
    LLM_SLO_CONNECT_LATENCY = 4,
    LLM_SLO_TLS_HANDSHAKE   = 5,
    LLM_SLO_CPU_STEAL       = 6,
};

/*
 * llm_slo_event is the shared ring buffer event structure emitted by all
 * CO-RE probes. The Go-side consumer decodes signal_type to route events
 * to the appropriate signal constant and schema field.
 *
 * Fields:
 *   pid, tid        — task identifiers for pod+pid correlation tier
 *   timestamp_ns    — ktime_get_ns() capture for window matching
 *   signal_type     — discriminator (see enum above)
 *   value_ns        — latency in nanoseconds, count, or fixed-point pct*100
 *   conn_src_port   — source port for conn_tuple correlation
 *   conn_dst_port   — destination port (53=DNS, 443=TLS, etc.)
 *   conn_dst_ip     — destination IPv4 in network byte order
 *   errno_val       — kernel errno when applicable (connect failures)
 */
struct llm_slo_event {
    __u32 pid;
    __u32 tid;
    __u64 timestamp_ns;
    __u32 signal_type;
    __u64 value_ns;
    __u16 conn_src_port;
    __u16 conn_dst_port;
    __u32 conn_dst_ip;
    __s32 errno_val;
} __attribute__((packed));

#endif /* __LLM_SLO_EVENT_H */

/*
 * tcp_retransmit.bpf.c — Counts TCP retransmission events via the
 * tcp:tcp_retransmit_skb tracepoint. Each retransmit emits a ring
 * buffer event with the connection tuple for correlation.
 *
 * Hook point:
 *   tracepoint/tcp/tcp_retransmit_skb
 *
 * Signal: tcp_retransmits_total (LLM_SLO_TCP_RETRANSMIT)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

SEC("tracepoint/tcp/tcp_retransmit_skb")
int handle_tcp_retransmit(struct trace_event_raw_tcp_retransmit_skb *ctx) {
    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_TCP_RETRANSMIT;
    event->value_ns      = 1; /* count: 1 retransmit event */
    event->conn_src_port = ctx->sport;
    event->conn_dst_port = ctx->dport;
    event->conn_dst_ip   = 0; /* filled from skb if needed */
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

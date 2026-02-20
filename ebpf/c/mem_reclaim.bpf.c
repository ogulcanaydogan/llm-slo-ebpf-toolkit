/*
 * mem_reclaim.bpf.c — Measures direct memory reclaim latency by timing
 * the vmscan direct reclaim begin/end tracepoints. Events are emitted to
 * a ring buffer for Go-side consumption.
 *
 * Hook points:
 *   tracepoint/vmscan/mm_vmscan_direct_reclaim_begin — records start timestamp
 *   tracepoint/vmscan/mm_vmscan_direct_reclaim_end   — computes delta, emits event
 *
 * Signal: mem_reclaim_latency_ms (LLM_SLO_MEM_RECLAIM)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

/* Tracks in-flight reclaim start timestamps keyed by pid_tgid. */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u64);   /* pid_tgid */
    __type(value, __u64); /* start timestamp_ns */
} reclaim_start SEC(".maps");

/* Ring buffer for emitting events to userspace. */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

SEC("tracepoint/vmscan/mm_vmscan_direct_reclaim_begin")
int handle_reclaim_begin(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&reclaim_start, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

SEC("tracepoint/vmscan/mm_vmscan_direct_reclaim_end")
int handle_reclaim_end(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 *start_ns = bpf_map_lookup_elem(&reclaim_start, &pid_tgid);
    if (!start_ns)
        return 0;

    __u64 delta_ns = bpf_ktime_get_ns() - *start_ns;
    bpf_map_delete_elem(&reclaim_start, &pid_tgid);

    /* Filter out very short reclaims (<10us) to reduce noise. */
    if (delta_ns < 10000)
        return 0;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_MEM_RECLAIM;
    event->value_ns      = delta_ns;
    event->conn_src_port = 0;
    event->conn_dst_port = 0;
    event->conn_dst_ip   = 0;
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

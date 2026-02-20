/*
 * disk_io_latency.bpf.c — Measures block device I/O request latency by
 * timing the interval between block_rq_issue and block_rq_complete
 * tracepoints. Events are emitted to a ring buffer for Go-side consumption.
 *
 * Hook points:
 *   tracepoint/block/block_rq_issue    — records issue timestamp keyed by (dev, sector)
 *   tracepoint/block/block_rq_complete — computes delta, emits event
 *
 * Signal: disk_io_latency_ms (LLM_SLO_DISK_IO_LATENCY)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

struct blk_key {
    __u32 dev;
    __u64 sector;
};

/* Tracks in-flight block I/O requests keyed by (dev, sector). */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, struct blk_key);
    __type(value, __u64); /* start timestamp_ns */
} blk_start SEC(".maps");

/* Ring buffer for emitting events to userspace. */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

SEC("tracepoint/block/block_rq_issue")
int handle_block_rq_issue(struct trace_event_raw_block_rq_completion *ctx) {
    struct blk_key key = {
        .dev    = ctx->dev,
        .sector = ctx->sector,
    };
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&blk_start, &key, &ts, BPF_ANY);
    return 0;
}

SEC("tracepoint/block/block_rq_complete")
int handle_block_rq_complete(struct trace_event_raw_block_rq_completion *ctx) {
    struct blk_key key = {
        .dev    = ctx->dev,
        .sector = ctx->sector,
    };
    __u64 *start_ns = bpf_map_lookup_elem(&blk_start, &key);
    if (!start_ns)
        return 0;

    __u64 delta_ns = bpf_ktime_get_ns() - *start_ns;
    bpf_map_delete_elem(&blk_start, &key);

    /* Filter out very fast I/O (<500us) to focus on blocking operations. */
    if (delta_ns < 500000)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_DISK_IO_LATENCY;
    event->value_ns      = delta_ns;
    event->conn_src_port = 0;
    event->conn_dst_port = 0;
    event->conn_dst_ip   = 0;
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

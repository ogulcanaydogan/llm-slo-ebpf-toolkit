/*
 * runqueue_delay.bpf.c — Measures scheduler run-queue latency: the time
 * between a task being woken up (enqueued) and actually being scheduled
 * on a CPU. High values indicate CPU contention or CFS throttling.
 *
 * Hook points:
 *   tracepoint/sched/sched_wakeup       — records enqueue timestamp
 *   tracepoint/sched/sched_wakeup_new   — records enqueue for new tasks
 *   tracepoint/sched/sched_switch       — computes delta on context switch
 *
 * Signal: runqueue_delay_ms (LLM_SLO_RUNQUEUE_DELAY)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

/* Tracks wakeup timestamps keyed by pid. */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, __u32);   /* pid */
    __type(value, __u64); /* wakeup timestamp_ns */
} runq_enqueue SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

static __always_inline int record_wakeup(__u32 pid) {
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&runq_enqueue, &pid, &ts, BPF_ANY);
    return 0;
}

SEC("tracepoint/sched/sched_wakeup")
int handle_sched_wakeup(struct trace_event_raw_sched_wakeup_template *ctx) {
    return record_wakeup(ctx->pid);
}

SEC("tracepoint/sched/sched_wakeup_new")
int handle_sched_wakeup_new(struct trace_event_raw_sched_wakeup_template *ctx) {
    return record_wakeup(ctx->pid);
}

SEC("tracepoint/sched/sched_switch")
int handle_sched_switch(struct trace_event_raw_sched_switch *ctx) {
    /* The task being switched IN is next_pid. */
    __u32 pid = ctx->next_pid;
    __u64 *enqueue_ts = bpf_map_lookup_elem(&runq_enqueue, &pid);
    if (!enqueue_ts)
        return 0;

    __u64 now = bpf_ktime_get_ns();
    __u64 delta_ns = now - *enqueue_ts;

    bpf_map_delete_elem(&runq_enqueue, &pid);

    /* Only emit events with measurable delay (>100us) to reduce noise. */
    if (delta_ns < 100000)
        return 0;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    event->pid           = pid;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = now;
    event->signal_type   = LLM_SLO_RUNQUEUE_DELAY;
    event->value_ns      = delta_ns;
    event->conn_src_port = 0;
    event->conn_dst_port = 0;
    event->conn_dst_ip   = 0;
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

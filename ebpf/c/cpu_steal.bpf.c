/*
 * cpu_steal.bpf.c — Captures CPU steal time / involuntary wait by
 * hooking sched:sched_stat_wait. This tracepoint fires when a task
 * accumulates involuntary wait time (time the vCPU was preempted by
 * the hypervisor or delayed by scheduler contention).
 *
 * In VM/container environments, elevated values indicate the host is
 * overcommitting CPU, directly impacting inference latency.
 *
 * Hook point:
 *   tracepoint/sched/sched_stat_wait — captures involuntary wait
 *
 * Signal: cpu_steal_pct (LLM_SLO_CPU_STEAL)
 *
 * Note: value_ns carries the raw wait duration in nanoseconds. The
 * Go-side consumer aggregates these into a percentage over a sampling
 * window using total CPU time from /proc/stat.
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

/* Minimum wait threshold to reduce noise: 50us. */
#define MIN_WAIT_NS 50000

SEC("tracepoint/sched/sched_stat_wait")
int handle_sched_stat_wait(struct trace_event_raw_sched_stat_template *ctx) {
    __u64 delay_ns = ctx->delay;

    if (delay_ns < MIN_WAIT_NS)
        return 0;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    event->pid           = ctx->pid;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_CPU_STEAL;
    event->value_ns      = delay_ns;
    event->conn_src_port = 0;
    event->conn_dst_port = 0;
    event->conn_dst_ip   = 0;
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

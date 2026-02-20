/*
 * syscall_latency.bpf.c — Measures blocking read/write syscall latency
 * by timing kprobe/kretprobe pairs on ksys_read and ksys_write. Captures
 * slow I/O calls that indicate provider API response latency or disk
 * contention.
 *
 * Hook points:
 *   kprobe/ksys_read    — records start timestamp keyed by pid_tgid
 *   kretprobe/ksys_read — computes delta, emits if above threshold
 *   kprobe/ksys_write   — records start timestamp keyed by pid_tgid
 *   kretprobe/ksys_write — computes delta, emits if above threshold
 *
 * Signal: syscall_latency_ms (LLM_SLO_SYSCALL_LATENCY)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

/* Tracks in-flight syscall start timestamps keyed by pid_tgid. */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, __u64);   /* pid_tgid */
    __type(value, __u64); /* start timestamp_ns */
} syscall_start SEC(".maps");

/* Ring buffer for emitting events to userspace. */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

static __always_inline int handle_entry(void) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&syscall_start, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

static __always_inline int handle_return(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 *start_ns = bpf_map_lookup_elem(&syscall_start, &pid_tgid);
    if (!start_ns)
        return 0;

    __u64 delta_ns = bpf_ktime_get_ns() - *start_ns;
    bpf_map_delete_elem(&syscall_start, &pid_tgid);

    /* Only emit slow syscalls (>=1ms) to focus on blocking calls. */
    if (delta_ns < 1000000)
        return 0;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_SYSCALL_LATENCY;
    event->value_ns      = delta_ns;
    event->conn_src_port = 0;
    event->conn_dst_port = 0;
    event->conn_dst_ip   = 0;
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

SEC("kprobe/ksys_read")
int BPF_KPROBE(kprobe_ksys_read) {
    return handle_entry();
}

SEC("kretprobe/ksys_read")
int BPF_KRETPROBE(kretprobe_ksys_read) {
    return handle_return(ctx);
}

SEC("kprobe/ksys_write")
int BPF_KPROBE(kprobe_ksys_write) {
    return handle_entry();
}

SEC("kretprobe/ksys_write")
int BPF_KRETPROBE(kretprobe_ksys_write) {
    return handle_return(ctx);
}

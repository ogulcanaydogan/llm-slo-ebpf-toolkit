/*
 * dns_latency.bpf.c — Measures DNS resolution latency by timing UDP
 * sends to port 53 (udp_sendmsg) and their corresponding receives
 * (udp_recvmsg). Events are emitted to a ring buffer for Go-side
 * consumption.
 *
 * Hook points:
 *   kprobe/udp_sendmsg   — records start timestamp keyed by (pid, tid)
 *   kretprobe/udp_recvmsg — computes delta if dst_port was 53
 *
 * Signal: dns_latency_ms (LLM_SLO_DNS_LATENCY)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

/* Tracks in-flight DNS send timestamps keyed by pid_tgid. */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u64);   /* pid_tgid */
    __type(value, __u64); /* start timestamp_ns */
} dns_start SEC(".maps");

/* Ring buffer for emitting events to userspace. */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

/*
 * Temporary storage for destination port extracted from the socket.
 * Keyed by pid_tgid so we can check port 53 on the return path.
 */
struct send_ctx {
    __u64 start_ns;
    __u16 dst_port;
    __u32 dst_ip;
    __u16 src_port;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u64);
    __type(value, struct send_ctx);
} dns_inflight SEC(".maps");

SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(kprobe_udp_sendmsg, struct sock *sk) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u16 dst_port = 0;
    __u32 dst_ip = 0;
    __u16 src_port = 0;

    BPF_CORE_READ_INTO(&dst_port, sk, __sk_common.skc_dport);
    dst_port = __builtin_bswap16(dst_port);

    /* Only track DNS traffic (port 53). */
    if (dst_port != 53)
        return 0;

    BPF_CORE_READ_INTO(&dst_ip, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&src_port, sk, __sk_common.skc_num);

    struct send_ctx ctx = {
        .start_ns = bpf_ktime_get_ns(),
        .dst_port = dst_port,
        .dst_ip   = dst_ip,
        .src_port = src_port,
    };

    bpf_map_update_elem(&dns_inflight, &pid_tgid, &ctx, BPF_ANY);
    return 0;
}

SEC("kprobe/udp_recvmsg")
int BPF_KPROBE(kprobe_udp_recvmsg) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    struct send_ctx *ctx = bpf_map_lookup_elem(&dns_inflight, &pid_tgid);
    if (!ctx)
        return 0;

    __u64 delta_ns = bpf_ktime_get_ns() - ctx->start_ns;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event) {
        bpf_map_delete_elem(&dns_inflight, &pid_tgid);
        return 0;
    }

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_DNS_LATENCY;
    event->value_ns      = delta_ns;
    event->conn_src_port = ctx->src_port;
    event->conn_dst_port = ctx->dst_port;
    event->conn_dst_ip   = ctx->dst_ip;
    event->errno_val     = 0;

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&dns_inflight, &pid_tgid);
    return 0;
}

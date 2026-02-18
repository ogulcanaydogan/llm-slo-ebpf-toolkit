/*
 * connect_latency.bpf.c — Measures TCP connect() latency by timing
 * tcp_v4_connect / tcp_v6_connect calls. Captures both successful
 * connections and failures with errno.
 *
 * Hook points:
 *   kprobe/tcp_v4_connect  — records start timestamp
 *   kretprobe/tcp_v4_connect — computes delta, captures errno
 *   kprobe/tcp_v6_connect  — records start timestamp
 *   kretprobe/tcp_v6_connect — computes delta, captures errno
 *
 * Signal: connect_latency_ms (LLM_SLO_CONNECT_LATENCY)
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

struct connect_ctx {
    __u64 start_ns;
    __u16 dst_port;
    __u32 dst_ip;
    __u16 src_port;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u64);   /* pid_tgid */
    __type(value, struct connect_ctx);
} connect_inflight SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

static __always_inline int enter_connect(struct sock *sk) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u16 dst_port = 0;
    __u32 dst_ip = 0;
    __u16 src_port = 0;

    BPF_CORE_READ_INTO(&dst_port, sk, __sk_common.skc_dport);
    dst_port = __builtin_bswap16(dst_port);
    BPF_CORE_READ_INTO(&dst_ip, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&src_port, sk, __sk_common.skc_num);

    struct connect_ctx ctx = {
        .start_ns = bpf_ktime_get_ns(),
        .dst_port = dst_port,
        .dst_ip   = dst_ip,
        .src_port = src_port,
    };
    bpf_map_update_elem(&connect_inflight, &pid_tgid, &ctx, BPF_ANY);
    return 0;
}

static __always_inline int exit_connect(int ret) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    struct connect_ctx *ctx = bpf_map_lookup_elem(&connect_inflight, &pid_tgid);
    if (!ctx)
        return 0;

    __u64 delta_ns = bpf_ktime_get_ns() - ctx->start_ns;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event) {
        bpf_map_delete_elem(&connect_inflight, &pid_tgid);
        return 0;
    }

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_CONNECT_LATENCY;
    event->value_ns      = delta_ns;
    event->conn_src_port = ctx->src_port;
    event->conn_dst_port = ctx->dst_port;
    event->conn_dst_ip   = ctx->dst_ip;
    event->errno_val     = ret < 0 ? -ret : 0;

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&connect_inflight, &pid_tgid);
    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(kprobe_tcp_v4_connect, struct sock *sk) {
    return enter_connect(sk);
}

SEC("kretprobe/tcp_v4_connect")
int BPF_KRETPROBE(kretprobe_tcp_v4_connect, int ret) {
    return exit_connect(ret);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(kprobe_tcp_v6_connect, struct sock *sk) {
    return enter_connect(sk);
}

SEC("kretprobe/tcp_v6_connect")
int BPF_KRETPROBE(kretprobe_tcp_v6_connect, int ret) {
    return exit_connect(ret);
}

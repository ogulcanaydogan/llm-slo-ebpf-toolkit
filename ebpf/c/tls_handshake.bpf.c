/*
 * tls_handshake.bpf.c — Measures TLS handshake latency by attaching
 * uprobes to OpenSSL's SSL_do_handshake. This captures the duration
 * of the TLS negotiation phase that contributes to TTFT.
 *
 * Hook points:
 *   uprobe/SSL_do_handshake   — records start timestamp
 *   uretprobe/SSL_do_handshake — computes delta, captures success/failure
 *
 * Signal: tls_handshake_ms (LLM_SLO_TLS_HANDSHAKE)
 *
 * Note: Uprobe attachment requires the path to libssl.so to be provided
 * at load time by the probe manager. The BPF program defines the probe
 * logic; the Go side handles finding and attaching to the correct library.
 */
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "llm_slo_event.h"

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u64);   /* pid_tgid */
    __type(value, __u64); /* start timestamp_ns */
} tls_start SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} llm_slo_events SEC(".maps");

SEC("uprobe/SSL_do_handshake")
int BPF_UPROBE(uprobe_ssl_do_handshake) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&tls_start, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

SEC("uretprobe/SSL_do_handshake")
int BPF_URETPROBE(uretprobe_ssl_do_handshake, int ret) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 *start_ts = bpf_map_lookup_elem(&tls_start, &pid_tgid);
    if (!start_ts)
        return 0;

    __u64 delta_ns = bpf_ktime_get_ns() - *start_ts;

    struct llm_slo_event *event =
        bpf_ringbuf_reserve(&llm_slo_events, sizeof(*event), 0);
    if (!event) {
        bpf_map_delete_elem(&tls_start, &pid_tgid);
        return 0;
    }

    event->pid           = pid_tgid >> 32;
    event->tid           = (__u32)pid_tgid;
    event->timestamp_ns  = bpf_ktime_get_ns();
    event->signal_type   = LLM_SLO_TLS_HANDSHAKE;
    event->value_ns      = delta_ns;
    event->conn_src_port = 0;
    event->conn_dst_port = 443; /* conventional TLS port */
    event->conn_dst_ip   = 0;
    event->errno_val     = ret <= 0 ? 1 : 0; /* SSL_do_handshake: 1=success */

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&tls_start, &pid_tgid);
    return 0;
}

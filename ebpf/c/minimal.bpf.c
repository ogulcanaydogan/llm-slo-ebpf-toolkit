#include "vmlinux.h"
#include "bpf_helpers.h"

char LICENSE[] SEC("license") = "GPL";

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);
    __type(value, __u64);
} llm_slo_counter SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_nanosleep")
int handle_sys_enter_nanosleep(struct trace_event_raw_sys_enter *ctx) {
    __u32 key = 0;
    __u64 init = 1;
    __u64 *val;

    val = bpf_map_lookup_elem(&llm_slo_counter, &key);
    if (!val) {
        bpf_map_update_elem(&llm_slo_counter, &key, &init, BPF_ANY);
        return 0;
    }

    __sync_fetch_and_add(val, 1);
    return 0;
}

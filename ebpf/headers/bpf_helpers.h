#ifndef __BPF_HELPERS_H
#define __BPF_HELPERS_H

#define SEC(name) __attribute__((section(name), used))

#define __uint(name, val) int (*name)[val]
#define __type(name, val) val *name

#define BPF_ANY 0

typedef unsigned short __u16;
typedef unsigned int __u32;
typedef unsigned long long __u64;

static void *(*bpf_map_lookup_elem)(void *map, const void *key) = (void *)1;
static int (*bpf_map_update_elem)(void *map, const void *key, const void *value, __u64 flags) = (void *)2;

#endif

// SPDX-License-Identifier: GPL-2.0
//
// =============================================================================
// block.bpf.c — Citadel egress packet filter (eBPF cgroup_skb/egress)
// =============================================================================
//
// What this program does
// ----------------------
// Attaches to the cgroup_skb/egress hook on the runner's root cgroup. Every
// outbound IPv4 packet flows through this program. We look up the destination
// IP in a userspace-populated `blocked_ips` hash map; if present, the packet
// is dropped. Otherwise it's passed through.
//
// Userspace (Go) adds IPs to the map when it sees an outbound TCP connect to
// a non-allowlisted destination. The first SYN may sneak through (the kprobe
// + ringbuf path has nonzero latency), but every subsequent packet to the
// same IP — including retransmits and data segments — gets dropped, so the
// connection effectively fails to carry payload.
//
// cgroup_skb specifics
// --------------------
// For SEC("cgroup_skb/egress"), skb->data points at the L3 (IP) header (no
// L2/eth header in this hook). We verify skb->protocol == ETH_P_IP (the L2
// protocol field is still meaningful even though there's no L2 header).

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define ETH_P_IP 0x0800

// userspace inserts/deletes; BPF program only reads.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u32);   // destination IPv4 in network byte order
    __type(value, __u8);  // unused, presence is the signal
} blocked_ips SEC(".maps");

SEC("cgroup_skb/egress")
int cg_egress_filter(struct __sk_buff *skb)
{
    // We only inspect IPv4 here; IPv6 and other L3 protocols pass through.
    if (skb->protocol != bpf_htons(ETH_P_IP))
        return 1;

    // Manual packet parsing — read just enough of the IP header to grab
    // daddr without violating the verifier. We can't dereference data/data_end
    // directly through bpf_skb_load_bytes here because cgroup_skb has limited
    // direct packet access in some kernels, but it does support __sk_buff
    // helpers.
    __u32 daddr = 0;
    if (bpf_skb_load_bytes(skb, 16 /* offset of daddr in IPv4 hdr */,
                           &daddr, sizeof(daddr)) < 0)
        return 1;

    __u8 *blocked = bpf_map_lookup_elem(&blocked_ips, &daddr);
    if (blocked)
        return 0; // drop

    return 1; // allow
}

char LICENSE[] SEC("license") = "GPL";

// SPDX-License-Identifier: GPL-2.0
//
// =============================================================================
// net.bpf.c — Citadel network egress probe (eBPF)
// =============================================================================
//
// What this program does
// ----------------------
// Attaches a kprobe to the kernel function `tcp_v4_connect` so we get notified
// every time *any* process inside the runner initiates an outbound IPv4 TCP
// connection. For each connection, we capture:
//
//   - pid, ppid, uid of the calling task
//   - 16-byte process comm (e.g. "curl", "node")
//   - destination IPv4 address (network byte order — userspace converts)
//   - destination port      (network byte order — userspace converts)
//   - monotonic timestamp (ns)
//
// Events are streamed to userspace via a `BPF_MAP_TYPE_RINGBUF` map called
// `net_events`. The Go agent reads this ringbuffer and enriches each event
// with process ancestry, reverse DNS, and workflow metadata before posting to
// the backend.
//
// Why a kprobe on tcp_v4_connect?
// ------------------------------
// `tcp_v4_connect(struct sock *sk, struct sockaddr *uaddr, int addr_len)` is
// the kernel function the IPv4 TCP stack invokes for every connect() syscall
// targeting an IPv4 socket. By the time it runs, the kernel has already
// validated and copied the user-supplied sockaddr into kernel memory, so we
// can safely use kernel reads (BPF_CORE_READ) on the sockaddr_in fields.
//
// CO-RE (Compile Once, Run Everywhere)
// -----------------------------------
// We pull kernel struct definitions from `vmlinux.h`, which is generated on
// the target runner via:
//
//     bpftool btf dump file /sys/kernel/btf/vmlinux format c > bpf/vmlinux.h
//
// `BPF_CORE_READ` / `BPF_CORE_READ_INTO` apply field-offset relocations at
// load time, so this object file is portable across kernels that share BTF.
//
// Struct layout
// -------------
// The event struct is laid out so that there are no implicit padding bytes:
// after `dport (u16)` we add an explicit `_pad (u16)` to align `ts_ns (u64)`.
// The Go side mirrors this layout exactly — any drift will scramble fields.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define TASK_COMM_LEN 16

// ---------------------------------------------------------------------------
// Event emitted to userspace
// ---------------------------------------------------------------------------
struct net_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    char  comm[TASK_COMM_LEN];
    __u32 saddr;       // source IPv4 (network byte order; 0 at connect time)
    __u32 daddr;       // destination IPv4 (network byte order)
    __u16 dport;       // destination port (network byte order)
    __u16 _pad;        // explicit padding so ts_ns is 8-byte aligned
    __u64 ts_ns;       // bpf_ktime_get_ns()
};

// Force the verifier to include the type so userspace tooling (bpf2go) can
// emit a matching Go type if desired.
const struct net_event *unused_net_event __attribute__((unused));

// ---------------------------------------------------------------------------
// Ringbuffer used to stream events to userspace
// ---------------------------------------------------------------------------
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256 KiB ringbuffer
} net_events SEC(".maps");

// ---------------------------------------------------------------------------
// kprobe handler — fires on entry to tcp_v4_connect
// ---------------------------------------------------------------------------
//
// BPF_KPROBE expands to a function whose signature matches the underlying
// kernel function's positional arguments. So `uaddr` here is the second
// argument to tcp_v4_connect: the destination sockaddr (kernel pointer).
//
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(handle_tcp_connect,
               struct sock *sk,
               struct sockaddr *uaddr,
               int addr_len)
{
    // Reserve space in the ringbuffer up front. If it fails (ringbuf full),
    // we drop the event rather than blocking the syscall.
    struct net_event *e = bpf_ringbuf_reserve(&net_events, sizeof(*e), 0);
    if (!e)
        return 0;

    // ---- pid / tgid ----
    // bpf_get_current_pid_tgid() packs tgid in the high 32 bits, pid in the
    // low 32 bits. For a multi-threaded process, "pid" in userspace usually
    // means tgid — so we emit the tgid as `pid`.
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)(pid_tgid >> 32);

    // ---- uid ----
    // bpf_get_current_uid_gid() packs gid high, uid low.
    e->uid = (__u32)bpf_get_current_uid_gid();

    // ---- ppid via task_struct->real_parent->tgid (CO-RE) ----
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = BPF_CORE_READ(task, real_parent, tgid);

    // ---- comm ----
    bpf_get_current_comm(&e->comm, sizeof(e->comm));

    // ---- destination IPv4 + port from sockaddr_in ----
    // tcp_v4_connect only handles AF_INET, so casting to sockaddr_in is safe.
    // Both s_addr and sin_port are stored in network byte order — we copy the
    // raw bytes; userspace converts to host order / net.IP.
    struct sockaddr_in *sin = (struct sockaddr_in *)uaddr;
    BPF_CORE_READ_INTO(&e->daddr, sin, sin_addr.s_addr);
    BPF_CORE_READ_INTO(&e->dport, sin, sin_port);

    // saddr is not meaningful at connect-entry time (the kernel hasn't bound
    // a source address yet). We leave it as 0 and let userspace ignore it.
    e->saddr = 0;
    e->_pad  = 0;

    e->ts_ns = bpf_ktime_get_ns();

    bpf_ringbuf_submit(e, 0);
    return 0;
}

// ---------------------------------------------------------------------------
// License — required for kprobe + ringbuf helpers
// ---------------------------------------------------------------------------
char LICENSE[] SEC("license") = "GPL";

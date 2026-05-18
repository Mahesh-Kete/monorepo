// SPDX-License-Identifier: GPL-2.0
//
// =============================================================================
// file.bpf.c — Citadel filesystem-write probe (eBPF)
// =============================================================================
//
// What this program does
// ----------------------
// Attaches a tracepoint to `syscalls/sys_enter_openat`. Every time *any*
// process opens a file with write/create intent, we emit an event:
//
//   - pid, ppid, uid of the opener
//   - 16-byte comm
//   - filename (user-space path, up to 256 bytes)
//   - openat() flags (so userspace can render "WRONLY|CREAT" etc.)
//   - monotonic timestamp (ns)
//
// Events flow over the `file_events` ringbuffer. The Go agent filters in
// userspace to keep only paths under the configured workspace prefix
// (default `/home/runner/work/`) and drop `/proc`, `/sys`, `/tmp`, etc.
//
// Why filter in userspace, not the kernel?
// ----------------------------------------
// The build plan's Phase 3 spec says "Workspace-only filter in userspace."
// Doing the prefix match in BPF would mean bounded string compare, which is
// fiddly with the verifier. The kernel emits everything; Go decides what to
// keep. Trade-off: more ringbuf traffic. For hackathon scope this is fine.
//
// openat signature
// ----------------
// `int openat(int dirfd, const char *pathname, int flags, mode_t mode)`
//   - dirfd      → ctx->args[0]
//   - pathname   → ctx->args[1]  (user-space pointer)
//   - flags      → ctx->args[2]
//   - mode       → ctx->args[3]
//
// We filter on flags & (O_WRONLY | O_RDWR | O_CREAT) to drop the (huge) flood
// of read-only opens. Definitions:
//   O_WRONLY = 0x1, O_RDWR = 0x2, O_CREAT = 0x40

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define TASK_COMM_LEN 16

#define O_WRONLY 0x1
#define O_RDWR   0x2
#define O_CREAT  0x40

// No internal padding needed: comm[16]+filename[256] starts at offset 12 so
// flags (s32, 4-aligned) lands at 284, and ts_ns (u64, 8-aligned) lands at
// 288. 288 % 8 == 0 — natural alignment. Total size = 296 bytes.
struct file_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    char  comm[TASK_COMM_LEN];
    char  filename[256];
    __s32 flags;
    __u64 ts_ns;
};

const struct file_event *unused_file_event __attribute__((unused));

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} file_events SEC(".maps");

// Generic sys_enter tracepoint context layout.
struct sys_enter_args {
    unsigned long long unused;   // common trace_entry header
    long              id;
    unsigned long     args[6];
};

SEC("tracepoint/syscalls/sys_enter_openat")
int handle_openat_enter(struct sys_enter_args *ctx)
{
    int flags = (int)ctx->args[2];

    // Filter: only opens with write or create intent. Read-only opens are
    // ~99% of openat traffic and we don't care about them.
    if (!(flags & (O_WRONLY | O_RDWR | O_CREAT)))
        return 0;

    struct file_event *e = bpf_ringbuf_reserve(&file_events, sizeof(*e), 0);
    if (!e)
        return 0;

    __builtin_memset(e->filename, 0, sizeof(e->filename));

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)(pid_tgid >> 32);
    e->uid = (__u32)bpf_get_current_uid_gid();

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = BPF_CORE_READ(task, real_parent, tgid);

    bpf_get_current_comm(&e->comm, sizeof(e->comm));

    // pathname is a user-space pointer — use bpf_probe_read_user_str.
    const char *path_user = (const char *)ctx->args[1];
    bpf_probe_read_user_str(&e->filename, sizeof(e->filename), path_user);

    e->flags = flags;
    e->ts_ns = bpf_ktime_get_ns();

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";

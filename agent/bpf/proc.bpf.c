// SPDX-License-Identifier: GPL-2.0
//
// =============================================================================
// proc.bpf.c — Citadel process-exec probe (eBPF)
// =============================================================================
//
// What this program does
// ----------------------
// Attaches a tracepoint to `sched/sched_process_exec` so we observe every
// process exec on the runner. For each exec we capture:
//
//   - pid, ppid, uid of the new process
//   - 16-byte comm
//   - filename of the executed program (up to 128 bytes)
//   - the raw argv blob (NUL-separated, up to 256 bytes — userspace splits)
//   - monotonic timestamp (ns)
//
// Events flow to userspace via the `proc_events` ringbuffer. The Go agent
// stores each one in an in-memory process tree so that *later* events
// (network, file) can be tagged with their full process ancestry — e.g.
// `[bash, npm, node, curl]` for a curl call deep inside an npm script.
//
// Tracepoint vs kprobe
// --------------------
// We use a tracepoint here (not a kprobe) because `sched_process_exec` is a
// stable ABI: the tracepoint context struct is part of the kernel's
// commitment. kprobes on the underlying functions break across kernel
// versions. Tracepoints are slightly slower but rock-solid.
//
// Tracepoint format reference:
//   cat /sys/kernel/debug/tracing/events/sched/sched_process_exec/format
//
// Reading argv
// ------------
// `current->mm->arg_start` and `current->mm->arg_end` are user-space pointers
// bracketing the argv blob. We use `bpf_probe_read_user()` (NOT _str — argv
// has embedded NULs between args) to copy up to 256 bytes. Userspace then
// splits on NUL.
//
// Struct alignment
// ----------------
// The compiler would auto-insert 4 bytes of padding between `args[256]` (ends
// at offset 412) and the 8-aligned `ts_ns`. We declare the pad explicitly so
// the Go side can mirror the layout without surprise. Total size = 424 bytes.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define TASK_COMM_LEN 16

struct citadel_proc_event {
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    char  comm[TASK_COMM_LEN];
    char  filename[128];
    char  args[256];
    __u32 _pad;           // explicit padding for 8-byte ts_ns alignment
    __u64 ts_ns;
};

const struct citadel_proc_event *unused_proc_event __attribute__((unused));

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} proc_events SEC(".maps");

// Mirror of the sched_process_exec tracepoint format. The `__data_loc` field
// is a 32-bit value where the low 16 bits are the data offset from the start
// of the trace record. We read the filename string from that offset.
struct sched_process_exec_args {
    unsigned long long unused;   // common trace_entry header (8 bytes)
    __u32 filename_loc;
    __u32 pid;
    __u32 old_pid;
};

SEC("tracepoint/sched/sched_process_exec")
int handle_sched_process_exec(struct sched_process_exec_args *ctx)
{
    struct citadel_proc_event *e = bpf_ringbuf_reserve(&proc_events, sizeof(*e), 0);
    if (!e)
        return 0;

    // Zero the dynamic-size buffers — ringbuf reservation doesn't zero memory.
    __builtin_memset(e->filename, 0, sizeof(e->filename));
    __builtin_memset(e->args,     0, sizeof(e->args));

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)(pid_tgid >> 32);
    e->uid = (__u32)bpf_get_current_uid_gid();
    e->_pad = 0;

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    e->ppid = BPF_CORE_READ(task, real_parent, tgid);

    bpf_get_current_comm(&e->comm, sizeof(e->comm));

    // Read the exec'd filename from the tracepoint's __data_loc field.
    // The low 16 bits of filename_loc are the byte offset from `ctx`.
    unsigned short off = ctx->filename_loc & 0xFFFF;
    bpf_probe_read_kernel_str(&e->filename, sizeof(e->filename),
                              (const char *)ctx + off);

    // Read up to 256 bytes of the argv blob from user memory. argv args are
    // NUL-separated; userspace splits.
    unsigned long arg_start = BPF_CORE_READ(task, mm, arg_start);
    unsigned long arg_end   = BPF_CORE_READ(task, mm, arg_end);
    if (arg_start && arg_end > arg_start) {
        bpf_probe_read_user(&e->args, sizeof(e->args), (const void *)arg_start);
    }

    e->ts_ns = bpf_ktime_get_ns();
    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";

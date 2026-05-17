# /agent/bpf

eBPF C source files compiled with `clang -O2 -g -target bpf` and loaded by the Go agent via `github.com/cilium/ebpf` + `bpf2go` code generation.

## Files

- `vmlinux.h` — generated once via `bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h` and committed. Provides CO-RE-portable kernel type definitions.
- `net.bpf.c` — kprobe on `tcp_v4_connect`. Captures outbound TCP connections (pid, comm, dst IP, dst port).
- `proc.bpf.c` — tracepoint `sched/sched_process_exec`. Captures every process exec with filename + argv.
- `file.bpf.c` — tracepoint `syscalls/sys_enter_openat`. Captures writeable file opens for tampering detection.
- `block.bpf.c` — `cgroup_skb/egress` program for in-kernel packet drops in block mode.

## Build output

Compiled object files land under `build/` (e.g. `build/net.bpf.o`) and are embedded into the Go binary by `bpf2go` at `go generate` time.

## Regenerating `vmlinux.h`

```sh
sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h
```

Pin this to the runner's kernel — different kernels produce different `vmlinux.h`.

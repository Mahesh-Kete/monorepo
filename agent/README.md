# /agent

The Citadel agent: a Go binary that loads eBPF programs and streams runtime events from inside a CI/CD runner to the backend.

Runs as a `--privileged` container with `--pid=host` so it can attach BPF programs to the host kernel and resolve PIDs across the runner.

## Layout

- `cmd/agent/` — entrypoint, flag parsing, signal handling
- `bpf/` — eBPF C source compiled with `clang -target bpf` (see `/agent/bpf/README.md`)
- `internal/probes/{net,proc,file,block}/` — Go wrappers around each BPF program (one per probe), generated bindings via `bpf2go`
- `internal/dns/` — reverse-DNS cache used to enrich network events
- `internal/proctree/` — in-memory process ancestry cache built from process events
- `internal/integrity/` — workspace snapshot + diff for tampering detection
- `internal/workflow/` — reads `GITHUB_*` metadata and the current-step sentinel
- `internal/events/` — unified `Event` schema posted to the backend
- `internal/backend/` — batching/retrying HTTP client
- `internal/policy/` — policy loader and allowlist matching
- `internal/enforcer/` — userspace process-kill enforcement

## Build

```sh
make bpf        # compile each .bpf.c → bpf/build/*.o
make build      # go generate ./... + go build
make clean
```

See `/agent/Dockerfile` for the multi-stage container build.

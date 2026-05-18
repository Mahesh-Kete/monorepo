# Local Demo (no GHCR required)

This walks through running Citadel end-to-end on a single self-hosted Ubuntu
runner using **locally-built Docker images**, without pushing anything to
GHCR. Useful for the hackathon demo where you don't want a network dependency
on a registry.

## Prerequisites

- Ubuntu 22.04 (or any kernel ≥ 5.15) with `docker`, `make`, `clang-14`,
  `libbpf-dev`, `linux-headers-$(uname -r)`, Go 1.25+, Node 20+, Python 3.11+
- A self-hosted GitHub Actions runner registered to your repo with the label
  `citadel-runner` (see the build plan's pre-flight checklist)
- `/sys/kernel/btf/vmlinux` must exist (CONFIG_DEBUG_INFO_BTF=y)

## One-time setup

```sh
# 1) Generate vmlinux.h for this kernel (one-time, commit the result)
sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c > agent/bpf/vmlinux.h

# 2) Build all four images locally
make local-images

# Expected:
#   citadel-agent:dev      ~120 MB
#   citadel-backend:dev    ~25 MB
#   citadel-detector:dev   ~180 MB
#   citadel-dashboard:dev  ~250 MB

# 3) Bring up backend + detector + dashboard
make docker-up

# Sanity:
curl http://localhost:8080/healthz
curl http://localhost:8000/healthz
open http://localhost:3000
```

The agent is **not** in `docker-compose.yml` because it needs host-level eBPF.
It runs from the GitHub Actions composite step.

## Using the action in a workflow

Reference the action from `examples/` (or wherever your workflows live):

```yaml
- name: Citadel
  uses: ./action          # path to /action in this repo
  with:
    mode: audit           # or 'block'
    backend-url: http://localhost:8080
    image-tag: dev        # ← the magic flag; skips docker pull, uses local images
    watch-path: ${{ github.workspace }}
```

When `image-tag: dev`, the composite action references images as
`citadel-agent:dev` (no registry prefix), so it'll use whatever `make
local-images` produced.

## Running an attack demo

The three demo workflows are in `/examples`:

```sh
gh workflow run attack-1-exfil.yml      # exfiltration via curl
gh workflow run attack-2-revshell.yml   # reverse shell via npm postinstall
gh workflow run attack-3-tamper.yml     # source-code modification
```

Watch the dashboard at `http://localhost:3000/runs` — each one shows up as a
new run with detections in the sidebar.

## Reset between demos

```sh
make demo-reset
make docker-up
```

That clears the SQLite database, the detector's baseline state, any leftover
iptables rules, and the agent container. Use it freely — it's idempotent.

## What to do if a demo fails

1. **Verifier rejection on probe load**: check that `agent/bpf/vmlinux.h` was
   generated on *this* kernel. Different kernels produce different BTF.
2. **Agent container exits immediately**: `docker logs citadel` — usually a
   missing capability (`--privileged`) or a wrong cgroup path.
3. **No events showing up**: tail the agent log inside the container —
   `docker exec citadel cat /tmp/citadel-agent.log`.
4. **Dashboard shows red status dot**: backend container not running. Check
   `docker compose ps`.

See `/docs/DEMO.md` for the live walkthrough script.

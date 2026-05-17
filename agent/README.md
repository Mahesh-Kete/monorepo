# agent

Go binary that runs on a self-hosted GitHub Actions runner alongside the job.

## Responsibilities

- Capture DNS queries made by processes inside the job.
- Capture outbound TCP/UDP connection attempts.
- Correlate each network event with the currently executing workflow step
  (using runner metadata / environment / process tree).
- Enforce a policy: allow, log, or block destinations.
- Ship events to the backend ingest API in near real time.

## Approach (planned)

- DNS: tap the local resolver (e.g. an on-host resolver the runner is pointed
  at, or eBPF on Linux) so we see the name being resolved, not just the IP.
- Egress: eBPF / netfilter hook to observe connect() and optionally drop
  packets to non-allowlisted destinations.
- Step correlation: read GitHub Actions runner state (`GITHUB_*` env, the
  runner's `_diag` files, or step boundaries reported by the composite action
  wrapper in `/action`).

## Status

Placeholder. No code yet.

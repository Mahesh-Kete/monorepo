# guardrail

A runtime security layer for GitHub Actions. Guardrail captures outbound network
calls from CI jobs, correlates them with workflow steps, and blocks unapproved
destinations.

## Why

CI runners execute third-party code (actions, build tools, scripts) with broad
network access. A compromised dependency can exfiltrate secrets, beacon to a C2,
or pull additional payloads — and nothing in the default GitHub Actions runner
will notice. Guardrail sits on the runner, observes DNS and egress at the host
level, attributes each connection to the workflow step that initiated it, and
enforces an allowlist policy.

## Components

| Path                  | Description                                                                     |
| --------------------- | ------------------------------------------------------------------------------- |
| `/agent`              | Go binary that runs on self-hosted runners, captures DNS, enforces policy.      |
| `/backend`            | Go API server with SQLite that ingests events from agents.                      |
| `/dashboard`          | Next.js UI for viewing events, building policies, and reviewing incidents.     |
| `/action`             | Composite GitHub Action wrapper that installs and starts the agent in a job.    |
| `/example-workflows`  | Sample victim and attack workflows used for end-to-end demos.                   |
| `/docs`               | Architecture notes, threat model, design decisions.                             |

## Build targets

Top-level `make` targets (Makefiles are stubs for now):

```
make agent       # build the runner-side Go agent
make backend     # build the ingest API server
make dashboard   # build the Next.js dashboard
```

## Status

Early hackathon scaffolding. Nothing here works yet — directory layout and
placeholders only.

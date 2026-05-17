# backend

Go API server, backed by SQLite, that ingests events from runner-side agents.

## Responsibilities

- Accept event streams (DNS queries, connection attempts, policy decisions)
  from `/agent` instances.
- Persist events with workflow/run/step attribution.
- Serve a read API for the `/dashboard` UI:
  - Recent runs and their network activity
  - Per-step destination lists
  - Policy violations / blocked attempts
- Serve / manage policy documents that agents pull at startup.

## Tech

- Go (standard `net/http` or a thin router).
- SQLite via `modernc.org/sqlite` or `mattn/go-sqlite3`.
- JSON over HTTP for ingest and read APIs.

## Status

Placeholder. No code yet.

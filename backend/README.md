# /backend

Go HTTP API backed by SQLite. The system of record for every event the agent emits and every detection the detector produces.

## Stack

- `github.com/go-chi/chi/v5` — router
- `modernc.org/sqlite` — pure-Go SQLite (no CGO)
- `github.com/google/uuid` — IDs

## Endpoints

- `GET  /healthz`
- `POST /api/events` — batch ingest from the agent
- `GET  /api/runs` — recent runs with per-type event counts
- `GET  /api/runs/:id` — single run (events + detections; `?type=` filter supported)
- `GET  /api/runs/:id/process-tree` — reconstructed process tree
- `GET  /api/runs/:id/baseline-domains` — distinct hostnames seen in the run
- `GET  /api/detections?since=ISO_TIME` — for detector polling
- `POST /api/detections` — detector writes findings
- `GET/POST /api/policies` — policy CRUD
- `GET  /api/policies/applicable?repo=X&workflow=Y` — most specific policy match

## Run

```sh
go run ./cmd/backend --db-path ./citadel.db
```

# /detector

Python detection service. Pulls new events from the backend on a short polling interval, runs them through detection rules, learns a per-job baseline, and POSTs detections back to the backend.

## Stack

- Python 3.11 + FastAPI + httpx
- Pydantic models mirroring the Go `Event` schema

## Rules

- `new_domain` — outbound to a domain not in the per-job baseline (after baseline is `stable`)
- `reverse_shell` — build-tool ancestor → interactive shell, especially with concurrent egress
- `source_tamper` — file write under `$GITHUB_WORKSPACE` between checkout and build
- `suspicious_exec` — exec from `/tmp`, `/dev/shm`, `/var/tmp`, or unexpected downloader
- `token_in_payload` — secret regex match in captured argv / network metadata

## Run

```sh
BACKEND_URL=http://localhost:8080 uvicorn app.main:app --reload
```

The worker loop starts on app startup and polls `BACKEND_URL` every `POLL_INTERVAL_SECONDS` (default `2`).

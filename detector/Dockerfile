# Phase 0 STUB Dockerfile.
#
# Replaced in Phase 6 by python:3.11-slim + FastAPI + httpx running uvicorn
# alongside a background polling worker. For now: just stay alive so
# `docker compose up -d` shows detector as Up.
FROM python:3.11-slim

WORKDIR /app

RUN printf '%s\n' \
    'import os, time, sys' \
    'print("citadel-detector Phase 0 stub", flush=True)' \
    'print(f"BACKEND_URL={os.environ.get(\"BACKEND_URL\", \"<unset>\")}", flush=True)' \
    'print(f"POLL_INTERVAL_SECONDS={os.environ.get(\"POLL_INTERVAL_SECONDS\", \"<unset>\")}", flush=True)' \
    'while True:' \
    '    time.sleep(60)' \
    > stub.py

CMD ["python", "stub.py"]

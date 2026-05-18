"""Citadel detector — FastAPI app + background polling worker.

Two responsibilities:
  1. Expose ``/healthz`` and ``/stats`` so docker-compose and the dashboard
     can probe liveness and see what the worker has been doing.
  2. Spin up a single background asyncio task on startup that polls the
     backend's ``GET /api/events`` every POLL_INTERVAL_SECONDS, evaluates
     detection rules, and POSTs results to ``/api/detections``.
"""

from __future__ import annotations

import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI

from .baseline import Baseline
from .client import BackendClient
from .engine import Engine
from .worker import Worker

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s :: %(message)s",
)
log = logging.getLogger("citadel.detector")

POLL_INTERVAL = float(os.environ.get("POLL_INTERVAL_SECONDS", "2.0"))
BASELINE_PATH = os.environ.get("BASELINE_PATH", "/data/baseline.json")
STATE_PATH = os.environ.get("STATE_PATH", "/data/state.json")


_client: BackendClient | None = None
_worker: Worker | None = None


@asynccontextmanager
async def lifespan(_: FastAPI):
    global _client, _worker
    _client = BackendClient()
    baseline = Baseline(BASELINE_PATH)
    engine = Engine(baseline)
    _worker = Worker(_client, engine, poll_interval=POLL_INTERVAL, state_path=STATE_PATH)
    _worker.start()
    log.info("citadel-detector ready (backend=%s, poll=%.1fs)",
             _client.base_url, POLL_INTERVAL)
    try:
        yield
    finally:
        if _worker:
            await _worker.stop()
        if _client:
            await _client.close()


app = FastAPI(title="citadel-detector", lifespan=lifespan)


@app.get("/healthz")
async def healthz():
    return {"status": "ok"}


@app.get("/stats")
async def stats():
    if _worker is None:
        return {"status": "not_started"}
    s = _worker.stats
    return {
        "cycles": s.cycles,
        "events_seen": s.events_seen,
        "detections_emitted": s.detections_emitted,
        "last_event_id": s.last_event_id,
        "last_error": s.last_error,
        "poll_interval_seconds": POLL_INTERVAL,
        "backend_url": _client.base_url if _client else None,
    }

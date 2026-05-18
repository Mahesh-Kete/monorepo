"""Async polling worker: pulls events from the backend on an interval,
evaluates rules, ships detections back. Runs alongside the FastAPI app via
a startup hook.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
from pathlib import Path

from .baseline import Baseline
from .client import BackendClient
from .engine import Engine

log = logging.getLogger("citadel.detector.worker")

DEFAULT_STATE_PATH = "/data/state.json"


class WorkerStats:
    """Lightweight counters surfaced by GET /stats."""
    def __init__(self):
        self.cycles = 0
        self.events_seen = 0
        self.detections_emitted = 0
        self.last_event_id = 0
        self.last_error: str = ""


class Worker:
    def __init__(
        self,
        client: BackendClient,
        engine: Engine,
        poll_interval: float = 2.0,
        state_path: str = DEFAULT_STATE_PATH,
    ):
        self.client = client
        self.engine = engine
        self.poll_interval = poll_interval
        self.state_path = Path(state_path)
        self.state_path.parent.mkdir(parents=True, exist_ok=True)
        self.stats = WorkerStats()
        self._stop = asyncio.Event()
        self._task: asyncio.Task | None = None

    def start(self) -> None:
        if self._task is None or self._task.done():
            self._task = asyncio.create_task(self._loop(), name="citadel-detector-worker")

    async def stop(self) -> None:
        self._stop.set()
        if self._task:
            try:
                await asyncio.wait_for(self._task, timeout=5.0)
            except asyncio.TimeoutError:
                log.warning("worker did not stop in 5s; cancelling")
                self._task.cancel()

    # ------------------------------------------------------------------ loop

    async def _loop(self) -> None:
        last_id = self._load_state()
        self.stats.last_event_id = last_id
        log.info("citadel-detector worker started (last_event_id=%d, interval=%.1fs)",
                 last_id, self.poll_interval)

        while not self._stop.is_set():
            try:
                events = await self.client.fetch_events(after_id=last_id)
                if events:
                    log.info("fetched %d new events (after_id=%d)", len(events), last_id)

                for le in events:
                    detections = self.engine.evaluate(le)
                    for det in detections:
                        backend_id = await self.client.post_detection(det)
                        self.stats.detections_emitted += 1
                        log.info(
                            "DETECTION %s/%s run=%d event=%d -> backend_id=%s :: %s",
                            det.severity, det.rule_name, det.run_id, det.event_id or 0,
                            backend_id, det.message,
                        )

                    last_id = le.id
                    self.stats.events_seen += 1
                    self.stats.last_event_id = last_id

                if events:
                    self._save_state(last_id)

            except Exception as exc:  # noqa: BLE001
                self.stats.last_error = str(exc)
                log.warning("worker cycle error: %s", exc)

            self.stats.cycles += 1
            try:
                await asyncio.wait_for(self._stop.wait(), timeout=self.poll_interval)
            except asyncio.TimeoutError:
                continue
            break  # _stop was set

        log.info("citadel-detector worker stopped")

    # ------------------------------------------------------------------ state I/O

    def _load_state(self) -> int:
        if not self.state_path.exists():
            return 0
        try:
            data = json.loads(self.state_path.read_text())
            return int(data.get("last_event_id", 0))
        except (OSError, ValueError) as e:
            log.warning("state load failed (%s); starting from id=0", e)
            return 0

    def _save_state(self, last_id: int) -> None:
        tmp = self.state_path.with_suffix(".json.tmp")
        try:
            tmp.write_text(json.dumps({"last_event_id": last_id}))
            os.replace(tmp, self.state_path)
        except OSError as e:
            log.warning("state save failed: %s", e)

"""HTTP client for the Citadel backend (``GET /api/events``, ``POST /api/detections``).

Async via httpx so the FastAPI app and the polling worker share one event loop.
"""

from __future__ import annotations

import logging
import os
from typing import Optional

import httpx

from .models import Detection, ListedEvent

log = logging.getLogger("citadel.detector.client")


class BackendClient:
    def __init__(self, base_url: Optional[str] = None, timeout: float = 10.0):
        self.base_url = (base_url or os.environ.get("BACKEND_URL", "http://backend:8080")).rstrip("/")
        self.client = httpx.AsyncClient(timeout=timeout)

    async def close(self) -> None:
        await self.client.aclose()

    async def healthz(self) -> bool:
        try:
            r = await self.client.get(f"{self.base_url}/healthz")
            return r.status_code == 200
        except httpx.HTTPError:
            return False

    async def fetch_events(self, after_id: int = 0, limit: int = 500) -> list[ListedEvent]:
        """Fetch all events with DB id > ``after_id``, oldest first.

        Returns an empty list on transport errors (caller retries on next tick).
        """
        try:
            r = await self.client.get(
                f"{self.base_url}/api/events",
                params={"after_id": after_id, "limit": limit},
            )
            r.raise_for_status()
        except httpx.HTTPError as e:
            log.warning("fetch_events failed: %s", e)
            return []
        try:
            rows = r.json()
        except ValueError as e:
            log.warning("fetch_events bad JSON: %s", e)
            return []
        return [ListedEvent.model_validate(row) for row in rows]

    async def post_detection(self, det: Detection) -> Optional[int]:
        """POST a detection. Returns the backend's id on success, ``None`` on failure."""
        try:
            r = await self.client.post(
                f"{self.base_url}/api/detections",
                json=det.model_dump(exclude_none=True),
            )
            r.raise_for_status()
            data = r.json()
            return data.get("id")
        except httpx.HTTPError as e:
            log.warning("post_detection failed (%s/%s): %s", det.rule_name, det.severity, e)
            return None

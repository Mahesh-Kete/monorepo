"""Per-job baseline learner.

We keep a per-(repository, workflow, job) record of the domains, processes,
and file-write paths that have been observed across runs. After 3 runs the
baseline is considered "stable" and rules can start flagging deviations.

Persisted as JSON on disk so detector restarts don't lose state.
"""

from __future__ import annotations

import json
import logging
import os
import threading
from dataclasses import asdict, dataclass, field
from pathlib import Path

from .models import Event

log = logging.getLogger("citadel.detector.baseline")

# Status thresholds — see status() below.
STABLE_RUNS = 3
UNSTABLE_NEW_ENDPOINTS = 10  # if last 3 runs added > N endpoints, treat as unstable


@dataclass
class _JobState:
    domains: set[str] = field(default_factory=set)
    processes: set[str] = field(default_factory=set)
    file_writes: set[str] = field(default_factory=set)
    runs_seen: set[str] = field(default_factory=set)  # set of GITHUB_RUN_IDs
    recent_new_endpoints: list[int] = field(default_factory=list)  # rolling count per run

    def to_dict(self) -> dict:
        d = asdict(self)
        # JSON doesn't have sets — convert.
        for k in ("domains", "processes", "file_writes", "runs_seen"):
            d[k] = sorted(d[k])
        return d

    @classmethod
    def from_dict(cls, d: dict) -> "_JobState":
        out = cls()
        out.domains = set(d.get("domains", []))
        out.processes = set(d.get("processes", []))
        out.file_writes = set(d.get("file_writes", []))
        out.runs_seen = set(d.get("runs_seen", []))
        out.recent_new_endpoints = list(d.get("recent_new_endpoints", []))
        return out


JobKey = tuple[str, str, str]  # (repository, workflow, job)


class Baseline:
    """Thread-safe baseline. ``update()`` is called by the worker, the rules
    only call the ``is_known_*`` and ``status`` query methods."""

    def __init__(self, path: str = "/data/baseline.json"):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = threading.Lock()
        self._jobs: dict[JobKey, _JobState] = self._load()

    # ------------------------------------------------------------------ I/O

    def _load(self) -> dict[JobKey, _JobState]:
        if not self.path.exists():
            return {}
        try:
            raw = json.loads(self.path.read_text())
        except (OSError, ValueError) as e:
            log.warning("baseline load failed (%s); starting fresh", e)
            return {}
        out: dict[JobKey, _JobState] = {}
        for key_str, val in raw.items():
            try:
                parts = key_str.split("|", 2)
                if len(parts) != 3:
                    continue
                key = (parts[0], parts[1], parts[2])
                out[key] = _JobState.from_dict(val)
            except Exception as e:  # noqa: BLE001
                log.warning("baseline skip bad entry %r: %s", key_str, e)
        return out

    def _save(self) -> None:
        # Caller must hold _lock.
        serialized = {
            f"{k[0]}|{k[1]}|{k[2]}": v.to_dict() for k, v in self._jobs.items()
        }
        tmp = self.path.with_suffix(".json.tmp")
        tmp.write_text(json.dumps(serialized, indent=2))
        os.replace(tmp, self.path)

    # ------------------------------------------------------------------ update

    def update(self, event: Event) -> None:
        key = self._key(event)
        with self._lock:
            job = self._jobs.setdefault(key, _JobState())
            # Track which runs we've seen.
            run_id = event.workflow.run_id
            if run_id:
                job.runs_seen.add(run_id)

            net_added = proc_added = file_added = 0
            if event.network and event.network.hostname:
                if self._add_domain(job, event.network.hostname):
                    net_added += 1
            if event.process and event.process.comm:
                if event.process.comm not in job.processes:
                    job.processes.add(event.process.comm)
                    proc_added += 1
            if event.file and event.file.path:
                if event.file.path not in job.file_writes:
                    job.file_writes.add(event.file.path)
                    file_added += 1

            # Maintain a sliding window of "new endpoints seen in the last N
            # runs" to flag unstable jobs (rapidly-churning surface).
            if net_added or proc_added or file_added:
                if not job.recent_new_endpoints:
                    job.recent_new_endpoints.append(0)
                job.recent_new_endpoints[-1] += net_added + proc_added + file_added

            self._save()

    def _add_domain(self, job: _JobState, hostname: str) -> bool:
        if hostname in job.domains:
            return False
        job.domains.add(hostname)
        # Wildcard expansion: store the parent suffix as a wildcard so future
        # subdomains under the same parent count as "known". E.g. seeing
        # `xyz.docker.io` stores `*.docker.io` too.
        parts = hostname.split(".")
        if len(parts) >= 2:
            job.domains.add("*." + ".".join(parts[1:]))
        return True

    # ------------------------------------------------------------------ queries

    def is_known_domain(self, key: JobKey, hostname: str) -> bool:
        job = self._jobs.get(key)
        if not job or not hostname:
            return False
        if hostname in job.domains:
            return True
        # Match against any wildcard we know about.
        parts = hostname.split(".")
        for i in range(1, len(parts)):
            wildcard = "*." + ".".join(parts[i:])
            if wildcard in job.domains:
                return True
        return False

    def is_known_process(self, key: JobKey, comm: str) -> bool:
        job = self._jobs.get(key)
        return bool(job and comm in job.processes)

    def is_known_file(self, key: JobKey, path: str) -> bool:
        job = self._jobs.get(key)
        return bool(job and path in job.file_writes)

    def status(self, key: JobKey) -> str:
        """Returns one of ``creating`` / ``stable`` / ``unstable``."""
        job = self._jobs.get(key)
        if not job:
            return "creating"
        if len(job.runs_seen) < STABLE_RUNS:
            return "creating"
        # If we've added more than N new endpoints in the last 3 windowed runs,
        # treat the baseline as unstable so rules can be lenient.
        last_three = job.recent_new_endpoints[-3:]
        if sum(last_three) > UNSTABLE_NEW_ENDPOINTS:
            return "unstable"
        return "stable"

    @staticmethod
    def _key(event: Event) -> JobKey:
        return (
            event.workflow.repository or "(unknown)",
            event.workflow.workflow or "(unknown)",
            event.workflow.job or "(unknown)",
        )

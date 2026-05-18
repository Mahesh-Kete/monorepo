"""Rule engine + per-event context.

The engine owns the cross-event state that individual rules need (recent
network events per PID, the PID → comm map, process ancestry). Rules query
this state via a ``Context`` object instead of reaching into module globals.
"""

from __future__ import annotations

import logging
from collections import defaultdict, deque
from datetime import datetime, timedelta, timezone
from typing import Deque

from .baseline import Baseline, JobKey
from .models import Detection, Event, ListedEvent
from .rules import ALL_RULES

log = logging.getLogger("citadel.detector.engine")

# Cross-event state retention. 5 minutes is enough for the
# possible_reverse_shell heuristic (which only looks 1s back) plus slack.
EVENT_WINDOW = timedelta(minutes=5)


class Context:
    """Per-engine state exposed to rules. Refreshed (mutated) on every event
    *before* the rules run, so rules see the new event in the proctree etc."""

    def __init__(self, baseline: Baseline):
        self.baseline = baseline
        # PID → comm. Accumulated from process events.
        self._pid_to_comm: dict[int, str] = {}
        # PID → parent PID. Accumulated from process events.
        self._pid_to_ppid: dict[int, int] = {}
        # PID → deque[datetime] of recent outbound network connect times.
        self._net_times: dict[int, Deque[datetime]] = defaultdict(deque)

    # -- mutators (engine calls these) ----------------------------------

    def absorb(self, e: Event) -> None:
        if e.type == "process" and e.process:
            self._pid_to_comm[e.process.pid] = e.process.comm
            self._pid_to_ppid[e.process.pid] = e.process.ppid
        elif e.type == "network" and e.network and e.process_chain:
            # The agent doesn't currently set network.pid in the event JSON
            # (it does on the agent side, but the field isn't surfaced). We
            # can't track per-pid network history without it — leave the
            # plumbing in place anyway for when it's wired up.
            pass

    def absorb_network_pid(self, pid: int, ts: datetime) -> None:
        """Called explicitly by the worker when it has a network event's pid."""
        dq = self._net_times[pid]
        dq.append(ts)
        cutoff = ts - EVENT_WINDOW
        while dq and dq[0] < cutoff:
            dq.popleft()

    # -- queries (rules use these) --------------------------------------

    def key_for(self, e: Event) -> JobKey:
        return (
            e.workflow.repository or "(unknown)",
            e.workflow.workflow or "(unknown)",
            e.workflow.job or "(unknown)",
        )

    def comm_for(self, pid: int) -> str:
        return self._pid_to_comm.get(pid, "")

    def ancestry_comms(self, pid: int, max_depth: int = 16) -> list[str]:
        """Walk the parent chain returning comm names from the named pid up."""
        out: list[str] = []
        cur = pid
        for _ in range(max_depth):
            comm = self._pid_to_comm.get(cur)
            if not comm:
                break
            out.append(comm)
            ppid = self._pid_to_ppid.get(cur)
            if not ppid or ppid == cur:
                break
            cur = ppid
        return out

    def recent_network_window(self, pid: int, within_seconds: int = 1) -> bool:
        dq = self._net_times.get(pid)
        if not dq:
            return False
        cutoff = datetime.now(timezone.utc) - timedelta(seconds=within_seconds)
        return any(ts >= cutoff for ts in dq)


class Engine:
    def __init__(self, baseline: Baseline):
        self.baseline = baseline
        self.ctx = Context(baseline)
        self.rules = ALL_RULES

    def evaluate(self, le: ListedEvent) -> list[Detection]:
        # Update cross-event state BEFORE rules run so the new event is visible.
        self.ctx.absorb(le.payload)
        if le.type == "network" and le.payload.network and le.payload.process_chain:
            # Best-effort: associate the network event with the process pid we
            # can derive. The agent doesn't put pid on network.payload — but
            # we can use the process_chain's head comm + recent process events
            # later. For now, no-op.
            pass

        detections: list[Detection] = []
        for rule in self.rules:
            try:
                got = rule.evaluate(le, self.ctx) or []
                detections.extend(got)
            except Exception as exc:  # noqa: BLE001
                log.warning("rule %s errored: %s", rule.name, exc)

        # Update baseline AFTER rules run — otherwise "new domain" never fires.
        try:
            self.baseline.update(le.payload)
        except Exception as exc:  # noqa: BLE001
            log.warning("baseline update failed: %s", exc)

        return detections

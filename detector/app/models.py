"""Pydantic models mirroring the Go Event schema from /agent/internal/events.

The Go side serializes timestamps as RFC 3339 strings and uses snake_case JSON
keys throughout. These models match that wire format exactly so we can
``Event.model_validate(payload)`` without an adapter.
"""

from __future__ import annotations

from datetime import datetime
from typing import Literal, Optional

from pydantic import BaseModel, ConfigDict, Field


class NetworkData(BaseModel):
    model_config = ConfigDict(extra="ignore")
    src_ip: str = ""
    dst_ip: str = ""
    dst_port: int = 0
    hostname: str = ""
    process: str = ""


class ProcessData(BaseModel):
    model_config = ConfigDict(extra="ignore")
    pid: int = 0
    ppid: int = 0
    uid: int = 0
    comm: str = ""
    filename: str = ""
    args: list[str] = Field(default_factory=list)


class FileData(BaseModel):
    model_config = ConfigDict(extra="ignore")
    path: str = ""
    flags: str = ""
    old_hash: str = ""
    new_hash: str = ""
    action: str = ""


class WorkflowMeta(BaseModel):
    model_config = ConfigDict(extra="ignore")
    repository: str = ""
    workflow: str = ""
    workflow_file: str = ""
    run_id: str = ""
    run_number: str = ""
    sha: str = ""
    ref: str = ""
    actor: str = ""
    event_name: str = ""
    job: str = ""
    step: str = ""


class Event(BaseModel):
    """The unified Event the agent emits. Network / Process / File are mutually
    exclusive (at most one is populated, discriminated by ``type``)."""
    model_config = ConfigDict(extra="ignore")
    id: str = ""
    type: str = ""
    timestamp: Optional[datetime] = None
    network: Optional[NetworkData] = None
    process: Optional[ProcessData] = None
    file: Optional[FileData] = None
    process_chain: list[str] = Field(default_factory=list)
    workflow: WorkflowMeta = Field(default_factory=WorkflowMeta)


class ListedEvent(BaseModel):
    """One row from ``GET /api/events`` — includes the backend's DB ids so we
    can reference them when posting detections back."""
    model_config = ConfigDict(extra="ignore")
    id: int
    run_id: int
    type: str
    timestamp: datetime
    step: str = ""
    payload: Event


Severity = Literal["info", "low", "medium", "high", "critical"]


class Detection(BaseModel):
    """Wire shape for ``POST /api/detections`` (and what every rule returns)."""
    model_config = ConfigDict(extra="ignore")
    run_id: int
    event_id: Optional[int] = None
    rule_name: str
    severity: Severity
    message: str = ""

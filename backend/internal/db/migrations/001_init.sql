-- 001_init.sql — Citadel backend schema (idempotent: safe to re-run).
--
-- Tables:
--   runs        one row per (repository, run_id) — the workflow execution
--   events      every probe event, with full JSON payload preserved
--   detections  findings from the Python detector
--   policies    audit/block policies + allowlists + per-rule actions

CREATE TABLE IF NOT EXISTS runs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repository    TEXT NOT NULL,
    workflow      TEXT,
    run_id        TEXT NOT NULL,
    run_number    TEXT,
    sha           TEXT,
    ref           TEXT,
    actor         TEXT,
    started_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    policy_mode   TEXT NOT NULL DEFAULT 'audit',
    status        TEXT NOT NULL DEFAULT 'in_progress',
    UNIQUE(repository, run_id)
);

CREATE TABLE IF NOT EXISTS events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id        INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    type          TEXT NOT NULL,
    timestamp     TIMESTAMP NOT NULL,
    payload       TEXT NOT NULL,            -- full Event JSON from the agent
    process_chain TEXT,                      -- JSON array
    step          TEXT
);
CREATE INDEX IF NOT EXISTS idx_events_run_id    ON events(run_id);
CREATE INDEX IF NOT EXISTS idx_events_type      ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

CREATE TABLE IF NOT EXISTS detections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    event_id    INTEGER REFERENCES events(id) ON DELETE SET NULL,
    rule_name   TEXT NOT NULL,
    severity    TEXT NOT NULL CHECK(severity IN ('info','low','medium','high','critical')),
    message     TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_detections_run_id     ON detections(run_id);
CREATE INDEX IF NOT EXISTS idx_detections_created_at ON detections(created_at);

CREATE TABLE IF NOT EXISTS policies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    scope_repo      TEXT,
    scope_workflow  TEXT,
    mode            TEXT NOT NULL CHECK(mode IN ('audit','block')),
    allowlist       TEXT,                          -- JSON array of domains/IPs
    detection_rules TEXT,                          -- JSON: rule_name -> action
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

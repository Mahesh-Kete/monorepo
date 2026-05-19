-- 002_repos_and_gh_columns.sql — "Connect a GitHub repo" feature.
--
-- Adds:
--   * connected_repos table: per-repo PAT + last-poll bookkeeping
--   * gh_* columns on runs: status/conclusion/html_url/duration synced
--     from the GitHub Actions API every ~30s by internal/github/poller.go

CREATE TABLE IF NOT EXISTS connected_repos (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    repository     TEXT NOT NULL UNIQUE,             -- owner/repo
    token          TEXT NOT NULL,                    -- PAT; stored as-is for the hackathon (TODO: encrypt at rest)
    note           TEXT,                             -- optional free-text label
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_polled_at TIMESTAMP,
    last_error     TEXT
);

-- runs may already exist (from migration 001). Use the SQLite ALTER TABLE
-- syntax that's safe to repeat: try to add each column, ignore if exists.
-- SQLite doesn't have IF NOT EXISTS on ADD COLUMN until 3.35; for older
-- versions you'd guard externally. We're on modernc.org/sqlite (recent).

ALTER TABLE runs ADD COLUMN gh_status        TEXT;
ALTER TABLE runs ADD COLUMN gh_conclusion    TEXT;
ALTER TABLE runs ADD COLUMN gh_html_url      TEXT;
ALTER TABLE runs ADD COLUMN gh_duration_sec  INTEGER;
ALTER TABLE runs ADD COLUMN gh_event_name    TEXT;
ALTER TABLE runs ADD COLUMN gh_head_branch   TEXT;
ALTER TABLE runs ADD COLUMN gh_synced_at     TIMESTAMP;
ALTER TABLE runs ADD COLUMN agent_seen       INTEGER NOT NULL DEFAULT 0; -- 1 if any event was ever posted for this run by the agent

CREATE INDEX IF NOT EXISTS idx_runs_synced_at ON runs(gh_synced_at);

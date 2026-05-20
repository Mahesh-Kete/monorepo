-- 003_optional_tokens_and_action_logs.sql
--
-- Make GitHub PATs optional and store GitHub Actions annotations/log lines
-- explicitly so the dashboard can render the same signal shown in Actions.

DROP TABLE IF EXISTS connected_repos_new;

CREATE TABLE connected_repos_new (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    repository     TEXT NOT NULL UNIQUE,             -- owner/repo
    token          TEXT,                             -- optional PAT; NULL when repo uses manual/public-key flow
    note           TEXT,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_polled_at TIMESTAMP,
    last_error     TEXT
);

INSERT INTO connected_repos_new (
    id, repository, token, note, created_at, last_polled_at, last_error
)
SELECT id, repository, NULLIF(token, ''), note, created_at, last_polled_at, last_error
FROM connected_repos;

DROP TABLE connected_repos;
ALTER TABLE connected_repos_new RENAME TO connected_repos;

CREATE TABLE IF NOT EXISTS github_action_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    job_name    TEXT,
    step        TEXT,
    level       TEXT NOT NULL DEFAULT 'notice',
    rule_name   TEXT,
    message     TEXT NOT NULL,
    html_url    TEXT,
    line        INTEGER,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_github_action_logs_run_id
    ON github_action_logs(run_id);

CREATE INDEX IF NOT EXISTS idx_github_action_logs_level
    ON github_action_logs(level);

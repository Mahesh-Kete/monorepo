// Background poller: every interval, for each connected repo, fetch the
// recent workflow runs from GitHub and upsert them into the `runs` table.
//
// One goroutine handles all repos sequentially. If one repo is slow / rate-
// limited / token-invalid, only that one falls behind; others continue.
package github

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

const (
	pollInterval = 30 * time.Second
	perRepoLimit = 20
)

type Poller struct {
	DB     *sql.DB
	Logger *slog.Logger
	gh     *Client
}

func NewPoller(db *sql.DB, logger *slog.Logger) *Poller {
	return &Poller{
		DB:     db,
		Logger: logger,
		gh:     New(),
	}
}

// Start launches the poller goroutine. Cancel ctx to stop.
func (p *Poller) Start(ctx context.Context) {
	go p.loop(ctx)
}

func (p *Poller) loop(ctx context.Context) {
	p.Logger.Info("github poller started", "interval", pollInterval)
	for {
		// Run a sweep immediately, then sleep.
		p.sweep(ctx)
		select {
		case <-ctx.Done():
			p.Logger.Info("github poller stopped")
			return
		case <-time.After(pollInterval):
		}
	}
}

// connectedRepo is the row shape from connected_repos.
type connectedRepo struct {
	ID         int64
	Repository string
	Token      string
}

func (p *Poller) sweep(ctx context.Context) {
	rows, err := p.DB.QueryContext(ctx, `SELECT id, repository, token FROM connected_repos`)
	if err != nil {
		p.Logger.Warn("list connected repos", "err", err)
		return
	}
	defer func() { _ = rows.Close() }()

	var repos []connectedRepo
	for rows.Next() {
		var r connectedRepo
		if err := rows.Scan(&r.ID, &r.Repository, &r.Token); err == nil {
			repos = append(repos, r)
		}
	}

	for _, r := range repos {
		if err := p.syncRepo(ctx, r); err != nil {
			p.Logger.Warn("sync repo", "repo", r.Repository, "err", err)
			_, _ = p.DB.ExecContext(ctx, `
				UPDATE connected_repos
				SET last_error=?, last_polled_at=CURRENT_TIMESTAMP
				WHERE id=?`, err.Error(), r.ID)
			continue
		}
		_, _ = p.DB.ExecContext(ctx, `
			UPDATE connected_repos
			SET last_error=NULL, last_polled_at=CURRENT_TIMESTAMP
			WHERE id=?`, r.ID)
	}
}

func (p *Poller) syncRepo(ctx context.Context, r connectedRepo) error {
	runs, err := p.gh.ListRecentRuns(ctx, r.Token, r.Repository, perRepoLimit)
	if err != nil {
		return err
	}
	for _, wr := range runs {
		if err := p.upsertRun(ctx, r.Repository, wr); err != nil {
			p.Logger.Warn("upsert gh run", "repo", r.Repository, "run_id", wr.ID, "err", err)
		}
	}
	p.Logger.Info("github poll", "repo", r.Repository, "runs", len(runs))
	return nil
}

// upsertRun creates or updates a runs row from a GitHub workflow run.
// We never clobber agent-set fields with empty values, and we preserve
// `agent_seen=1` once it's been flipped.
func (p *Poller) upsertRun(ctx context.Context, repo string, wr WorkflowRun) error {
	runIDStr := strconv.FormatInt(wr.ID, 10)
	runNumStr := strconv.FormatInt(wr.RunNumber, 10)
	duration := wr.DurationSec()

	// Try update first; if no row, insert.
	res, err := p.DB.ExecContext(ctx, `
		UPDATE runs SET
			workflow         = COALESCE(NULLIF(?, ''), workflow),
			run_number       = COALESCE(NULLIF(?, ''), run_number),
			sha              = COALESCE(NULLIF(?, ''), sha),
			ref              = COALESCE(NULLIF(?, ''), ref),
			actor            = COALESCE(NULLIF(?, ''), actor),
			started_at       = COALESCE(?, started_at),
			gh_status        = ?,
			gh_conclusion    = NULLIF(?, ''),
			gh_html_url      = ?,
			gh_duration_sec  = CASE WHEN ? > 0 THEN ? ELSE gh_duration_sec END,
			gh_event_name    = NULLIF(?, ''),
			gh_head_branch   = NULLIF(?, ''),
			gh_synced_at     = CURRENT_TIMESTAMP
		WHERE repository = ? AND run_id = ?`,
		wr.Name, runNumStr, wr.HeadSHA, "refs/heads/"+wr.HeadBranch, wr.Actor.Login,
		ifZeroTime(wr.RunStartedAt),
		wr.Status, wr.Conclusion, wr.HTMLURL,
		duration, duration,
		wr.Event, wr.HeadBranch,
		repo, runIDStr,
	)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}

	_, err = p.DB.ExecContext(ctx, `
		INSERT INTO runs
			(repository, workflow, run_id, run_number, sha, ref, actor, started_at, policy_mode, status,
			 gh_status, gh_conclusion, gh_html_url, gh_duration_sec, gh_event_name, gh_head_branch,
			 gh_synced_at, agent_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP),
		        'audit', 'in_progress',
		        ?, NULLIF(?, ''), ?, CASE WHEN ? > 0 THEN ? ELSE NULL END, NULLIF(?, ''), NULLIF(?, ''),
		        CURRENT_TIMESTAMP, 0)`,
		repo, wr.Name, runIDStr, runNumStr, wr.HeadSHA, "refs/heads/"+wr.HeadBranch, wr.Actor.Login,
		ifZeroTime(wr.RunStartedAt),
		wr.Status, wr.Conclusion, wr.HTMLURL,
		duration, duration,
		wr.Event, wr.HeadBranch,
	)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	return nil
}

func ifZeroTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

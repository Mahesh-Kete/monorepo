package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

// detectionRow is the shape returned by GET /api/detections and embedded
// inside runDetail.
type detectionRow struct {
	ID        int64     `json:"id"`
	RunID     int64     `json:"run_id"`
	EventID   *int64    `json:"event_id,omitempty"`
	RuleName  string    `json:"rule_name"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type postDetectionRequest struct {
	RunID    int64  `json:"run_id"`
	EventID  *int64 `json:"event_id,omitempty"`
	RuleName string `json:"rule_name"`
	Severity string `json:"severity"`
	Message  string `json:"message,omitempty"`
}

// postDetectionByGitHubRequest is what the GitHub Actions composite action
// posts — it doesn't know Citadel's internal run id, only the GitHub-side
// repository + run_id pair. We resolve those to the row id.
type postDetectionByGitHubRequest struct {
	Repository  string `json:"repository"`
	GitHubRunID string `json:"github_run_id"`
	RuleName    string `json:"rule_name"`
	Severity    string `json:"severity"`
	Message     string `json:"message,omitempty"`
}

var validSeverity = map[string]bool{
	"info":     true,
	"low":      true,
	"medium":   true,
	"high":     true,
	"critical": true,
}

// ---------------------------------------------------------------------------
// POST /api/detections  (the Python detector writes findings here)
// ---------------------------------------------------------------------------

func (a *API) handlePostDetection(w http.ResponseWriter, r *http.Request) {
	var req postDetectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.RunID <= 0 || req.RuleName == "" || req.Severity == "" {
		writeError(w, http.StatusBadRequest, "run_id, rule_name, severity are required")
		return
	}
	if !validSeverity[req.Severity] {
		writeError(w, http.StatusBadRequest, "severity must be one of info|low|medium|high|critical")
		return
	}

	var eventID sql.NullInt64
	if req.EventID != nil {
		eventID.Valid = true
		eventID.Int64 = *req.EventID
	}

	res, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO detections (run_id, event_id, rule_name, severity, message)
		VALUES (?, ?, ?, ?, ?)`,
		req.RunID, eventID, req.RuleName, req.Severity, req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert detection: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// ---------------------------------------------------------------------------
// POST /api/detections/by-github-run
//
// The Citadel composite action calls this after scanning the workflow YAML
// for imposter-commit references. It knows the GitHub-side identifiers
// (repository + run_id) but not Citadel's internal row id, so we resolve
// here. Idempotent-ish: duplicate calls just insert multiple detection rows.
// ---------------------------------------------------------------------------

func (a *API) handlePostDetectionByGitHub(w http.ResponseWriter, r *http.Request) {
	var req postDetectionByGitHubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Repository == "" || req.GitHubRunID == "" || req.RuleName == "" || req.Severity == "" {
		writeError(w, http.StatusBadRequest,
			"repository, github_run_id, rule_name, severity are required")
		return
	}
	if !validSeverity[req.Severity] {
		writeError(w, http.StatusBadRequest, "severity must be info|low|medium|high|critical")
		return
	}

	var runID int64
	err := a.DB.QueryRowContext(r.Context(),
		`SELECT id FROM runs WHERE repository = ? AND run_id = ?`,
		req.Repository, req.GitHubRunID).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		// Be lenient: insert a placeholder run row so the detection isn't
		// lost. The agent's first event will then backfill workflow / sha
		// / actor on the existing row via upsertRun's UPDATE path.
		res, errIns := a.DB.ExecContext(r.Context(), `
			INSERT INTO runs (repository, run_id, policy_mode)
			VALUES (?, ?, 'block')`,
			req.Repository, req.GitHubRunID)
		if errIns != nil {
			writeError(w, http.StatusInternalServerError, "create placeholder run: "+errIns.Error())
			return
		}
		runID, _ = res.LastInsertId()
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "query run: "+err.Error())
		return
	}

	res, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO detections (run_id, rule_name, severity, message)
		VALUES (?, ?, ?, ?)`,
		runID, req.RuleName, req.Severity, req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert detection: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     id,
		"run_id": runID,
	})
}

// ---------------------------------------------------------------------------
// GET /api/detections?since=ISO_TIME  (detector polls this on restart)
// ---------------------------------------------------------------------------

func (a *API) handleListDetections(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, run_id, event_id, rule_name, severity, COALESCE(message, ''), created_at
	      FROM detections`
	args := []any{}

	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339Nano, since); err == nil {
			q += ` WHERE created_at > ?`
			args = append(args, t)
		} else if t, err := time.Parse(time.RFC3339, since); err == nil {
			q += ` WHERE created_at > ?`
			args = append(args, t)
		}
	}
	q += ` ORDER BY created_at DESC LIMIT ?`

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}
	args = append(args, limit)

	rows, err := a.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	out := make([]detectionRow, 0)
	for rows.Next() {
		var d detectionRow
		var evtID sql.NullInt64
		if err := rows.Scan(&d.ID, &d.RunID, &evtID, &d.RuleName, &d.Severity, &d.Message, &d.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		if evtID.Valid {
			d.EventID = &evtID.Int64
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// listDetectionsForRun is called by handleGetRun to embed detections inside
// the run detail response.
func (a *API) listDetectionsForRun(ctx context.Context, runID int64) ([]detectionRow, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT id, run_id, event_id, rule_name, severity, COALESCE(message, ''), created_at
		FROM detections WHERE run_id = ?
		ORDER BY created_at DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]detectionRow, 0)
	for rows.Next() {
		var d detectionRow
		var evtID sql.NullInt64
		if err := rows.Scan(&d.ID, &d.RunID, &evtID, &d.RuleName, &d.Severity, &d.Message, &d.CreatedAt); err != nil {
			return nil, err
		}
		if evtID.Valid {
			d.EventID = &evtID.Int64
		}
		out = append(out, d)
	}
	return out, nil
}

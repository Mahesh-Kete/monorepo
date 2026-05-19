package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// runSummary is the shape returned by GET /api/runs.
//
// Fields prefixed gh_* are populated from the GitHub Actions API by
// /internal/github/poller.go (Phase 11 / "Connect repo" feature). AgentSeen
// is true once at least one event has been ingested from the Citadel agent
// for this run — used by the dashboard to render a "Citadel coverage" badge.
type runSummary struct {
	ID           int64          `json:"id"`
	Repository   string         `json:"repository"`
	Workflow     string         `json:"workflow,omitempty"`
	RunID        string         `json:"run_id"`
	RunNumber    string         `json:"run_number,omitempty"`
	SHA          string         `json:"sha,omitempty"`
	Ref          string         `json:"ref,omitempty"`
	Actor        string         `json:"actor,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	PolicyMode   string         `json:"policy_mode"`
	Status       string         `json:"status"`
	EventCounts  map[string]int `json:"event_counts"`
	DetectionCnt int            `json:"detection_count"`
	SeverityMax  string         `json:"severity_max,omitempty"`

	// GitHub Actions metadata (Phase 11 "Connect repo")
	GHStatus       string  `json:"gh_status,omitempty"`     // queued | in_progress | completed
	GHConclusion   string  `json:"gh_conclusion,omitempty"` // success | failure | cancelled | …
	GHHTMLURL      string  `json:"gh_html_url,omitempty"`
	GHDurationSec  int     `json:"gh_duration_sec,omitempty"`
	GHEventName    string  `json:"gh_event_name,omitempty"`
	GHHeadBranch   string  `json:"gh_head_branch,omitempty"`
	GHSyncedAt     *time.Time `json:"gh_synced_at,omitempty"`
	AgentSeen      bool    `json:"agent_seen"`
}

// runDetail is the shape returned by GET /api/runs/:id.
type runDetail struct {
	Run        runSummary        `json:"run"`
	Events     []json.RawMessage `json:"events"`
	Detections []detectionRow    `json:"detections"`
}

// ---------------------------------------------------------------------------
// GET /api/runs
// ---------------------------------------------------------------------------

func (a *API) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT
			r.id, r.repository, COALESCE(r.workflow, ''), r.run_id,
			COALESCE(r.run_number, ''), COALESCE(r.sha, ''), COALESCE(r.ref, ''),
			COALESCE(r.actor, ''), r.started_at, r.policy_mode, r.status,
			COALESCE(r.gh_status, ''), COALESCE(r.gh_conclusion, ''),
			COALESCE(r.gh_html_url, ''), COALESCE(r.gh_duration_sec, 0),
			COALESCE(r.gh_event_name, ''), COALESCE(r.gh_head_branch, ''),
			r.gh_synced_at, COALESCE(r.agent_seen, 0),
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'network') AS net_count,
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'process') AS proc_count,
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'file') AS file_count,
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'file_tamper') AS tamper_count,
			(SELECT COUNT(*) FROM detections WHERE run_id = r.id) AS det_count,
			COALESCE((SELECT severity FROM detections WHERE run_id = r.id
				ORDER BY CASE severity
					WHEN 'critical' THEN 5 WHEN 'high' THEN 4
					WHEN 'medium'   THEN 3 WHEN 'low'  THEN 2
					WHEN 'info'     THEN 1 ELSE 0 END DESC
				LIMIT 1), '') AS severity_max
		FROM runs r
		ORDER BY r.started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query runs: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	out := make([]runSummary, 0)
	for rows.Next() {
		var s runSummary
		var net, proc, file, tamper int
		var synced sql.NullTime
		var agentSeen int
		if err := rows.Scan(
			&s.ID, &s.Repository, &s.Workflow, &s.RunID,
			&s.RunNumber, &s.SHA, &s.Ref, &s.Actor,
			&s.StartedAt, &s.PolicyMode, &s.Status,
			&s.GHStatus, &s.GHConclusion, &s.GHHTMLURL, &s.GHDurationSec,
			&s.GHEventName, &s.GHHeadBranch, &synced, &agentSeen,
			&net, &proc, &file, &tamper,
			&s.DetectionCnt, &s.SeverityMax,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		if synced.Valid {
			t := synced.Time
			s.GHSyncedAt = &t
		}
		s.AgentSeen = agentSeen != 0
		s.EventCounts = map[string]int{
			"network":     net,
			"process":     proc,
			"file":        file,
			"file_tamper": tamper,
		}
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// GET /api/runs/{id}
// ---------------------------------------------------------------------------

func (a *API) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	// Header row.
	var s runSummary
	var net, proc, file, tamper int
	var synced sql.NullTime
	var agentSeen int
	err = a.DB.QueryRowContext(r.Context(), `
		SELECT
			r.id, r.repository, COALESCE(r.workflow, ''), r.run_id,
			COALESCE(r.run_number, ''), COALESCE(r.sha, ''), COALESCE(r.ref, ''),
			COALESCE(r.actor, ''), r.started_at, r.policy_mode, r.status,
			COALESCE(r.gh_status, ''), COALESCE(r.gh_conclusion, ''),
			COALESCE(r.gh_html_url, ''), COALESCE(r.gh_duration_sec, 0),
			COALESCE(r.gh_event_name, ''), COALESCE(r.gh_head_branch, ''),
			r.gh_synced_at, COALESCE(r.agent_seen, 0),
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'network'),
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'process'),
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'file'),
			(SELECT COUNT(*) FROM events WHERE run_id = r.id AND type = 'file_tamper'),
			(SELECT COUNT(*) FROM detections WHERE run_id = r.id),
			COALESCE((SELECT severity FROM detections WHERE run_id = r.id
				ORDER BY CASE severity
					WHEN 'critical' THEN 5 WHEN 'high' THEN 4
					WHEN 'medium'   THEN 3 WHEN 'low'  THEN 2
					WHEN 'info'     THEN 1 ELSE 0 END DESC
				LIMIT 1), '')
		FROM runs r WHERE r.id = ?`, id).Scan(
		&s.ID, &s.Repository, &s.Workflow, &s.RunID,
		&s.RunNumber, &s.SHA, &s.Ref, &s.Actor,
		&s.StartedAt, &s.PolicyMode, &s.Status,
		&s.GHStatus, &s.GHConclusion, &s.GHHTMLURL, &s.GHDurationSec,
		&s.GHEventName, &s.GHHeadBranch, &synced, &agentSeen,
		&net, &proc, &file, &tamper,
		&s.DetectionCnt, &s.SeverityMax,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query run: "+err.Error())
		return
	}
	if synced.Valid {
		t := synced.Time
		s.GHSyncedAt = &t
	}
	s.AgentSeen = agentSeen != 0
	s.EventCounts = map[string]int{
		"network": net, "process": proc, "file": file, "file_tamper": tamper,
	}

	// Events (optionally filtered by ?type=).
	typeFilter := r.URL.Query().Get("type")
	q := `SELECT payload FROM events WHERE run_id = ?`
	args := []any{id}
	if typeFilter != "" {
		q += ` AND type = ?`
		args = append(args, typeFilter)
	}
	q += ` ORDER BY timestamp ASC, id ASC LIMIT 5000`

	rows, err := a.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query events: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	events := []json.RawMessage{} // never return nil — UI calls .filter() on it
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			writeError(w, http.StatusInternalServerError, "scan event: "+err.Error())
			return
		}
		events = append(events, json.RawMessage(payload))
	}

	// Detections.
	detections, err := a.listDetectionsForRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query detections: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, runDetail{
		Run:        s,
		Events:     events,
		Detections: detections,
	})
}

// ---------------------------------------------------------------------------
// DELETE /api/runs/{id}
// ---------------------------------------------------------------------------
//
// Removes a single run. The events and detections tables both declare
// `run_id ... ON DELETE CASCADE`, and foreign keys are enabled in the DSN
// (see internal/db/db.go), so dependent rows go with it.

func (a *API) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}
	res, err := a.DB.ExecContext(r.Context(), `DELETE FROM runs WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// ---------------------------------------------------------------------------
// DELETE /api/runs/unknown
// ---------------------------------------------------------------------------
//
// Bulk-cleanup endpoint for placeholder runs created when the agent posted
// events without GitHub context (repository = "(unknown)" — see
// events.go:upsertRun). Useful when the demo runner was started outside a
// real workflow.

func (a *API) handleDeleteUnknownRuns(w http.ResponseWriter, r *http.Request) {
	res, err := a.DB.ExecContext(r.Context(),
		`DELETE FROM runs WHERE repository = '(unknown)'`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// ---------------------------------------------------------------------------
// GET /api/runs/{id}/process-tree
// ---------------------------------------------------------------------------

type processNode struct {
	PID      uint32         `json:"pid"`
	PPID     uint32         `json:"ppid"`
	Comm     string         `json:"comm"`
	Filename string         `json:"filename,omitempty"`
	Args     []string       `json:"args,omitempty"`
	Children []*processNode `json:"children,omitempty"`
}

func (a *API) handleGetProcessTree(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	rows, err := a.DB.QueryContext(r.Context(),
		`SELECT payload FROM events WHERE run_id = ? AND type = 'process'`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	nodes := map[uint32]*processNode{}
	order := []uint32{} // for stable tree-root output
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			continue
		}
		var ev struct {
			Process *struct {
				PID      uint32   `json:"pid"`
				PPID     uint32   `json:"ppid"`
				Comm     string   `json:"comm"`
				Filename string   `json:"filename"`
				Args     []string `json:"args"`
			} `json:"process"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil || ev.Process == nil {
			continue
		}
		p := ev.Process
		if _, exists := nodes[p.PID]; !exists {
			order = append(order, p.PID)
		}
		nodes[p.PID] = &processNode{
			PID:      p.PID,
			PPID:     p.PPID,
			Comm:     p.Comm,
			Filename: p.Filename,
			Args:     p.Args,
		}
	}

	// Link children to parents. Nodes whose parent isn't in the map are
	// treated as roots.
	var roots []*processNode
	for _, pid := range order {
		n := nodes[pid]
		if parent, ok := nodes[n.PPID]; ok && n.PPID != n.PID {
			parent.Children = append(parent.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	writeJSON(w, http.StatusOK, roots)
}

// ---------------------------------------------------------------------------
// GET /api/runs/{id}/baseline-domains
// ---------------------------------------------------------------------------

func (a *API) handleGetBaselineDomains(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT DISTINCT json_extract(payload, '$.network.hostname') AS hostname
		FROM events
		WHERE run_id = ? AND type = 'network'
		  AND json_extract(payload, '$.network.hostname') IS NOT NULL
		  AND json_extract(payload, '$.network.hostname') != ''`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("query: %v", err))
		return
	}
	defer func() { _ = rows.Close() }()

	out := []string{}
	for rows.Next() {
		var h sql.NullString
		if err := rows.Scan(&h); err == nil && h.Valid {
			out = append(out, h.String)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

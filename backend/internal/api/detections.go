package api

import (
	"context"
	"database/sql"
	"encoding/json"
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

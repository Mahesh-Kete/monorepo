package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Each event in the batch is unmarshaled once for indexing (the typed
// fields below), and the original JSON bytes are stored verbatim as the
// payload. json.RawMessage preserves the original encoding.
type incomingEvent struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Timestamp    time.Time       `json:"timestamp"`
	Workflow     incomingWorkflow `json:"workflow"`
	ProcessChain []string        `json:"process_chain"`
}

type incomingWorkflow struct {
	Repository string `json:"repository"`
	Workflow   string `json:"workflow"`
	RunID      string `json:"run_id"`
	RunNumber  string `json:"run_number"`
	SHA        string `json:"sha"`
	Ref        string `json:"ref"`
	Actor      string `json:"actor"`
	EventName  string `json:"event_name"`
	Job        string `json:"job"`
	Step       string `json:"step"`
}

type postEventsRequest struct {
	Events []json.RawMessage `json:"events"`
}

const maxBatchSize = 1000

// handlePostEvents ingests a batch of agent events. The request body is the
// agent's batch shape: {"events": [Event, Event, ...]}.
//
// For each event we upsert the run keyed on (repository, run_id) — local
// events without GitHub context use the placeholder "(unknown)/(local)".
// All inserts happen in one transaction so a partial failure doesn't leave
// orphan rows.
func (a *API) handlePostEvents(w http.ResponseWriter, r *http.Request) {
	var req postEventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Events) == 0 {
		writeJSON(w, http.StatusOK, map[string]int{"accepted": 0})
		return
	}
	if len(req.Events) > maxBatchSize {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("batch too large: max %d events", maxBatchSize))
		return
	}

	ctx := r.Context()
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "begin tx: "+err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }() // no-op if Commit succeeded

	accepted := 0
	for i, raw := range req.Events {
		var typed incomingEvent
		if err := json.Unmarshal(raw, &typed); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event[%d] invalid: %v", i, err))
			return
		}
		runID, err := upsertRun(ctx, tx, typed.Workflow)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("event[%d] upsert run: %v", i, err))
			return
		}
		if err := insertEvent(ctx, tx, runID, typed, raw); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("event[%d] insert: %v", i, err))
			return
		}
		accepted++
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "commit: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"accepted": accepted})
}

// upsertRun finds-or-creates the runs row for this workflow context and
// returns its primary key. Empty repository / run_id collapse to a single
// placeholder run so local agent output is still navigable in the UI.
func upsertRun(ctx context.Context, tx *sql.Tx, wf incomingWorkflow) (int64, error) {
	repo := wf.Repository
	if repo == "" {
		repo = "(unknown)"
	}
	runID := wf.RunID
	if runID == "" {
		runID = "(local)"
	}

	// Try to find existing.
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM runs WHERE repository=? AND run_id=?`,
		repo, runID).Scan(&id)
	if err == nil {
		// Backfill any fields we now know that were unknown before.
		_, _ = tx.ExecContext(ctx, `
			UPDATE runs SET
				workflow    = COALESCE(NULLIF(?, ''), workflow),
				run_number  = COALESCE(NULLIF(?, ''), run_number),
				sha         = COALESCE(NULLIF(?, ''), sha),
				ref         = COALESCE(NULLIF(?, ''), ref),
				actor       = COALESCE(NULLIF(?, ''), actor)
			WHERE id = ?`,
			wf.Workflow, wf.RunNumber, wf.SHA, wf.Ref, wf.Actor, id)
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	// Insert new run.
	res, err := tx.ExecContext(ctx, `
		INSERT INTO runs (repository, workflow, run_id, run_number, sha, ref, actor)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		repo, wf.Workflow, runID, wf.RunNumber, wf.SHA, wf.Ref, wf.Actor)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ---------------------------------------------------------------------------
// GET /api/events  (detector polls this every 2s)
// ---------------------------------------------------------------------------

type listedEvent struct {
	ID        int64           `json:"id"`         // DB id (used by detector for ?after_id=)
	RunID     int64           `json:"run_id"`     // DB id of the parent run
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Step      string          `json:"step,omitempty"`
	Payload   json.RawMessage `json:"payload"`    // full Event JSON as the agent emitted it
}

func (a *API) handleListEvents(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, run_id, type, timestamp, COALESCE(step, ''), payload
	      FROM events`
	args := []any{}
	conds := []string{}

	if afterIDStr := r.URL.Query().Get("after_id"); afterIDStr != "" {
		var afterID int64
		if _, err := fmt.Sscan(afterIDStr, &afterID); err == nil {
			conds = append(conds, "id > ?")
			args = append(args, afterID)
		}
	}
	if since := r.URL.Query().Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339Nano, since)
		if err != nil {
			t, err = time.Parse(time.RFC3339, since)
		}
		if err == nil {
			conds = append(conds, "timestamp > ?")
			args = append(args, t)
		}
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY id ASC LIMIT ?"

	limit := 500
	if l := r.URL.Query().Get("limit"); l != "" {
		var v int
		if _, err := fmt.Sscan(l, &v); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}
	args = append(args, limit)

	rows, err := a.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query events: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	out := make([]listedEvent, 0)
	for rows.Next() {
		var e listedEvent
		var payload string
		if err := rows.Scan(&e.ID, &e.RunID, &e.Type, &e.Timestamp, &e.Step, &payload); err != nil {
			writeError(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		e.Payload = json.RawMessage(payload)
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

func insertEvent(ctx context.Context, tx *sql.Tx, runID int64, e incomingEvent, raw json.RawMessage) error {
	processChain := ""
	if len(e.ProcessChain) > 0 {
		b, _ := json.Marshal(e.ProcessChain)
		processChain = string(b)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO events (run_id, type, timestamp, payload, process_chain, step)
		VALUES (?, ?, ?, ?, ?, ?)`,
		runID, e.Type, e.Timestamp, string(raw), processChain, e.Workflow.Step)
	return err
}

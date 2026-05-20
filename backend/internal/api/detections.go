package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// detectionRow is the shape returned by GET /api/detections and embedded
// inside runDetail.
type detectionRow struct {
	ID        int64             `json:"id"`
	RunID     int64             `json:"run_id"`
	EventID   *int64            `json:"event_id,omitempty"`
	RuleName  string            `json:"rule_name"`
	Severity  string            `json:"severity"`
	Message   string            `json:"message,omitempty"`
	Title     string            `json:"title,omitempty"`
	Summary   string            `json:"summary,omitempty"`
	Details   []detectionDetail `json:"details,omitempty"`
	Source    *detectionSource  `json:"source,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type detectionDetail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type detectionSource struct {
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	URL  string `json:"url,omitempty"`
	Code string `json:"code,omitempty"`
}

type postDetectionRequest struct {
	RunID    int64             `json:"run_id"`
	EventID  *int64            `json:"event_id,omitempty"`
	RuleName string            `json:"rule_name"`
	Severity string            `json:"severity"`
	Message  string            `json:"message,omitempty"`
	Title    string            `json:"title,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	Details  []detectionDetail `json:"details,omitempty"`
	Source   *detectionSource  `json:"source,omitempty"`
}

// postDetectionByGitHubRequest is what the GitHub Actions composite action
// posts — it doesn't know Citadel's internal run id, only the GitHub-side
// repository + run_id pair. We resolve those to the row id.
type postDetectionByGitHubRequest struct {
	Repository  string            `json:"repository"`
	GitHubRunID string            `json:"github_run_id"`
	RuleName    string            `json:"rule_name"`
	Severity    string            `json:"severity"`
	Message     string            `json:"message,omitempty"`
	PolicyMode  string            `json:"policy_mode,omitempty"`
	Title       string            `json:"title,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	Details     []detectionDetail `json:"details,omitempty"`
	Source      *detectionSource  `json:"source,omitempty"`
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

	title, summary, detailsJSON, sourceJSON := structuredDetectionValues(
		req.RuleName, req.Message, req.Title, req.Summary, req.Details, req.Source)
	res, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO detections (run_id, event_id, rule_name, severity, message, title, summary, details, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.RunID, eventID, req.RuleName, req.Severity, req.Message, title, summary, detailsJSON, sourceJSON)
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
	if req.PolicyMode != "" && req.PolicyMode != "audit" && req.PolicyMode != "block" {
		writeError(w, http.StatusBadRequest, "policy_mode must be audit or block")
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
	if req.PolicyMode != "" {
		_, _ = a.DB.ExecContext(r.Context(),
			`UPDATE runs SET policy_mode = ? WHERE id = ?`, req.PolicyMode, runID)
	}

	title, summary, detailsJSON, sourceJSON := structuredDetectionValues(
		req.RuleName, req.Message, req.Title, req.Summary, req.Details, req.Source)
	res, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO detections (run_id, rule_name, severity, message, title, summary, details, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, req.RuleName, req.Severity, req.Message, title, summary, detailsJSON, sourceJSON)
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
	q := `SELECT id, run_id, event_id, rule_name, severity, COALESCE(message, ''),
	             COALESCE(title, ''), COALESCE(summary, ''), COALESCE(details, ''), COALESCE(source, ''),
	             created_at
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
		var detailsJSON, sourceJSON string
		if err := rows.Scan(
			&d.ID, &d.RunID, &evtID, &d.RuleName, &d.Severity, &d.Message,
			&d.Title, &d.Summary, &detailsJSON, &sourceJSON, &d.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		if evtID.Valid {
			d.EventID = &evtID.Int64
		}
		hydrateDetectionPresentation(&d, detailsJSON, sourceJSON)
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// listDetectionsForRun is called by handleGetRun to embed detections inside
// the run detail response.
func (a *API) listDetectionsForRun(ctx context.Context, runID int64) ([]detectionRow, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT id, run_id, event_id, rule_name, severity, COALESCE(message, ''),
		       COALESCE(title, ''), COALESCE(summary, ''), COALESCE(details, ''), COALESCE(source, ''),
		       created_at
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
		var detailsJSON, sourceJSON string
		if err := rows.Scan(
			&d.ID, &d.RunID, &evtID, &d.RuleName, &d.Severity, &d.Message,
			&d.Title, &d.Summary, &detailsJSON, &sourceJSON, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		if evtID.Valid {
			d.EventID = &evtID.Int64
		}
		hydrateDetectionPresentation(&d, detailsJSON, sourceJSON)
		out = append(out, d)
	}
	return out, nil
}

func structuredDetectionValues(
	ruleName, message, title, summary string,
	details []detectionDetail,
	source *detectionSource,
) (string, string, string, string) {
	title = strings.TrimSpace(title)
	summary = strings.TrimSpace(summary)
	if title == "" {
		title = formatDetectionTitle(ruleName)
	}
	if summary == "" || len(details) == 0 {
		parsedSummary, parsedDetails := parseDetectionMessage(message)
		if summary == "" {
			summary = parsedSummary
		}
		if len(details) == 0 {
			details = parsedDetails
		}
	}
	detailsJSON, _ := json.Marshal(details)
	sourceJSON := ""
	if source != nil {
		b, _ := json.Marshal(source)
		sourceJSON = string(b)
	}
	return title, summary, string(detailsJSON), sourceJSON
}

func hydrateDetectionPresentation(d *detectionRow, detailsJSON, sourceJSON string) {
	if strings.TrimSpace(detailsJSON) != "" {
		_ = json.Unmarshal([]byte(detailsJSON), &d.Details)
	}
	if strings.TrimSpace(sourceJSON) != "" {
		var source detectionSource
		if err := json.Unmarshal([]byte(sourceJSON), &source); err == nil {
			d.Source = &source
		}
	}
	if d.Title == "" || d.Summary == "" || len(d.Details) == 0 {
		summary, details := parseDetectionMessage(d.Message)
		if d.Title == "" {
			d.Title = formatDetectionTitle(d.RuleName)
		}
		if d.Summary == "" {
			d.Summary = summary
		}
		if len(d.Details) == 0 {
			d.Details = details
		}
	}
}

func formatDetectionTitle(ruleName string) string {
	parts := strings.FieldsFunc(ruleName, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i, p := range parts {
		if p == "" {
			continue
		}
		switch strings.ToLower(p) {
		case "ip", "pid", "url", "tcp", "dns", "bpf":
			parts[i] = strings.ToUpper(p)
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	title := strings.Join(parts, " ")
	if strings.TrimSpace(title) == "" {
		return "Detection"
	}
	return title
}

var (
	processRe = regexp.MustCompile(`(?:^|process\s+)([a-zA-Z0-9_.-]+)\(pid=([0-9]+)`)
	fieldRe   = regexp.MustCompile(`([a-zA-Z_]+)="([^"]*)"`)
)

func parseDetectionMessage(message string) (string, []detectionDetail) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "No detection details were provided.", nil
	}
	details := make([]detectionDetail, 0, 6)
	if match := processRe.FindStringSubmatch(message); len(match) == 3 {
		details = append(details,
			detectionDetail{Label: "Process", Value: match[1]},
			detectionDetail{Label: "PID", Value: match[2]},
		)
	}
	for _, match := range fieldRe.FindAllStringSubmatch(message, -1) {
		if len(match) != 3 {
			continue
		}
		label := formatDetectionTitle(match[1])
		value := cleanDetectionValue(match[2])
		if value == "" {
			value = "unknown"
		}
		details = append(details, detectionDetail{Label: label, Value: value})
	}
	if strings.Contains(message, "blocked TCP connect") {
		return "Outbound TCP connection was blocked by Citadel.", details
	}
	if before, _, ok := strings.Cut(message, "—"); ok {
		return strings.TrimSpace(before), details
	}
	return message, details
}

func cleanDetectionValue(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.Contains(lower, "unknown hostname") {
		return "unknown"
	}
	return value
}

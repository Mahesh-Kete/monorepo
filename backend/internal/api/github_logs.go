package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type githubLogIngestRequest struct {
	Repository   string                `json:"repository"`
	Workflow     string                `json:"workflow"`
	WorkflowFile string                `json:"workflow_file,omitempty"`
	RunID        string                `json:"run_id"`
	RunNumber    string                `json:"run_number,omitempty"`
	SHA          string                `json:"sha,omitempty"`
	Ref          string                `json:"ref,omitempty"`
	Actor        string                `json:"actor,omitempty"`
	HTMLURL      string                `json:"html_url,omitempty"`
	DurationSec  int                   `json:"duration_sec,omitempty"`
	EventName    string                `json:"event_name,omitempty"`
	HeadBranch   string                `json:"head_branch,omitempty"`
	Status       string                `json:"status,omitempty"`
	Conclusion   string                `json:"conclusion,omitempty"`
	PolicyMode   string                `json:"policy_mode,omitempty"`
	Annotations  []githubLogAnnotation `json:"annotations,omitempty"`
}

type githubLogAnnotation struct {
	Level    string `json:"level,omitempty"`
	RuleName string `json:"rule_name,omitempty"`
	Message  string `json:"message"`
	Step     string `json:"step,omitempty"`
	Kind     string `json:"kind,omitempty"`
	HTMLURL  string `json:"html_url,omitempty"`
	Line     int    `json:"line,omitempty"`
}

func (a *API) handlePostGitHubLog(w http.ResponseWriter, r *http.Request) {
	var req githubLogIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Repository == "" || req.RunID == "" {
		writeError(w, http.StatusBadRequest, "repository and run_id are required")
		return
	}
	req.Repository = strings.ToLower(strings.TrimSpace(req.Repository))

	if req.Status == "" {
		req.Status = statusFromConclusion(req.Conclusion)
	}
	if req.Status == "" {
		req.Status = "completed"
	}
	if req.Conclusion == "" {
		req.Conclusion = conclusionFromStatus(req.Status)
	}
	if req.PolicyMode == "" {
		req.PolicyMode = "block"
	}
	if req.EventName == "" {
		req.EventName = "workflow_run"
	}

	ctx := r.Context()
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "begin tx: "+err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()

	runID, err := upsertGitHubLogRun(ctx, tx, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert run: "+err.Error())
		return
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM detections WHERE run_id = ?`, runID); err != nil {
		writeError(w, http.StatusInternalServerError, "replace detections: "+err.Error())
		return
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM github_action_logs WHERE run_id = ?`, runID); err != nil {
		writeError(w, http.StatusInternalServerError, "replace github action logs: "+err.Error())
		return
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE run_id = ?`, runID); err != nil {
		writeError(w, http.StatusInternalServerError, "replace events: "+err.Error())
		return
	}

	eventCount, detectionCount, err := insertGitHubLogAnnotations(ctx, tx, runID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert annotations: "+err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "commit: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"run_id":          runID,
		"events":          eventCount,
		"detections":      detectionCount,
		"github_run_id":   req.RunID,
		"github_html_url": req.HTMLURL,
	})
}

func upsertGitHubLogRun(ctx context.Context, tx *sql.Tx, req githubLogIngestRequest) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO runs (
			repository, workflow, run_id, run_number, sha, ref, actor, started_at,
			policy_mode, status, gh_status, gh_conclusion, gh_html_url, gh_duration_sec,
			gh_event_name, gh_head_branch, gh_synced_at, agent_seen
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 1)
		ON CONFLICT(repository, run_id) DO UPDATE SET
			workflow        = excluded.workflow,
			run_number      = excluded.run_number,
			sha             = excluded.sha,
			ref             = excluded.ref,
			actor           = excluded.actor,
			started_at      = CURRENT_TIMESTAMP,
			policy_mode     = excluded.policy_mode,
			status          = excluded.status,
			gh_status       = excluded.gh_status,
			gh_conclusion   = excluded.gh_conclusion,
			gh_html_url     = excluded.gh_html_url,
			gh_duration_sec = excluded.gh_duration_sec,
			gh_event_name   = excluded.gh_event_name,
			gh_head_branch  = excluded.gh_head_branch,
			gh_synced_at    = CURRENT_TIMESTAMP,
			agent_seen      = 1`,
		req.Repository, req.Workflow, req.RunID, req.RunNumber, req.SHA, req.Ref, req.Actor,
		req.PolicyMode, req.Status, "completed", req.Conclusion, req.HTMLURL, req.DurationSec,
		req.EventName, req.HeadBranch)
	if err != nil {
		return 0, err
	}
	if id, err := res.LastInsertId(); err == nil && id != 0 {
		return id, nil
	}

	var runID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM runs WHERE repository = ? AND run_id = ?`,
		req.Repository, req.RunID).Scan(&runID)
	return runID, err
}

func insertGitHubLogAnnotations(ctx context.Context, tx *sql.Tx, runID int64, req githubLogIngestRequest) (int, int, error) {
	annotations := req.Annotations
	if len(annotations) == 0 {
		annotations = []githubLogAnnotation{{
			Level:    severityFromConclusion(req.Conclusion),
			RuleName: "github.actions.workflow",
			Message:  fmt.Sprintf("GitHub Actions run %s completed with conclusion %q.", req.RunID, req.Conclusion),
			Step:     "GitHub Actions",
			Kind:     "process",
		}}
	}

	eventCount, detectionCount := 0, 0
	for i, ann := range annotations {
		if strings.TrimSpace(ann.Message) == "" {
			continue
		}
		if err := insertGitHubActionLog(ctx, tx, runID, req, ann); err != nil {
			return eventCount, detectionCount, err
		}
		eventType := eventTypeForAnnotation(ann)
		payload, processChain := githubLogPayload(req, ann, eventType, i)
		payloadBytes, _ := json.Marshal(payload)
		chainBytes, _ := json.Marshal(processChain)

		res, err := tx.ExecContext(ctx, `
			INSERT INTO events (run_id, type, timestamp, payload, process_chain, step)
			VALUES (?, ?, CURRENT_TIMESTAMP, ?, ?, ?)`,
			runID, eventType, string(payloadBytes), string(chainBytes), ann.Step)
		if err != nil {
			return eventCount, detectionCount, err
		}
		eventID, _ := res.LastInsertId()
		eventCount++

		severity := severityForAnnotation(ann)
		if severity == "" {
			continue
		}
		ruleName := ann.RuleName
		if ruleName == "" {
			ruleName = "github.actions.log"
		}
		source := &detectionSource{
			Line: ann.Line,
			URL:  firstNonEmpty(ann.HTMLURL, req.HTMLURL),
		}
		title, summary, detailsJSON, sourceJSON := structuredDetectionValues(
			ruleName,
			ann.Message,
			"",
			"",
			[]detectionDetail{
				{Label: "Job", Value: "build"},
				{Label: "Step", Value: firstNonEmpty(ann.Step, "GitHub Actions annotation")},
			},
			source,
		)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO detections (run_id, event_id, rule_name, severity, message, title, summary, details, source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, eventID, ruleName, severity, ann.Message, title, summary, detailsJSON, sourceJSON); err != nil {
			return eventCount, detectionCount, err
		}
		detectionCount++
	}
	return eventCount, detectionCount, nil
}

func insertGitHubActionLog(ctx context.Context, tx *sql.Tx, runID int64, req githubLogIngestRequest, ann githubLogAnnotation) error {
	level := strings.ToLower(strings.TrimSpace(ann.Level))
	if level == "" {
		level = severityForAnnotation(ann)
	}
	if level == "" {
		level = "notice"
	}
	htmlURL := ann.HTMLURL
	if htmlURL == "" {
		htmlURL = req.HTMLURL
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO github_action_logs (run_id, job_name, step, level, rule_name, message, html_url, line)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, 0))`,
		runID, "build", ann.Step, level, ann.RuleName, ann.Message, htmlURL, ann.Line)
	return err
}

func githubLogPayload(req githubLogIngestRequest, ann githubLogAnnotation, eventType string, idx int) (map[string]any, []string) {
	step := ann.Step
	if step == "" {
		step = "GitHub Actions annotation"
	}
	chain := []string{"GitHub Actions", req.Workflow}
	if req.WorkflowFile != "" {
		chain = append(chain, req.WorkflowFile)
	}
	chain = append(chain, step)

	payload := map[string]any{
		"id":            fmt.Sprintf("github-log-%s-%d", req.RunID, idx+1),
		"type":          eventType,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"process_chain": chain,
		"workflow": map[string]any{
			"repository":    req.Repository,
			"workflow":      req.Workflow,
			"workflow_file": req.WorkflowFile,
			"run_id":        req.RunID,
			"run_number":    req.RunNumber,
			"sha":           req.SHA,
			"ref":           req.Ref,
			"actor":         req.Actor,
			"event_name":    req.EventName,
			"job":           "build",
			"step":          step,
		},
	}
	if eventType == "network" {
		payload["network"] = map[string]any{
			"dst_ip":   "127.0.0.153",
			"dst_port": 53,
			"hostname": "Citadel DNS enforcement",
			"process":  "citadel-action",
			"blocked":  true,
		}
	} else {
		payload["process"] = map[string]any{
			"pid":      137,
			"ppid":     1,
			"uid":      1001,
			"comm":     "github-actions",
			"filename": req.WorkflowFile,
			"args":     []string{req.EventName, step},
		}
	}
	return payload, chain
}

func eventTypeForAnnotation(ann githubLogAnnotation) string {
	kind := strings.ToLower(strings.TrimSpace(ann.Kind))
	msg := strings.ToLower(ann.Message)
	switch {
	case kind == "network" || strings.Contains(msg, "dns") || strings.Contains(msg, "egress") || strings.Contains(msg, "nft"):
		return "network"
	default:
		return "process"
	}
}

func severityForAnnotation(ann githubLogAnnotation) string {
	level := strings.ToLower(strings.TrimSpace(ann.Level))
	if validSeverity[level] {
		return level
	}
	msg := strings.ToLower(ann.Message)
	switch {
	case strings.Contains(msg, "blocked") || strings.Contains(msg, "exit code 137") || strings.Contains(msg, "failure"):
		return "high"
	case strings.Contains(msg, "warning") || strings.Contains(msg, "deprecated"):
		return "medium"
	case strings.Contains(msg, "notice"):
		return "info"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func statusFromConclusion(conclusion string) string {
	switch strings.ToLower(conclusion) {
	case "failure", "cancelled", "timed_out", "action_required":
		return "blocked"
	case "success", "neutral", "skipped":
		return "completed"
	default:
		return ""
	}
}

func conclusionFromStatus(status string) string {
	switch strings.ToLower(status) {
	case "blocked", "failed", "failure":
		return "failure"
	case "completed", "success":
		return "success"
	default:
		return ""
	}
}

func severityFromConclusion(conclusion string) string {
	if strings.EqualFold(conclusion, "failure") {
		return "high"
	}
	return "info"
}

package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// policy is the JSON shape used for create/list responses. allowlist and
// detection_rules are stored as JSON strings in SQLite and surfaced as
// native JSON to clients.
type policy struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	ScopeRepo      string          `json:"scope_repo,omitempty"`
	ScopeWorkflow  string          `json:"scope_workflow,omitempty"`
	Mode           string          `json:"mode"`
	Allowlist      json.RawMessage `json:"allowlist,omitempty"`
	DetectionRules json.RawMessage `json:"detection_rules,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// defaultPolicy is returned by /api/policies/applicable when no DB row
// matches. Permissive on purpose — runs start in audit so people see
// "Citadel is watching" before they see "Citadel is blocking." Real users
// flip to block once they've reviewed a few clean runs.
var defaultPolicy = policy{
	ID:             0,
	Name:           "default-permissive",
	Mode:           "audit",
	Allowlist:      json.RawMessage(`[]`),
	DetectionRules: json.RawMessage(`{}`),
}

// ---------------------------------------------------------------------------
// GET /api/policies
// ---------------------------------------------------------------------------

func (a *API) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, name, COALESCE(scope_repo, ''), COALESCE(scope_workflow, ''),
			mode, COALESCE(allowlist, '[]'), COALESCE(detection_rules, '{}'),
			updated_at
		FROM policies
		ORDER BY updated_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	out := make([]policy, 0)
	for rows.Next() {
		var p policy
		var allowlist, rules string
		if err := rows.Scan(&p.ID, &p.Name, &p.ScopeRepo, &p.ScopeWorkflow,
			&p.Mode, &allowlist, &rules, &p.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		p.Allowlist = json.RawMessage(allowlist)
		p.DetectionRules = json.RawMessage(rules)
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// POST /api/policies
// ---------------------------------------------------------------------------

type createPolicyRequest struct {
	Name           string          `json:"name"`
	ScopeRepo      string          `json:"scope_repo,omitempty"`
	ScopeWorkflow  string          `json:"scope_workflow,omitempty"`
	Mode           string          `json:"mode"`
	Allowlist      json.RawMessage `json:"allowlist,omitempty"`
	DetectionRules json.RawMessage `json:"detection_rules,omitempty"`
}

func (a *API) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req createPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" || req.Mode == "" {
		writeError(w, http.StatusBadRequest, "name and mode are required")
		return
	}
	if req.Mode != "audit" && req.Mode != "block" {
		writeError(w, http.StatusBadRequest, "mode must be 'audit' or 'block'")
		return
	}

	allowlist := string(req.Allowlist)
	if allowlist == "" {
		allowlist = "[]"
	}
	rules := string(req.DetectionRules)
	if rules == "" {
		rules = "{}"
	}

	res, err := a.DB.ExecContext(r.Context(), `
		INSERT INTO policies (name, scope_repo, scope_workflow, mode, allowlist, detection_rules, updated_at)
		VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, CURRENT_TIMESTAMP)`,
		req.Name, req.ScopeRepo, req.ScopeWorkflow, req.Mode, allowlist, rules)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// ---------------------------------------------------------------------------
// GET /api/policies/{id}
// ---------------------------------------------------------------------------

func (a *API) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p policy
	var allowlist, rules string
	err := a.DB.QueryRowContext(r.Context(), `
		SELECT id, name, COALESCE(scope_repo, ''), COALESCE(scope_workflow, ''),
		       mode, COALESCE(allowlist, '[]'), COALESCE(detection_rules, '{}'),
		       updated_at
		FROM policies WHERE id = ?`, id).Scan(
		&p.ID, &p.Name, &p.ScopeRepo, &p.ScopeWorkflow,
		&p.Mode, &allowlist, &rules, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	p.Allowlist = json.RawMessage(allowlist)
	p.DetectionRules = json.RawMessage(rules)
	writeJSON(w, http.StatusOK, p)
}

// ---------------------------------------------------------------------------
// DELETE /api/policies/{id}
// ---------------------------------------------------------------------------

func (a *API) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := a.DB.ExecContext(r.Context(), `DELETE FROM policies WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// ---------------------------------------------------------------------------
// GET /api/policies/applicable?repo=X&workflow=Y
// ---------------------------------------------------------------------------

func (a *API) handleApplicablePolicy(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	workflow := r.URL.Query().Get("workflow")

	// Precedence: exact (repo, workflow) > repo-only > wildcard ('*' or NULL)
	queries := []struct {
		sql  string
		args []any
	}{
		{`SELECT id, name, COALESCE(scope_repo,''), COALESCE(scope_workflow,''), mode,
		   COALESCE(allowlist,'[]'), COALESCE(detection_rules,'{}'), updated_at
		  FROM policies WHERE scope_repo = ? AND scope_workflow = ? LIMIT 1`,
			[]any{repo, workflow}},
		{`SELECT id, name, COALESCE(scope_repo,''), COALESCE(scope_workflow,''), mode,
		   COALESCE(allowlist,'[]'), COALESCE(detection_rules,'{}'), updated_at
		  FROM policies WHERE scope_repo = ? AND scope_workflow IS NULL LIMIT 1`,
			[]any{repo}},
		{`SELECT id, name, COALESCE(scope_repo,''), COALESCE(scope_workflow,''), mode,
		   COALESCE(allowlist,'[]'), COALESCE(detection_rules,'{}'), updated_at
		  FROM policies WHERE scope_repo IS NULL AND scope_workflow IS NULL LIMIT 1`,
			nil},
	}

	for _, q := range queries {
		var p policy
		var allowlist, rules string
		err := a.DB.QueryRowContext(r.Context(), q.sql, q.args...).Scan(
			&p.ID, &p.Name, &p.ScopeRepo, &p.ScopeWorkflow, &p.Mode,
			&allowlist, &rules, &p.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query: "+err.Error())
			return
		}
		p.Allowlist = json.RawMessage(allowlist)
		p.DetectionRules = json.RawMessage(rules)
		writeJSON(w, http.StatusOK, p)
		return
	}

	// No DB match → return the hardcoded permissive default.
	writeJSON(w, http.StatusOK, defaultPolicy)
}

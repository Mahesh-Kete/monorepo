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

	"github.com/go-chi/chi/v5"

	"github.com/Mahesh-Kete/citadel/backend/internal/github"
)

// connectedRepo is the JSON returned to clients. Token is NEVER sent back
// — only stored.
type connectedRepo struct {
	ID           int64      `json:"id"`
	Repository   string     `json:"repository"`
	Note         string     `json:"note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	LastPolledAt *time.Time `json:"last_polled_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
}

type connectRepoRequest struct {
	Repository string `json:"repository"`     // owner/repo
	Token      string `json:"token"`          // PAT
	Note       string `json:"note,omitempty"`
}

// POST /api/repos/connect
//
// Validates the PAT by calling /user, then inserts into connected_repos.
// On conflict (repo already connected) replaces the token.
func (a *API) handleConnectRepo(w http.ResponseWriter, r *http.Request) {
	var req connectRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.Repository = strings.TrimSpace(req.Repository)
	req.Token = strings.TrimSpace(req.Token)

	if !validRepoFormat(req.Repository) {
		writeError(w, http.StatusBadRequest, "repository must be in owner/repo format")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cli := github.New()
	user, err := cli.Whoami(ctx, req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "token invalid: "+err.Error())
		return
	}

	// Quick sanity check that the token can also see the repo.
	if _, err := cli.ListRecentRuns(ctx, req.Token, req.Repository, 1); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot read %s with this token: %v", req.Repository, err))
		return
	}

	// Upsert.
	_, err = a.DB.ExecContext(r.Context(), `
		INSERT INTO connected_repos (repository, token, note)
		VALUES (?, ?, NULLIF(?, ''))
		ON CONFLICT(repository) DO UPDATE SET
			token=excluded.token,
			note=COALESCE(NULLIF(excluded.note, ''), note),
			last_error=NULL`,
		req.Repository, req.Token, req.Note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store: "+err.Error())
		return
	}

	a.Logger.Info("repo connected", "repo", req.Repository, "as_user", user.Login)
	writeJSON(w, http.StatusOK, map[string]string{
		"repository":   req.Repository,
		"authenticated_as": user.Login,
	})
}

// GET /api/repos
func (a *API) handleListRepos(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, repository, COALESCE(note, ''), created_at, last_polled_at, COALESCE(last_error, '')
		FROM connected_repos
		ORDER BY created_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

	out := make([]connectedRepo, 0)
	for rows.Next() {
		var c connectedRepo
		var lastPolledAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.Repository, &c.Note, &c.CreatedAt, &lastPolledAt, &c.LastError); err != nil {
			writeError(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		if lastPolledAt.Valid {
			t := lastPolledAt.Time
			c.LastPolledAt = &t
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/repos/{id}
func (a *API) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := a.DB.ExecContext(r.Context(), `DELETE FROM connected_repos WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// POST /api/repos/{id}/refresh — triggers an immediate poll of the given
// repo by setting last_polled_at far in the past. The background poller
// picks it up within ~1 sweep.
func (a *API) handleRefreshRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Find the repo, then trigger an immediate sync inline so the user
	// gets feedback right away.
	var repo, token string
	err := a.DB.QueryRowContext(r.Context(),
		`SELECT repository, token FROM connected_repos WHERE id = ?`, id).Scan(&repo, &token)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cli := github.New()
	runs, err := cli.ListRecentRuns(ctx, token, repo, 20)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"fetched": len(runs)})
}

func validRepoFormat(repo string) bool {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[1] != ""
}

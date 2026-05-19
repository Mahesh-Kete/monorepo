// Package github is a tiny client for the bits of the GitHub REST API we
// need: validating a PAT, listing recent workflow runs for a repo.
//
// Not a full SDK — we'd pull in google/go-github if we needed more.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL       = "https://api.github.com"
	defaultTimeout = 10 * time.Second
)

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: defaultTimeout}}
}

// User identifies the authenticated user for the given PAT. Used by the
// connect-repo endpoint to validate the token before we store it.
type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

func (c *Client) Whoami(ctx context.Context, token string) (*User, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/user", nil)
	c.attachAuth(req, token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("token unauthorized (%d)", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("/user: %s", resp.Status)
	}
	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// WorkflowRun matches the subset of fields we care about from
// /repos/{owner}/{repo}/actions/runs.
type WorkflowRun struct {
	ID           int64     `json:"id"`            // GITHUB_RUN_ID
	Name         string    `json:"name"`          // workflow name
	RunNumber    int64     `json:"run_number"`
	Event        string    `json:"event"`         // trigger event (push, pull_request, …)
	Status       string    `json:"status"`        // queued | in_progress | completed
	Conclusion   string    `json:"conclusion"`    // success | failure | cancelled | … | "" while running
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	HTMLURL      string    `json:"html_url"`
	RunStartedAt time.Time `json:"run_started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Actor        struct {
		Login string `json:"login"`
	} `json:"actor"`
	WorkflowID int64 `json:"workflow_id"`
	Path       string `json:"path"`
}

// DurationSec returns the run's wall-clock duration in seconds if it has
// completed; 0 otherwise.
func (r *WorkflowRun) DurationSec() int {
	if r.Status != "completed" || r.RunStartedAt.IsZero() || r.UpdatedAt.IsZero() {
		return 0
	}
	d := r.UpdatedAt.Sub(r.RunStartedAt)
	if d < 0 {
		return 0
	}
	return int(d.Seconds())
}

type runsResp struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

// ListRecentRuns returns the most-recent workflow runs for a repo. Default
// limit is 20; the GitHub API caps at 100.
func (c *Client) ListRecentRuns(ctx context.Context, token, repo string, limit int) ([]WorkflowRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	url := fmt.Sprintf("%s/repos/%s/actions/runs?per_page=%d", baseURL, repo, limit)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	c.attachAuth(req, token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("/actions/runs %s: %s: %s",
			repo, resp.Status, strings.TrimSpace(string(body)))
	}
	var rr runsResp
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, err
	}
	return rr.WorkflowRuns, nil
}

func (c *Client) attachAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "citadel-backend/0.1")
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

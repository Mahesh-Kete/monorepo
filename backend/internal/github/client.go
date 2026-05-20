// Package github is a tiny client for the bits of the GitHub REST API we
// need: validating a PAT, listing recent workflow runs for a repo.
//
// Not a full SDK — we'd pull in google/go-github if we needed more.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL        = "https://api.github.com"
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
	ID           int64     `json:"id"`   // GITHUB_RUN_ID
	Name         string    `json:"name"` // workflow name
	RunNumber    int64     `json:"run_number"`
	Event        string    `json:"event"`      // trigger event (push, pull_request, …)
	Status       string    `json:"status"`     // queued | in_progress | completed
	Conclusion   string    `json:"conclusion"` // success | failure | cancelled | … | "" while running
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	HTMLURL      string    `json:"html_url"`
	RunStartedAt time.Time `json:"run_started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Actor        struct {
		Login string `json:"login"`
	} `json:"actor"`
	WorkflowID int64  `json:"workflow_id"`
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

// RegistrationToken mints a one-time self-hosted runner registration token
// for the repo. Tokens are valid for 1 hour. The caller is expected to feed
// it into `./config.sh --token …` on the runner host.
type RegistrationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (c *Client) RegistrationToken(ctx context.Context, token, repo string) (*RegistrationToken, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/actions/runners/registration-token", baseURL, repo), nil)
	c.attachAuth(req, token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("registration-token %s: %s: %s",
			repo, resp.Status, strings.TrimSpace(string(body)))
	}
	var rt RegistrationToken
	if err := json.NewDecoder(resp.Body).Decode(&rt); err != nil {
		return nil, err
	}
	return &rt, nil
}

// DefaultBranch returns the repository's default branch (e.g., "main").
func (c *Client) DefaultBranch(ctx context.Context, token, repo string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/repos/%s", baseURL, repo), nil)
	c.attachAuth(req, token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("/repos/%s: %s: %s", repo, resp.Status, strings.TrimSpace(string(body)))
	}
	var r struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.DefaultBranch == "" {
		return "main", nil
	}
	return r.DefaultBranch, nil
}

// PutFileResult describes what PutWorkflowFile did. The handler surfaces this
// in the connect-repo response so the UI can tell the user what happened.
type PutFileResult struct {
	Created bool   // true iff a new commit was made
	Skipped bool   // true iff the file already existed (we never overwrite)
	Message string // human-readable summary
	HTMLURL string // GitHub URL to the new file commit (if Created)
}

// PutWorkflowFile commits the given content to repo:path on the default
// branch if (and only if) the file doesn't already exist. We never overwrite
// to avoid clobbering user customisations.
func (c *Client) PutWorkflowFile(
	ctx context.Context,
	token, repo, path string,
	content []byte,
	commitMessage string,
) (PutFileResult, error) {
	contentsURL := fmt.Sprintf("%s/repos/%s/contents/%s", baseURL, repo, path)

	// 1. Probe — does the file exist already?
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, contentsURL, nil)
	c.attachAuth(getReq, token)
	getResp, err := c.http.Do(getReq)
	if err != nil {
		return PutFileResult{}, fmt.Errorf("probe: %w", err)
	}
	defer drain(getResp)

	switch {
	case getResp.StatusCode == 200:
		return PutFileResult{
			Skipped: true,
			Message: fmt.Sprintf("%s already exists; left untouched", path),
		}, nil
	case getResp.StatusCode == 404:
		// fallthrough to PUT
	default:
		body, _ := io.ReadAll(io.LimitReader(getResp.Body, 512))
		return PutFileResult{}, fmt.Errorf("probe %s: %s: %s",
			path, getResp.Status, strings.TrimSpace(string(body)))
	}

	// 2. Resolve default branch so we commit to the right place.
	branch, err := c.DefaultBranch(ctx, token, repo)
	if err != nil {
		return PutFileResult{}, fmt.Errorf("default branch: %w", err)
	}

	// 3. PUT the new file. GitHub expects base64-encoded content.
	putBody := map[string]string{
		"message": commitMessage,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}
	putBytes, _ := json.Marshal(putBody)
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, contentsURL, bytes.NewReader(putBytes))
	c.attachAuth(putReq, token)
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := c.http.Do(putReq)
	if err != nil {
		return PutFileResult{}, fmt.Errorf("put: %w", err)
	}
	defer drain(putResp)
	if putResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(putResp.Body, 1024))
		return PutFileResult{}, fmt.Errorf("put %s: %s: %s",
			path, putResp.Status, strings.TrimSpace(string(body)))
	}

	var pr struct {
		Content struct {
			HTMLURL string `json:"html_url"`
		} `json:"content"`
	}
	_ = json.NewDecoder(putResp.Body).Decode(&pr)
	return PutFileResult{
		Created: true,
		Message: fmt.Sprintf("committed %s to %s", path, branch),
		HTMLURL: pr.Content.HTMLURL,
	}, nil
}

func (c *Client) attachAuth(req *http.Request, token string) {
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "citadel-backend/0.1")
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// Package policy loads the current Citadel policy from the backend.
//
// Precedence + the hardcoded permissive default are resolved by the backend
// (see backend/internal/api/policies.go handleApplicablePolicy). This package
// just consumes the JSON, parses the allowlist into a fast-lookup form, and
// exposes Should* helpers to the rest of the agent.
//
// SIGHUP causes the agent (main.go) to call Reload, which re-fetches the
// policy from the backend without restarting probes.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Policy is the in-memory view of the active policy for this run.
type Policy struct {
	Name             string
	Mode             string                       // "audit" | "block"
	AllowedDomains   []string                     // raw entries; matched with glob support
	AllowedIPs       []string                     // exact IPv4/v6 strings
	DetectionActions map[string]string            // rule_name -> "kill"|"fail"|""
}

// backendPolicy matches the JSON returned by GET /api/policies/applicable.
type backendPolicy struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	ScopeRepo      string            `json:"scope_repo,omitempty"`
	ScopeWorkflow  string            `json:"scope_workflow,omitempty"`
	Mode           string            `json:"mode"`
	Allowlist      []string          `json:"allowlist"`
	DetectionRules map[string]string `json:"detection_rules"`
}

// Default returns the permissive audit-mode default. Used when the backend
// is unreachable or no policy matches.
func Default() *Policy {
	return &Policy{
		Name: "default-permissive",
		Mode: "audit",
	}
}

// LoadFromBackend fetches the most-specific policy matching (repo, workflow)
// and parses it. On any error a default permissive policy is returned along
// with the error — callers should log the error but otherwise proceed.
func LoadFromBackend(ctx context.Context, backendURL, repo, workflow string) (*Policy, error) {
	if backendURL == "" {
		return Default(), nil
	}
	url := fmt.Sprintf("%s/api/policies/applicable?repo=%s&workflow=%s",
		strings.TrimRight(backendURL, "/"), repo, workflow)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Default(), err
	}
	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return Default(), fmt.Errorf("get policy: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		return Default(), fmt.Errorf("policy endpoint: %d", resp.StatusCode)
	}
	var bp backendPolicy
	if err := json.NewDecoder(resp.Body).Decode(&bp); err != nil {
		return Default(), fmt.Errorf("decode policy: %w", err)
	}

	out := &Policy{
		Name:             bp.Name,
		Mode:             bp.Mode,
		AllowedDomains:   bp.Allowlist,
		DetectionActions: bp.DetectionRules,
	}
	if out.Mode == "" {
		out.Mode = "audit"
	}
	if out.DetectionActions == nil {
		out.DetectionActions = map[string]string{}
	}
	return out, nil
}

// ShouldBlockDomain returns true iff we're in block mode AND the hostname is
// not on the allowlist. Wildcards in the allowlist (e.g. "*.docker.io") match
// any subdomain. Empty hostname → allow (we can't make a decision).
func (p *Policy) ShouldBlockDomain(hostname string) bool {
	if p == nil || p.Mode != "block" || hostname == "" {
		return false
	}
	for _, pat := range p.AllowedDomains {
		if matchDomain(pat, hostname) {
			return false
		}
	}
	return true
}

// ShouldKillProcess returns true if the detection rule's configured action
// implies process kill. Used by the enforcer.
func (p *Policy) ShouldKillProcess(rule string) bool {
	if p == nil {
		return false
	}
	a, ok := p.DetectionActions[rule]
	if !ok {
		return false
	}
	return a == "kill" || a == "fail"
}

// matchDomain handles either an exact match or a wildcard "*.example.com"
// prefix match. Comparisons are case-insensitive.
func matchDomain(pattern, host string) bool {
	pattern = strings.ToLower(pattern)
	host = strings.ToLower(host)
	if pattern == host {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return false
}

// Watcher periodically refreshes the policy. Use this if you want background
// reload; the agent currently reloads on SIGHUP (see main.go) instead.
type Watcher struct {
	mu     sync.RWMutex
	policy *Policy
}

func NewWatcher(initial *Policy) *Watcher {
	if initial == nil {
		initial = Default()
	}
	return &Watcher{policy: initial}
}

func (w *Watcher) Get() *Policy {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.policy
}

func (w *Watcher) Set(p *Policy) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.policy = p
}

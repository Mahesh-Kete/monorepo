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
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Policy is the in-memory view of the active policy for this run.
type Policy struct {
	Name             string
	Mode             string            // "audit" | "block"
	AllowedDomains   []string          // raw entries; matched with glob support
	AllowedIPs       []string          // exact IPv4/v6 strings from the policy itself
	DetectionActions map[string]string // rule_name -> "kill"|"fail"|""

	// AllowedIPSet is populated by ResolveAllowlist (called at agent startup
	// and on policy reload). Block-mode decisions check this set: any
	// outbound connect whose destination IP is *not* in here is dropped.
	// Pre-resolution sidesteps the reverse-DNS roundtrip — which is unreliable
	// for cloud-hosted destinations whose PTR records don't roundtrip with the
	// forward A records — and lets us block the first packet, not just the
	// retransmits.
	AllowedIPSet map[string]struct{}
}

// ResolveAllowlist forward-resolves every entry in AllowedDomains, plus any
// literal AllowedIPs, plus a few baseline pinned IPs (loopback, the supplied
// extraHosts) and stores the union in AllowedIPSet. Lookup failures are
// logged and skipped.
func (p *Policy) ResolveAllowlist(ctx context.Context, log *slog.Logger, extraHosts []string) {
	if p == nil {
		return
	}
	p.AllowedIPSet = make(map[string]struct{}, 64)
	// Always allow loopback so the agent can reach its own backend.
	for _, ip := range []string{"127.0.0.1", "::1"} {
		p.AllowedIPSet[ip] = struct{}{}
	}
	for _, ip := range p.AllowedIPs {
		p.AllowedIPSet[ip] = struct{}{}
	}
	domains := append([]string{}, p.AllowedDomains...)
	domains = append(domains, extraHosts...)
	r := net.DefaultResolver
	c, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" || strings.HasPrefix(d, "*.") {
			continue // wildcard domains can't be forward-resolved.
		}
		ips, err := r.LookupIPAddr(c, d)
		if err != nil {
			if log != nil {
				log.Warn("resolve allowlist domain", "domain", d, "err", err)
			}
			continue
		}
		for _, ip := range ips {
			p.AllowedIPSet[ip.IP.String()] = struct{}{}
		}
	}
	if log != nil {
		log.Info("allowlist resolved",
			"domains", len(domains), "ips", len(p.AllowedIPSet))
	}
}

// ShouldBlockIP returns true iff we're in block mode and the destination IP
// is not in the pre-resolved AllowedIPSet. Loopback addresses are always
// allowed (seeded by ResolveAllowlist).
func (p *Policy) ShouldBlockIP(ip net.IP) bool {
	if p == nil || p.Mode != "block" || ip == nil {
		return false
	}
	if _, ok := p.AllowedIPSet[ip.String()]; ok {
		return false
	}
	return true
}

// backendPolicy matches the JSON returned by GET /api/policies/applicable.
//
// DetectionRules is intentionally a permissive shape (RawMessage values) so
// the agent decodes both the legacy schema ({rule: "kill"}) and the richer
// dashboard schema ({rule: {enabled, severity, ...}}). The agent only cares
// about the action keyword when present; configuration knobs the detector
// uses are ignored here.
type backendPolicy struct {
	ID             int64                      `json:"id"`
	Name           string                     `json:"name"`
	ScopeRepo      string                     `json:"scope_repo,omitempty"`
	ScopeWorkflow  string                     `json:"scope_workflow,omitempty"`
	Mode           string                     `json:"mode"`
	Allowlist      []string                   `json:"allowlist"`
	DetectionRules map[string]json.RawMessage `json:"detection_rules"`
}

// extractActions normalises whatever shape the backend served into the simple
// rule_name -> action_string map the agent uses for kill/fail decisions.
// Unknown shapes safely yield an empty string (= no enforcement action).
func extractActions(raw map[string]json.RawMessage) map[string]string {
	out := make(map[string]string, len(raw))
	for rule, msg := range raw {
		var s string
		if err := json.Unmarshal(msg, &s); err == nil {
			out[rule] = s
			continue
		}
		var obj struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(msg, &obj); err == nil {
			out[rule] = obj.Action
			continue
		}
		out[rule] = ""
	}
	return out
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
		DetectionActions: extractActions(bp.DetectionRules),
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

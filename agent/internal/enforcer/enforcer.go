// Package enforcer is the userspace half of "block mode" — it polls the
// backend for new detections and, for those whose configured action is
// "kill" or "fail", sends SIGKILL to the offending process.
//
// This is the "Path B" choice from the build plan: instead of in-kernel
// signal delivery via bpf_send_signal (which has tricky verifier + kernel-
// version requirements), we do all kill decisions in userspace. The latency
// from detection-write-to-backend → poll → kill is on the order of 2 s; for
// the hackathon demo that's plenty fast — the reverse-shell process is still
// in `read()` waiting for attacker input.
package enforcer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mahesh-Kete/citadel/agent/internal/policy"
)

const (
	pollInterval    = 2 * time.Second
	defaultLookback = 30 * time.Second
)

type detection struct {
	ID        int64  `json:"id"`
	RunID     int64  `json:"run_id"`
	EventID   *int64 `json:"event_id,omitempty"`
	RuleName  string `json:"rule_name"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// Enforcer polls /api/detections and acts on the ones whose rule the policy
// says to kill.
type Enforcer struct {
	BackendURL string
	Logger     *slog.Logger
	Policy     *policy.Watcher

	httpClient *http.Client
	seen       map[int64]bool
	mu         sync.Mutex
}

func New(backendURL string, logger *slog.Logger, watcher *policy.Watcher) *Enforcer {
	return &Enforcer{
		BackendURL: strings.TrimRight(backendURL, "/"),
		Logger:     logger,
		Policy:     watcher,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		seen:       map[int64]bool{},
	}
}

// Start launches the polling loop. Returns immediately. The loop exits when
// ctx is cancelled.
func (e *Enforcer) Start(ctx context.Context) {
	if e.BackendURL == "" {
		// Without a backend there are no detections to react to.
		e.Logger.Info("enforcer disabled (no backend URL)")
		return
	}
	go e.loop(ctx)
}

func (e *Enforcer) loop(ctx context.Context) {
	since := time.Now().UTC().Add(-defaultLookback)
	e.Logger.Info("enforcer started", "poll_interval", pollInterval)

	for {
		select {
		case <-ctx.Done():
			e.Logger.Info("enforcer stopped")
			return
		case <-time.After(pollInterval):
		}

		dets, err := e.fetchDetections(ctx, since)
		if err != nil {
			e.Logger.Debug("fetch detections", "err", err)
			continue
		}
		for _, d := range dets {
			if e.alreadySeen(d.ID) {
				continue
			}
			pol := e.Policy.Get()
			if !pol.ShouldKillProcess(d.RuleName) {
				e.markSeen(d.ID)
				continue
			}
			e.act(d)
			e.markSeen(d.ID)
		}
		since = time.Now().UTC()
	}
}

func (e *Enforcer) fetchDetections(ctx context.Context, since time.Time) ([]detection, error) {
	url := fmt.Sprintf("%s/api/detections?since=%s",
		e.BackendURL, since.Format(time.RFC3339Nano))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("detections endpoint: %d", resp.StatusCode)
	}
	var out []detection
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// act consumes one detection and takes the configured enforcement action.
//
// For the hackathon we extract the PID heuristically from the message —
// rules in /detector/app/rules embed the pid in the message string (e.g.
// "pid 4321"). A more robust implementation would fetch the underlying
// event from the backend and pull pid from the payload JSON. That's a
// stretch goal; this approach works for the demo because we control both
// sides of the message format.
func (e *Enforcer) act(d detection) {
	pid := extractPID(d.Message)
	if pid == 0 {
		e.Logger.Warn("enforcer: kill action without resolvable pid",
			"rule", d.RuleName, "detection_id", d.ID, "message", d.Message)
		return
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		e.Logger.Warn("enforcer: SIGKILL failed",
			"pid", pid, "rule", d.RuleName, "err", err)
		return
	}
	e.Logger.Info("enforcer: killed process",
		"pid", pid, "rule", d.RuleName, "severity", d.Severity)
}

func (e *Enforcer) alreadySeen(id int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.seen[id]
}

func (e *Enforcer) markSeen(id int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seen[id] = true
	// Cap memory by trimming once it gets large. Detections at this rate are
	// rare; a hard cap of 10k is plenty for a single CI run.
	if len(e.seen) > 10_000 {
		e.seen = map[int64]bool{}
	}
}

// extractPID scans a detection message for "pid N" and returns N (or 0).
// Matches the format the Python rules use.
func extractPID(msg string) int {
	// Linear scan — message is short. Look for either "pid " or "pid="
	// followed by a number.
	for i := 0; i+4 < len(msg); i++ {
		s := msg[i:]
		if strings.HasPrefix(s, "pid ") || strings.HasPrefix(s, "pid=") {
			j := 4
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j == 4 {
				continue
			}
			pid := 0
			for k := 4; k < j; k++ {
				pid = pid*10 + int(s[k]-'0')
			}
			return pid
		}
	}
	return 0
}

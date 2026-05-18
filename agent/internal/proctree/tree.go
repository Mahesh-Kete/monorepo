// Package proctree maintains an in-memory cache of recent process execs so
// that other probes (network, file) can enrich their events with the full
// process ancestry chain — e.g. [bash, npm, node, curl] for a curl call
// spawned several frames deep into an npm script.
//
// The cache is populated by the process eBPF probe (see internal/probes/proc).
// Entries expire lazily on Add when older than 1 hour, so the map never
// grows unbounded.
package proctree

import (
	"sync"
	"time"

	procprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/proc"
)

// ProcessInfo is the minimal slice of a process exec we keep around to
// reconstruct ancestry later.
type ProcessInfo struct {
	PID       uint32    `json:"pid"`
	PPID      uint32    `json:"ppid"`
	Comm      string    `json:"comm"`
	Args      []string  `json:"args"`
	StartTime time.Time `json:"start_time"`
}

const (
	defaultExpiry  = 1 * time.Hour
	sweepEveryAdds = 200
)

type Tree struct {
	mu      sync.RWMutex
	m       map[uint32]*ProcessInfo
	addsSeen uint64
	expiry  time.Duration
}

// New returns a fresh tree with the default 1-hour expiry.
func New() *Tree {
	return &Tree{
		m:      make(map[uint32]*ProcessInfo),
		expiry: defaultExpiry,
	}
}

// Add stores a process exec event. Every Nth call we sweep expired entries.
func (t *Tree) Add(e procprobe.ProcEvent) {
	t.mu.Lock()
	t.m[e.PID] = &ProcessInfo{
		PID:       e.PID,
		PPID:      e.PPID,
		Comm:      e.Comm,
		Args:      e.Args,
		StartTime: e.Timestamp,
	}
	t.addsSeen++
	if t.addsSeen%sweepEveryAdds == 0 {
		t.sweepLocked()
	}
	t.mu.Unlock()
}

// Find returns the ProcessInfo for a PID, or nil if unknown.
func (t *Tree) Find(pid uint32) *ProcessInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if p, ok := t.m[pid]; ok {
		copy := *p
		return &copy
	}
	return nil
}

// Ancestry returns the chain of processes from pid up to (but not including)
// the first unknown ancestor — typically PID 1 or the systemd-spawned shell.
// The slice is ordered child → parent → grandparent → ...
func (t *Tree) Ancestry(pid uint32) []ProcessInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var chain []ProcessInfo
	cur := pid
	for range 64 { // hard cap on depth — avoids cycles
		p, ok := t.m[cur]
		if !ok {
			break
		}
		chain = append(chain, *p)
		if p.PPID == 0 || p.PPID == cur {
			break
		}
		cur = p.PPID
	}
	return chain
}

// AncestryComms is a convenience for the common "just give me the comm
// chain" case (e.g. logging a process chain inline with a network event).
func (t *Tree) AncestryComms(pid uint32) []string {
	chain := t.Ancestry(pid)
	if len(chain) == 0 {
		return nil
	}
	out := make([]string, len(chain))
	for i, p := range chain {
		out[i] = p.Comm
	}
	return out
}

// sweepLocked drops entries older than the expiry. Caller must hold t.mu.
func (t *Tree) sweepLocked() {
	cutoff := time.Now().Add(-t.expiry)
	for pid, p := range t.m {
		if p.StartTime.Before(cutoff) {
			delete(t.m, pid)
		}
	}
}

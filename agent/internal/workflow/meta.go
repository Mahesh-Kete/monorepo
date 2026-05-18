// Package workflow loads GitHub Actions workflow metadata so that every
// event we ship can be tagged with which run / job / step it came from.
//
// Sources, in order of precedence (later sources override earlier ones for
// non-empty fields):
//
//   1. GITHUB_* env vars currently set on the agent process.
//   2. /tmp/citadel-meta.json — written by the citadel-setup composite
//      action just before the agent starts. Contains the GITHUB_* vars
//      as the runner saw them, so it survives even after the workflow
//      step that started the agent exits.
//   3. /tmp/citadel-current-step — sentinel file the composite action's
//      step wrapper updates between user steps. Provides Meta.Step.
//
// Reads are cached for 500ms — the Step value changes per workflow step
// (potentially every second on a busy CI), but we don't want to stat() the
// filesystem on every single event.
package workflow

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// Meta is the workflow-context tag attached to every event. JSON tags are
// snake_case to match the unified Event schema in /agent/internal/events.
type Meta struct {
	Repository   string `json:"repository,omitempty"`
	Workflow     string `json:"workflow,omitempty"`
	WorkflowFile string `json:"workflow_file,omitempty"`
	RunID        string `json:"run_id,omitempty"`
	RunNumber    string `json:"run_number,omitempty"`
	SHA          string `json:"sha,omitempty"`
	Ref          string `json:"ref,omitempty"`
	Actor        string `json:"actor,omitempty"`
	EventName    string `json:"event_name,omitempty"`
	Job          string `json:"job,omitempty"`
	Step         string `json:"step,omitempty"`
}

// Default sentinel paths. Overridable via NewLoader fields if needed.
const (
	defaultMetaFile    = "/tmp/citadel-meta.json"
	defaultStepFile    = "/tmp/citadel-current-step"
	defaultRefreshTTL  = 500 * time.Millisecond
)

// Loader caches the merged Meta and refreshes from disk at most every TTL.
type Loader struct {
	metaFile    string
	stepFile    string
	refreshTTL  time.Duration

	mu          sync.Mutex
	cached      Meta
	lastRefresh time.Time
}

// NewLoader returns a Loader. metaFile may be empty to disable file lookups
// (env-only mode).
func NewLoader(metaFile string) *Loader {
	if metaFile == "" {
		metaFile = defaultMetaFile
	}
	return &Loader{
		metaFile:   metaFile,
		stepFile:   defaultStepFile,
		refreshTTL: defaultRefreshTTL,
	}
}

// Get returns the current Meta. Cached for refreshTTL.
func (l *Loader) Get() Meta {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.lastRefresh) < l.refreshTTL && !l.cached.isZero() {
		return l.cached
	}
	l.cached = l.reload()
	l.lastRefresh = time.Now()
	return l.cached
}

// reload reads env + meta file + step sentinel and returns the merged Meta.
func (l *Loader) reload() Meta {
	// Start with env vars — these set the floor for what we know.
	m := metaFromEnv()

	// Overlay anything from the meta JSON file (if present).
	if data, err := os.ReadFile(l.metaFile); err == nil {
		var fromFile Meta
		if json.Unmarshal(data, &fromFile) == nil {
			mergeNonEmpty(&m, &fromFile)
		}
	}

	// Step sentinel — updated per-step by the action's step wrapper.
	if data, err := os.ReadFile(l.stepFile); err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			m.Step = s
		}
	}

	return m
}

// metaFromEnv reads the GITHUB_* environment variables that GitHub Actions
// sets in every step.
func metaFromEnv() Meta {
	return Meta{
		Repository:   os.Getenv("GITHUB_REPOSITORY"),
		Workflow:     os.Getenv("GITHUB_WORKFLOW"),
		WorkflowFile: os.Getenv("GITHUB_WORKFLOW_REF"),
		RunID:        os.Getenv("GITHUB_RUN_ID"),
		RunNumber:    os.Getenv("GITHUB_RUN_NUMBER"),
		SHA:          os.Getenv("GITHUB_SHA"),
		Ref:          os.Getenv("GITHUB_REF"),
		Actor:        os.Getenv("GITHUB_ACTOR"),
		EventName:    os.Getenv("GITHUB_EVENT_NAME"),
		Job:          os.Getenv("GITHUB_JOB"),
	}
}

// mergeNonEmpty copies non-empty fields from src into dst.
func mergeNonEmpty(dst, src *Meta) {
	if src.Repository != "" {
		dst.Repository = src.Repository
	}
	if src.Workflow != "" {
		dst.Workflow = src.Workflow
	}
	if src.WorkflowFile != "" {
		dst.WorkflowFile = src.WorkflowFile
	}
	if src.RunID != "" {
		dst.RunID = src.RunID
	}
	if src.RunNumber != "" {
		dst.RunNumber = src.RunNumber
	}
	if src.SHA != "" {
		dst.SHA = src.SHA
	}
	if src.Ref != "" {
		dst.Ref = src.Ref
	}
	if src.Actor != "" {
		dst.Actor = src.Actor
	}
	if src.EventName != "" {
		dst.EventName = src.EventName
	}
	if src.Job != "" {
		dst.Job = src.Job
	}
	if src.Step != "" {
		dst.Step = src.Step
	}
}

func (m Meta) isZero() bool {
	return m == Meta{}
}

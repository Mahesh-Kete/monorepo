// Package api wires the Citadel backend HTTP routes and middleware.
//
// One API struct holds the DB handle and exposes methods that implement
// chi-compatible http.Handler functions. Routes are grouped under /api/*.
package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type API struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// New builds the router with all routes mounted.
func New(db *sql.DB, logger *slog.Logger) http.Handler {
	a := &API{DB: db, Logger: logger}

	r := chi.NewRouter()

	// --- middleware ---
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(a.requestLogger)
	r.Use(corsMiddleware)

	// --- routes ---
	r.Get("/healthz", healthz)

	r.Route("/api", func(r chi.Router) {
		r.Post("/events", a.handlePostEvents)
		r.Get("/events", a.handleListEvents)

		r.Get("/runs", a.handleListRuns)
		r.Delete("/runs/unknown", a.handleDeleteUnknownRuns)
		r.Get("/runs/{id}", a.handleGetRun)
		r.Delete("/runs/{id}", a.handleDeleteRun)
		r.Get("/runs/{id}/process-tree", a.handleGetProcessTree)
		r.Get("/runs/{id}/baseline-domains", a.handleGetBaselineDomains)

		r.Get("/detections", a.handleListDetections)
		r.Post("/detections", a.handlePostDetection)
		r.Post("/detections/by-github-run", a.handlePostDetectionByGitHub)
		r.Post("/github-actions/logs", a.handlePostGitHubLog)

		r.Get("/policies", a.handleListPolicies)
		r.Post("/policies", a.handleCreatePolicy)
		r.Get("/policies/applicable", a.handleApplicablePolicy)
		r.Get("/policies/{id}", a.handleGetPolicy)
		r.Delete("/policies/{id}", a.handleDeletePolicy)

		r.Get("/repos", a.handleListRepos)
		r.Post("/repos/connect", a.handleConnectRepo)
		r.Delete("/repos/{id}", a.handleDeleteRepo)
		r.Post("/repos/{id}/refresh", a.handleRefreshRepo)
		r.Get("/repos/{id}/runner-bootstrap.sh", a.handleRunnerBootstrap)
	})

	return r
}

// ---------------------------------------------------------------------------
// Common helpers
// ---------------------------------------------------------------------------

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// corsMiddleware permits the dashboard origin (localhost:3000) and any
// other origin during the hackathon. Tighten later.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requestLogger is a tiny slog-based access logger so we don't pull in
// middleware.Logger's stdout writer.
func (a *API) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		// Skip the noisy healthz line so logs stay readable.
		if strings.HasSuffix(r.URL.Path, "/healthz") {
			return
		}
		a.Logger.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
		)
	})
}

// writeJSON marshals v and writes it as 200 OK JSON. On marshal error it
// writes a 500.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "json marshal: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

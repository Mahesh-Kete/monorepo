// Package main is the citadel-backend entrypoint.
//
// One process: chi HTTP server on :8080, modernc.org/sqlite database. The
// agent POSTs events; the dashboard reads runs/events/detections; the
// Python detector polls events and writes detections back. See router.go
// for the endpoint list.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mahesh-Kete/citadel/backend/internal/api"
	"github.com/Mahesh-Kete/citadel/backend/internal/db"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db-path", "/data/citadel.db", "SQLite file path; :memory: for ephemeral testing")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Allow DB_PATH env to override the flag for container ergonomics
	// (docker-compose.yml sets DB_PATH=/data/citadel.db).
	if envPath := os.Getenv("DB_PATH"); envPath != "" {
		*dbPath = envPath
	}

	logger.Info("citadel-backend starting", "addr", *addr, "db", *dbPath)

	database, err := db.Open(*dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	handler := api.New(database, logger)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown: SIGINT/SIGTERM → Shutdown(10s).
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", *addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("server shutdown error", "err", err)
	}
	fmt.Fprintln(os.Stderr, "citadel-backend exited cleanly")
}

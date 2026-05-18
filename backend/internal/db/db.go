// Package db opens the SQLite database and runs the embedded migrations.
//
// We use modernc.org/sqlite (a pure-Go CGo-free SQLite port) so the backend
// binary is statically linkable for distroless images. The pragmas applied
// at open time enable WAL mode (better concurrent read/write), a generous
// busy timeout, and foreign-key enforcement.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open returns a *sql.DB with the right pragmas + applied migrations.
// path is the SQLite file path (use ":memory:" for tests).
func Open(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		// Pragmas via DSN query string — modernc.org/sqlite supports this.
		dsn = "file:" + path +
			"?_pragma=journal_mode(wal)" +
			"&_pragma=busy_timeout(5000)" +
			"&_pragma=foreign_keys(on)"
	}
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := runMigrations(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return conn, nil
}

// runMigrations applies every .sql file in the embedded migrations dir, in
// lexicographic order. Each file is expected to be idempotent
// (CREATE TABLE IF NOT EXISTS, etc.).
func runMigrations(db *sql.DB) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		data, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("apply %s: %w", f, err)
		}
	}
	return nil
}

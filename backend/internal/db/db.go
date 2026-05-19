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
// lexicographic order. Each file's statements are split on ';' and executed
// one at a time so that idempotent statements (CREATE TABLE IF NOT EXISTS)
// don't abort the whole file if a later non-idempotent statement (ALTER
// TABLE ADD COLUMN) errors on re-run.
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
		if err := execMigration(db, string(data)); err != nil {
			return fmt.Errorf("apply %s: %w", f, err)
		}
	}
	return nil
}

// execMigration runs each ;-delimited statement. Errors on
// "duplicate column" are tolerated (ALTER TABLE ADD COLUMN re-run on an
// already-migrated DB). All other errors propagate.
func execMigration(db *sql.DB, raw string) error {
	stmts := splitStatements(raw)
	for _, s := range stmts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, err := db.Exec(s); err != nil {
			if isTolerableMigrationErr(err) {
				continue
			}
			return fmt.Errorf("stmt %q: %w", firstLine(s), err)
		}
	}
	return nil
}

func splitStatements(raw string) []string {
	// Strip SQL line-comments (anything from `--` to end of line) before
	// splitting on `;` — otherwise a semicolon inside a comment would split
	// a CREATE TABLE in half. Migration files we control have no string
	// literals containing `--`, so this is safe.
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	cleaned := b.String()

	parts := strings.Split(cleaned, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func isTolerableMigrationErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate column name") ||
		strings.Contains(msg, "already exists")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

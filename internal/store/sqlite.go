// Package store is the SQLite persistence adapter. It implements
// domain.ProjectRepository, domain.DeploymentRepository and
// domain.ServerRepository via a single *Store type backed by modernc.org/sqlite
// (pure-Go, no CGO).
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store is the concrete SQLite adapter. A zero-value is not usable; always
// construct with New.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath, ensures the parent
// directory exists, enables foreign keys and the busy-timeout pragma, pings
// the database, and runs the idempotent schema migration before returning.
func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("store: create db dir: %w", err)
	}

	dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping db: %w", err)
	}

	if err := execSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: run schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// execSchema strips line comments (which may themselves contain ";"), splits the
// embedded SQL script on ";" and executes each non-empty statement individually
// so that every DDL and the seed INSERT are applied idempotently on every
// startup.
func execSchema(db *sql.DB) error {
	// Drop "--" line comments so semicolons inside comments don't corrupt the
	// statement split below.
	var sb strings.Builder
	for _, line := range strings.Split(schemaSQL, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	stmts := strings.Split(sb.String(), ";")
	for _, raw := range stmts {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec statement %q: %w", truncate(stmt, 60), err)
		}
	}
	return nil
}

// mapErr converts well-known database errors to domain sentinels.
func mapErr(err error, op string) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return fmt.Errorf("%s: %w", op, errNotFound)
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "constraint") {
		return fmt.Errorf("%s: %w", op, errConflict)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// truncate returns the first n bytes of s, used for error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}


package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Nolanndev/dev-natif/internal/domain"
)

const defaultEventLimit = 200

// RecordEvent persists an audit/history event.
func (s *Store) RecordEvent(ctx context.Context, e *domain.Event) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.Level == "" {
		e.Level = domain.LevelInfo
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events(id, created_at, level, type, project_id, deployment_id, message, details)
		 VALUES(?,?,?,?,?,?,?,?)`,
		e.ID, e.CreatedAt, e.Level, e.Type, e.ProjectID, e.DeploymentID, e.Message, e.Details)
	if err != nil {
		return mapErr(err, "record event")
	}
	return nil
}

// ListEvents returns events matching the filter, newest first.
func (s *Store) ListEvents(ctx context.Context, f domain.EventFilter) ([]*domain.Event, error) {
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = defaultEventLimit
	}
	query := `SELECT id, created_at, level, type, project_id, deployment_id, message, details FROM events WHERE 1=1`
	args := []any{}
	if f.ProjectID != "" {
		query += " AND project_id = ?"
		args = append(args, f.ProjectID)
	}
	if f.DeploymentID != "" {
		query += " AND deployment_id = ?"
		args = append(args, f.DeploymentID)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err, "list events")
	}
	defer rows.Close()

	var out []*domain.Event
	for rows.Next() {
		e := &domain.Event{}
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Level, &e.Type, &e.ProjectID, &e.DeploymentID, &e.Message, &e.Details); err != nil {
			return nil, mapErr(err, "scan event")
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PurgeEventsBefore deletes events older than t and returns the number removed.
func (s *Store) PurgeEventsBefore(ctx context.Context, t time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE created_at < ?`, t.UTC())
	if err != nil {
		return 0, mapErr(err, "purge events")
	}
	n, _ := res.RowsAffected()
	return n, nil
}

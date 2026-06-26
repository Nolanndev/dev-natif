package store

import (
	"context"
	"fmt"

	"github.com/actigraph/dev-natif/internal/domain"
)

// GetServer returns the server with the given id, or domain.ErrNotFound.
func (s *Store) GetServer(ctx context.Context, id string) (*domain.Server, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, host, status FROM servers WHERE id=?`, id)
	srv, err := scanServer(row)
	if err != nil {
		return nil, mapErr(err, "GetServer")
	}
	return srv, nil
}

// ListServers returns all configured Docker Engine targets.
func (s *Store) ListServers(ctx context.Context) ([]*domain.Server, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, host, status FROM servers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("ListServers: %w", err)
	}
	defer rows.Close()
	var out []*domain.Server
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("ListServers scan: %w", err)
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// DefaultServer returns the well-known local server row (id='local'), seeded
// by the schema migration.
func (s *Store) DefaultServer(ctx context.Context) (*domain.Server, error) {
	return s.GetServer(ctx, domain.LocalServerID)
}

// scanServer reads one server row from any scanner (row or rows).
func scanServer(row scanner) (*domain.Server, error) {
	srv := &domain.Server{}
	err := row.Scan(&srv.ID, &srv.Name, &srv.Host, &srv.Status)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

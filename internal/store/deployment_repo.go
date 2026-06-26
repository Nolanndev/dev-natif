package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Nolanndev/dev-natif/internal/domain"
)

// ----------------------------------------------------------------------------
// Deployment
// ----------------------------------------------------------------------------

// CreateDeployment inserts a deployment row with its override rows.
func (s *Store) CreateDeployment(ctx context.Context, d *domain.Deployment) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = now
	}
	if d.Status == "" {
		d.Status = domain.StatusPending
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("CreateDeployment: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx,
		`INSERT INTO deployments (id, project_id, server_id, name, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ProjectID, d.ServerID, d.Name, string(d.Status), d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return mapErr(err, "CreateDeployment")
	}

	if err := insertOverrides(ctx, tx, d.ID, d.Overrides); err != nil {
		return err
	}

	return tx.Commit()
}

// GetDeployment returns a deployment with Overrides and Containers populated.
func (s *Store) GetDeployment(ctx context.Context, id string) (*domain.Deployment, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, server_id, name, status, created_at, updated_at
		 FROM deployments WHERE id=?`, id)
	d, err := scanDeployment(row)
	if err != nil {
		return nil, mapErr(err, "GetDeployment")
	}
	if err := s.hydrateDeployment(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// ListDeployments returns all deployments without nested children.
func (s *Store) ListDeployments(ctx context.Context) ([]*domain.Deployment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, server_id, name, status, created_at, updated_at
		 FROM deployments ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListDeployments: %w", err)
	}
	defer rows.Close()
	var out []*domain.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("ListDeployments scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListDeploymentsByProject returns the deployments of one project, newest first
// (the project's deployment history).
func (s *Store) ListDeploymentsByProject(ctx context.Context, projectID string) ([]*domain.Deployment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, server_id, name, status, created_at, updated_at
		 FROM deployments WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("ListDeploymentsByProject: %w", err)
	}
	defer rows.Close()
	var out []*domain.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("ListDeploymentsByProject scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateDeployment updates the mutable fields of a deployment (name, status).
// UpdatedAt is refreshed to now.
func (s *Store) UpdateDeployment(ctx context.Context, d *domain.Deployment) error {
	d.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE deployments SET name=?, status=?, updated_at=? WHERE id=?`,
		d.Name, string(d.Status), d.UpdatedAt, d.ID,
	)
	if err != nil {
		return mapErr(err, "UpdateDeployment")
	}
	return requireOneRow(res, "UpdateDeployment")
}

// DeleteDeployment removes the deployment row; cascades via FK.
func (s *Store) DeleteDeployment(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM deployments WHERE id=?`, id)
	if err != nil {
		return mapErr(err, "DeleteDeployment")
	}
	return requireOneRow(res, "DeleteDeployment")
}

// ----------------------------------------------------------------------------
// Containers
// ----------------------------------------------------------------------------

// SaveContainers replaces the tracked container set of a deployment atomically.
func (s *Store) SaveContainers(ctx context.Context, deploymentID string, cs []*domain.Container) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("SaveContainers: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM containers WHERE deployment_id=?`, deploymentID); err != nil {
		return fmt.Errorf("SaveContainers: delete existing: %w", err)
	}

	now := time.Now().UTC()
	for _, c := range cs {
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
		c.DeploymentID = deploymentID
		if c.CreatedAt.IsZero() {
			c.CreatedAt = now
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO containers
			 (id, deployment_id, service_id, docker_container_id, name, state, health, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, deploymentID, c.ServiceID, c.DockerContainerID,
			c.Name, c.State, c.Health, c.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("SaveContainers: insert container: %w", err)
		}
	}
	return tx.Commit()
}

// ListContainers returns all container rows for a deployment.
func (s *Store) ListContainers(ctx context.Context, deploymentID string) ([]*domain.Container, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, deployment_id, service_id, docker_container_id, name, state, health, created_at
		 FROM containers WHERE deployment_id=? ORDER BY created_at`, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("ListContainers: %w", err)
	}
	defer rows.Close()
	var out []*domain.Container
	for rows.Next() {
		c := &domain.Container{}
		if err := rows.Scan(&c.ID, &c.DeploymentID, &c.ServiceID,
			&c.DockerContainerID, &c.Name, &c.State, &c.Health, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListContainers scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ----------------------------------------------------------------------------
// Overrides
// ----------------------------------------------------------------------------

func insertOverrides(ctx context.Context, tx execer, deploymentID string, overrides []*domain.DeploymentOverride) error {
	for _, o := range overrides {
		if o.ID == "" {
			o.ID = uuid.NewString()
		}
		o.DeploymentID = deploymentID
		_, err := tx.ExecContext(ctx,
			`INSERT INTO deployment_overrides (id, deployment_id, kind, target_ref, key, value)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			o.ID, deploymentID, string(o.Kind), o.TargetRef, o.Key, o.Value,
		)
		if err != nil {
			return mapErr(err, "insertOverrides")
		}
	}
	return nil
}

func (s *Store) listOverrides(ctx context.Context, deploymentID string) ([]*domain.DeploymentOverride, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, deployment_id, kind, target_ref, key, value
		 FROM deployment_overrides WHERE deployment_id=?`, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("listOverrides: %w", err)
	}
	defer rows.Close()
	var out []*domain.DeploymentOverride
	for rows.Next() {
		o := &domain.DeploymentOverride{}
		var kind string
		if err := rows.Scan(&o.ID, &o.DeploymentID, &kind, &o.TargetRef, &o.Key, &o.Value); err != nil {
			return nil, fmt.Errorf("listOverrides scan: %w", err)
		}
		o.Kind = domain.OverrideKind(kind)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ----------------------------------------------------------------------------
// Hydration
// ----------------------------------------------------------------------------

func (s *Store) hydrateDeployment(ctx context.Context, d *domain.Deployment) error {
	overrides, err := s.listOverrides(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("hydrateDeployment overrides: %w", err)
	}
	d.Overrides = overrides

	containers, err := s.ListContainers(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("hydrateDeployment containers: %w", err)
	}
	d.Containers = containers
	return nil
}

// ----------------------------------------------------------------------------
// Scan helper
// ----------------------------------------------------------------------------

func scanDeployment(row scanner) (*domain.Deployment, error) {
	d := &domain.Deployment{}
	var status string
	err := row.Scan(&d.ID, &d.ProjectID, &d.ServerID, &d.Name, &status, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	d.Status = domain.DeploymentStatus(status)
	return d, nil
}

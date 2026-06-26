package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Nolanndev/dev-natif/internal/domain"
)

// ----------------------------------------------------------------------------
// Project
// ----------------------------------------------------------------------------

// CreateProject inserts a new project row. Sets ID and timestamps when empty.
func (s *Store) CreateProject(ctx context.Context, p *domain.Project) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	return mapErr(err, "CreateProject")
}

// GetProject returns a project with its Services and Volumes fully populated.
func (s *Store) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM projects WHERE id = ?`, id)
	p, err := scanProject(row)
	if err != nil {
		return nil, mapErr(err, "GetProject")
	}
	if err := s.hydrateProject(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListProjects returns all projects without nested children (lightweight list).
func (s *Store) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, created_at, updated_at FROM projects ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListProjects: %w", err)
	}
	defer rows.Close()
	var out []*domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("ListProjects scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdateProject updates mutable fields. UpdatedAt is set to now.
func (s *Store) UpdateProject(ctx context.Context, p *domain.Project) error {
	p.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name=?, description=?, updated_at=? WHERE id=?`,
		p.Name, p.Description, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return mapErr(err, "UpdateProject")
	}
	return requireOneRow(res, "UpdateProject")
}

// DeleteProject removes the project row; cascades to services/volumes via FK.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return mapErr(err, "DeleteProject")
	}
	return requireOneRow(res, "DeleteProject")
}

// ----------------------------------------------------------------------------
// Service
// ----------------------------------------------------------------------------

// AddService inserts a service and all its nested Env, Ports, Mounts and
// DependsOn rows in a single transaction.
func (s *Store) AddService(ctx context.Context, svc *domain.Service) error {
	if svc.ID == "" {
		svc.ID = uuid.NewString()
	}
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = time.Now().UTC()
	}
	cmdJSON, err := marshalCommand(svc.Command)
	if err != nil {
		return fmt.Errorf("AddService: marshal command: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AddService: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx,
		`INSERT INTO services
		 (id, project_id, name, image, build_context, build_dockerfile,
		  command, restart_policy, replicas, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		svc.ID, svc.ProjectID, svc.Name,
		svc.Image, svc.BuildContext, svc.BuildDockerfile,
		cmdJSON, svc.RestartPolicy, svc.Replicas, svc.CreatedAt,
	)
	if err != nil {
		return mapErr(err, "AddService insert service")
	}

	if err := insertServiceEnv(ctx, tx, svc.ID, svc.Env); err != nil {
		return err
	}
	if err := insertServicePorts(ctx, tx, svc.ID, svc.Ports); err != nil {
		return err
	}
	if err := insertServiceMounts(ctx, tx, svc.ID, svc.Mounts); err != nil {
		return err
	}
	if err := insertServiceDeps(ctx, tx, svc.ID, svc.DependsOn); err != nil {
		return err
	}

	return tx.Commit()
}

// GetService returns a fully hydrated service (env, ports, mounts, deps).
func (s *Store) GetService(ctx context.Context, id string) (*domain.Service, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, image, build_context, build_dockerfile,
		        command, restart_policy, replicas, created_at
		 FROM services WHERE id = ?`, id)
	svc, err := scanService(row)
	if err != nil {
		return nil, mapErr(err, "GetService")
	}
	if err := s.hydrateService(ctx, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

// ListServices returns all services for a project, each fully hydrated.
func (s *Store) ListServices(ctx context.Context, projectID string) ([]*domain.Service, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, image, build_context, build_dockerfile,
		        command, restart_policy, replicas, created_at
		 FROM services WHERE project_id = ? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, fmt.Errorf("ListServices: %w", err)
	}
	defer rows.Close()
	var out []*domain.Service
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("ListServices scan: %w", err)
		}
		if err := s.hydrateService(ctx, svc); err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

// UpdateService replaces the service row and all its nested rows.
func (s *Store) UpdateService(ctx context.Context, svc *domain.Service) error {
	cmdJSON, err := marshalCommand(svc.Command)
	if err != nil {
		return fmt.Errorf("UpdateService: marshal command: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("UpdateService: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx,
		`UPDATE services SET name=?, image=?, build_context=?, build_dockerfile=?,
		  command=?, restart_policy=?, replicas=? WHERE id=?`,
		svc.Name, svc.Image, svc.BuildContext, svc.BuildDockerfile,
		cmdJSON, svc.RestartPolicy, svc.Replicas, svc.ID,
	)
	if err != nil {
		return mapErr(err, "UpdateService")
	}
	if err := requireOneRow(res, "UpdateService"); err != nil {
		return err
	}

	// Replace nested rows — delete all child rows first, then re-insert.
	for _, tbl := range []string{"service_env", "service_ports", "service_mounts", "service_deps"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+tbl+" WHERE service_id=?", svc.ID); err != nil {
			return fmt.Errorf("UpdateService: clear %s: %w", tbl, err)
		}
	}

	if err := insertServiceEnv(ctx, tx, svc.ID, svc.Env); err != nil {
		return err
	}
	if err := insertServicePorts(ctx, tx, svc.ID, svc.Ports); err != nil {
		return err
	}
	if err := insertServiceMounts(ctx, tx, svc.ID, svc.Mounts); err != nil {
		return err
	}
	if err := insertServiceDeps(ctx, tx, svc.ID, svc.DependsOn); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteService removes the service row; cascades to nested tables via FK.
func (s *Store) DeleteService(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id=?`, id)
	if err != nil {
		return mapErr(err, "DeleteService")
	}
	return requireOneRow(res, "DeleteService")
}

// ----------------------------------------------------------------------------
// Volume
// ----------------------------------------------------------------------------

// AddVolume inserts a volume row, generating an ID when absent.
func (s *Store) AddVolume(ctx context.Context, v *domain.Volume) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	if v.Driver == "" {
		v.Driver = "local"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO volumes (id, project_id, name, driver) VALUES (?, ?, ?, ?)`,
		v.ID, v.ProjectID, v.Name, v.Driver,
	)
	return mapErr(err, "AddVolume")
}

// GetVolume returns the volume or domain.ErrNotFound.
func (s *Store) GetVolume(ctx context.Context, id string) (*domain.Volume, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, driver FROM volumes WHERE id=?`, id)
	v := &domain.Volume{}
	err := row.Scan(&v.ID, &v.ProjectID, &v.Name, &v.Driver)
	if err != nil {
		return nil, mapErr(err, "GetVolume")
	}
	return v, nil
}

// ListVolumes returns all volumes for a project.
func (s *Store) ListVolumes(ctx context.Context, projectID string) ([]*domain.Volume, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, driver FROM volumes WHERE project_id=?`, projectID)
	if err != nil {
		return nil, fmt.Errorf("ListVolumes: %w", err)
	}
	defer rows.Close()
	var out []*domain.Volume
	for rows.Next() {
		v := &domain.Volume{}
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Name, &v.Driver); err != nil {
			return nil, fmt.Errorf("ListVolumes scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteVolume removes the volume row.
func (s *Store) DeleteVolume(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM volumes WHERE id=?`, id)
	if err != nil {
		return mapErr(err, "DeleteVolume")
	}
	return requireOneRow(res, "DeleteVolume")
}

// ----------------------------------------------------------------------------
// Hydration helpers
// ----------------------------------------------------------------------------

// hydrateProject fetches and attaches services and volumes to p.
func (s *Store) hydrateProject(ctx context.Context, p *domain.Project) error {
	svcs, err := s.ListServices(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("hydrateProject services: %w", err)
	}
	p.Services = svcs

	vols, err := s.ListVolumes(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("hydrateProject volumes: %w", err)
	}
	p.Volumes = vols
	return nil
}

// hydrateService fetches env, ports, mounts and depends_on for a service.
func (s *Store) hydrateService(ctx context.Context, svc *domain.Service) error {
	env, err := s.listServiceEnv(ctx, svc.ID)
	if err != nil {
		return err
	}
	svc.Env = env

	ports, err := s.listServicePorts(ctx, svc.ID)
	if err != nil {
		return err
	}
	svc.Ports = ports

	mounts, err := s.listServiceMounts(ctx, svc.ID)
	if err != nil {
		return err
	}
	svc.Mounts = mounts

	deps, err := s.listServiceDeps(ctx, svc.ID)
	if err != nil {
		return err
	}
	svc.DependsOn = deps
	return nil
}

// listServiceEnv returns all env rows for serviceID.
func (s *Store) listServiceEnv(ctx context.Context, serviceID string) ([]*domain.ServiceEnv, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, service_id, key, value, is_variable FROM service_env WHERE service_id=?`,
		serviceID)
	if err != nil {
		return nil, fmt.Errorf("listServiceEnv: %w", err)
	}
	defer rows.Close()
	var out []*domain.ServiceEnv
	for rows.Next() {
		e := &domain.ServiceEnv{}
		var isVar int
		if err := rows.Scan(&e.ID, &e.ServiceID, &e.Key, &e.Value, &isVar); err != nil {
			return nil, fmt.Errorf("listServiceEnv scan: %w", err)
		}
		e.IsVariable = isVar == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// listServicePorts returns all port rows for serviceID.
func (s *Store) listServicePorts(ctx context.Context, serviceID string) ([]*domain.ServicePort, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, service_id, container_port, host_port, protocol, is_variable
		 FROM service_ports WHERE service_id=?`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("listServicePorts: %w", err)
	}
	defer rows.Close()
	var out []*domain.ServicePort
	for rows.Next() {
		p := &domain.ServicePort{}
		var isVar int
		if err := rows.Scan(&p.ID, &p.ServiceID, &p.ContainerPort, &p.HostPort, &p.Protocol, &isVar); err != nil {
			return nil, fmt.Errorf("listServicePorts scan: %w", err)
		}
		p.IsVariable = isVar == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// listServiceMounts returns all mount rows for serviceID.
func (s *Store) listServiceMounts(ctx context.Context, serviceID string) ([]*domain.ServiceVolume, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, service_id, volume_id, target, read_only
		 FROM service_mounts WHERE service_id=?`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("listServiceMounts: %w", err)
	}
	defer rows.Close()
	var out []*domain.ServiceVolume
	for rows.Next() {
		m := &domain.ServiceVolume{}
		var ro int
		if err := rows.Scan(&m.ID, &m.ServiceID, &m.VolumeID, &m.Target, &ro); err != nil {
			return nil, fmt.Errorf("listServiceMounts scan: %w", err)
		}
		m.ReadOnly = ro == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// listServiceDeps returns the slice of service IDs that serviceID depends on.
func (s *Store) listServiceDeps(ctx context.Context, serviceID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT depends_on_id FROM service_deps WHERE service_id=?`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("listServiceDeps: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, fmt.Errorf("listServiceDeps scan: %w", err)
		}
		out = append(out, depID)
	}
	return out, rows.Err()
}

// ----------------------------------------------------------------------------
// Nested-row insert helpers (operate on a *sql.Tx)
// ----------------------------------------------------------------------------

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertServiceEnv(ctx context.Context, tx execer, serviceID string, envs []*domain.ServiceEnv) error {
	for _, e := range envs {
		if e.ID == "" {
			e.ID = uuid.NewString()
		}
		e.ServiceID = serviceID
		isVar := 0
		if e.IsVariable {
			isVar = 1
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO service_env (id, service_id, key, value, is_variable) VALUES (?, ?, ?, ?, ?)`,
			e.ID, serviceID, e.Key, e.Value, isVar)
		if err != nil {
			return mapErr(err, "insertServiceEnv")
		}
	}
	return nil
}

func insertServicePorts(ctx context.Context, tx execer, serviceID string, ports []*domain.ServicePort) error {
	for _, p := range ports {
		if p.ID == "" {
			p.ID = uuid.NewString()
		}
		p.ServiceID = serviceID
		isVar := 0
		if p.IsVariable {
			isVar = 1
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO service_ports (id, service_id, container_port, host_port, protocol, is_variable)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, serviceID, p.ContainerPort, p.HostPort, p.Protocol, isVar)
		if err != nil {
			return mapErr(err, "insertServicePorts")
		}
	}
	return nil
}

func insertServiceMounts(ctx context.Context, tx execer, serviceID string, mounts []*domain.ServiceVolume) error {
	for _, m := range mounts {
		if m.ID == "" {
			m.ID = uuid.NewString()
		}
		m.ServiceID = serviceID
		ro := 0
		if m.ReadOnly {
			ro = 1
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO service_mounts (id, service_id, volume_id, target, read_only) VALUES (?, ?, ?, ?, ?)`,
			m.ID, serviceID, m.VolumeID, m.Target, ro)
		if err != nil {
			return mapErr(err, "insertServiceMounts")
		}
	}
	return nil
}

func insertServiceDeps(ctx context.Context, tx execer, serviceID string, deps []string) error {
	for _, depID := range deps {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO service_deps (service_id, depends_on_id) VALUES (?, ?)`,
			serviceID, depID)
		if err != nil {
			return mapErr(err, "insertServiceDeps")
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// Scan helpers
// ----------------------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (*domain.Project, error) {
	p := &domain.Project{}
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func scanService(row scanner) (*domain.Service, error) {
	svc := &domain.Service{}
	var cmdJSON string
	err := row.Scan(
		&svc.ID, &svc.ProjectID, &svc.Name,
		&svc.Image, &svc.BuildContext, &svc.BuildDockerfile,
		&cmdJSON, &svc.RestartPolicy, &svc.Replicas, &svc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	svc.Command, err = unmarshalCommand(cmdJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal command for service %s: %w", svc.ID, err)
	}
	return svc, nil
}

// ----------------------------------------------------------------------------
// JSON helpers for Service.Command
// ----------------------------------------------------------------------------

func marshalCommand(cmd []string) (string, error) {
	if len(cmd) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(cmd)
	return string(b), err
}

func unmarshalCommand(s string) ([]string, error) {
	if s == "" || s == "[]" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// Generic helper
// ----------------------------------------------------------------------------

// requireOneRow returns domain.ErrNotFound when the statement affected no rows.
func requireOneRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", op, errNotFound)
	}
	return nil
}

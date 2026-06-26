package service

import (
	"context"
	"strings"

	"github.com/actigraph/dev-natif/internal/domain"
)

// ProjectService implements the use cases around the abstract description of an
// infrastructure (projects, services, volumes, environment variables).
type ProjectService struct {
	repo domain.ProjectRepository
}

// NewProjectService wires the project use cases to a repository.
func NewProjectService(repo domain.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

// ---- Projects --------------------------------------------------------------

func (s *ProjectService) CreateProject(ctx context.Context, p *domain.Project) error {
	if strings.TrimSpace(p.Name) == "" {
		return validation("project name is required")
	}
	return s.repo.CreateProject(ctx, p)
}

func (s *ProjectService) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	return s.repo.GetProject(ctx, id)
}

func (s *ProjectService) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	return s.repo.ListProjects(ctx)
}

func (s *ProjectService) UpdateProject(ctx context.Context, p *domain.Project) error {
	if strings.TrimSpace(p.Name) == "" {
		return validation("project name is required")
	}
	return s.repo.UpdateProject(ctx, p)
}

func (s *ProjectService) DeleteProject(ctx context.Context, id string) error {
	return s.repo.DeleteProject(ctx, id)
}

// ---- Services --------------------------------------------------------------

// AddService validates and attaches a service to a project.
func (s *ProjectService) AddService(ctx context.Context, projectID string, svc *domain.Service) error {
	if _, err := s.repo.GetProject(ctx, projectID); err != nil {
		return err
	}
	if strings.TrimSpace(svc.Name) == "" {
		return validation("service name is required")
	}
	if strings.TrimSpace(svc.Image) == "" && strings.TrimSpace(svc.BuildContext) == "" {
		return validation("service %q must define either an image or a build context", svc.Name)
	}
	if svc.Replicas <= 0 {
		svc.Replicas = 1
	}
	svc.ProjectID = projectID
	return s.repo.AddService(ctx, svc)
}

func (s *ProjectService) GetService(ctx context.Context, id string) (*domain.Service, error) {
	return s.repo.GetService(ctx, id)
}

func (s *ProjectService) ListServices(ctx context.Context, projectID string) ([]*domain.Service, error) {
	return s.repo.ListServices(ctx, projectID)
}

func (s *ProjectService) UpdateService(ctx context.Context, svc *domain.Service) error {
	if strings.TrimSpace(svc.Name) == "" {
		return validation("service name is required")
	}
	if svc.Replicas <= 0 {
		svc.Replicas = 1
	}
	return s.repo.UpdateService(ctx, svc)
}

func (s *ProjectService) DeleteService(ctx context.Context, id string) error {
	return s.repo.DeleteService(ctx, id)
}

// ---- Volumes ---------------------------------------------------------------

func (s *ProjectService) AddVolume(ctx context.Context, projectID string, v *domain.Volume) error {
	if _, err := s.repo.GetProject(ctx, projectID); err != nil {
		return err
	}
	if strings.TrimSpace(v.Name) == "" {
		return validation("volume name is required")
	}
	if v.Driver == "" {
		v.Driver = "local"
	}
	v.ProjectID = projectID
	return s.repo.AddVolume(ctx, v)
}

func (s *ProjectService) ListVolumes(ctx context.Context, projectID string) ([]*domain.Volume, error) {
	return s.repo.ListVolumes(ctx, projectID)
}

func (s *ProjectService) DeleteVolume(ctx context.Context, id string) error {
	return s.repo.DeleteVolume(ctx, id)
}

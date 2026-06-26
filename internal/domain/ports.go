package domain

import (
	"context"
	"errors"
)

// Sentinel errors used across layers. Adapters return these so the service and
// HTTP layers can map them to proper status codes without leaking infra types.
var (
	ErrNotFound      = errors.New("resource not found")
	ErrConflict      = errors.New("resource conflict")
	ErrValidation    = errors.New("validation error")
	ErrDockerEngine  = errors.New("docker engine error")
	ErrDependencyCyc = errors.New("dependency cycle detected")
)

// ----------------------------------------------------------------------------
// Persistence ports — implemented by internal/store (SQLite).
// ----------------------------------------------------------------------------

// ProjectRepository persists projects and their constituent elements.
type ProjectRepository interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProject(ctx context.Context, id string) (*Project, error)
	ListProjects(ctx context.Context) ([]*Project, error)
	UpdateProject(ctx context.Context, p *Project) error
	DeleteProject(ctx context.Context, id string) error

	AddService(ctx context.Context, s *Service) error
	GetService(ctx context.Context, id string) (*Service, error)
	ListServices(ctx context.Context, projectID string) ([]*Service, error)
	UpdateService(ctx context.Context, s *Service) error
	DeleteService(ctx context.Context, id string) error

	AddVolume(ctx context.Context, v *Volume) error
	GetVolume(ctx context.Context, id string) (*Volume, error)
	ListVolumes(ctx context.Context, projectID string) ([]*Volume, error)
	DeleteVolume(ctx context.Context, id string) error
}

// DeploymentRepository persists deployments, their overrides and runtime
// container tracking rows.
type DeploymentRepository interface {
	CreateDeployment(ctx context.Context, d *Deployment) error
	GetDeployment(ctx context.Context, id string) (*Deployment, error)
	ListDeployments(ctx context.Context) ([]*Deployment, error)
	UpdateDeployment(ctx context.Context, d *Deployment) error
	DeleteDeployment(ctx context.Context, id string) error

	// SaveContainers replaces the tracked container set of a deployment.
	SaveContainers(ctx context.Context, deploymentID string, cs []*Container) error
	ListContainers(ctx context.Context, deploymentID string) ([]*Container, error)
}

// ServerRepository exposes the configured Docker Engine targets.
type ServerRepository interface {
	GetServer(ctx context.Context, id string) (*Server, error)
	ListServers(ctx context.Context) ([]*Server, error)
	DefaultServer(ctx context.Context) (*Server, error)
}

// ----------------------------------------------------------------------------
// Docker Engine port — implemented by internal/docker (Docker SDK).
// ----------------------------------------------------------------------------

// ImagePullSpec describes an image to pull.
type ImagePullSpec struct {
	Ref      string // e.g. "nginx:1.27" or "registry/repo:tag"
	AuthB64  string // optional base64 X-Registry-Auth; empty for public images
}

// ImageBuildSpec describes an image build from a context directory.
type ImageBuildSpec struct {
	ContextDir string // path to build context (server-side / accessible to engine)
	Dockerfile string // relative path inside context, default "Dockerfile"
	Tag        string // resulting image tag
}

// PortBinding maps a container port to a host port.
type PortBinding struct {
	ContainerPort int
	HostPort      int    // 0 lets Docker choose a random host port
	Protocol      string // "tcp" | "udp"
}

// Mount binds a named volume into a container.
type Mount struct {
	VolumeName string
	Target     string
	ReadOnly   bool
}

// ContainerSpec is the engine-agnostic description used to create a container.
type ContainerSpec struct {
	Name          string
	Image         string
	Env           []string // "KEY=VALUE"
	Cmd           []string
	Labels        map[string]string
	Ports         []PortBinding
	Mounts        []Mount
	RestartPolicy string
}

// ContainerInfo is the live state of a container as reported by the engine.
type ContainerInfo struct {
	ID     string
	Name   string
	Image  string
	State  string            // "running", "exited", "created", "paused"...
	Health string            // "healthy", "unhealthy", "starting", "none"
	Labels map[string]string
}

// DockerEngine abstracts the Docker Engine. A concrete instance is bound to one
// Server, so Phase 2 simply constructs one DockerEngine per server.
type DockerEngine interface {
	Ping(ctx context.Context) error

	PullImage(ctx context.Context, spec ImagePullSpec) error
	BuildImage(ctx context.Context, spec ImageBuildSpec) error

	CreateContainer(ctx context.Context, spec ContainerSpec) (id string, err error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string, force bool) error
	InspectContainer(ctx context.Context, id string) (ContainerInfo, error)
	ListContainersByLabel(ctx context.Context, labels map[string]string) ([]ContainerInfo, error)

	CreateVolume(ctx context.Context, name string, labels map[string]string) error
	RemoveVolume(ctx context.Context, name string, force bool) error
}

// Label keys used to tag every engine resource created by the API. They make
// supervision and cleanup label-driven (no state drift).
const (
	LabelManaged    = "com.devnatif.managed"    // "true"
	LabelProject    = "com.devnatif.project"    // project ID
	LabelDeployment = "com.devnatif.deployment" // deployment ID
	LabelService    = "com.devnatif.service"    // service ID
)

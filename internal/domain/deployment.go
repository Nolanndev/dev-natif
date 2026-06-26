package domain

import "time"

// DeploymentStatus is the aggregated lifecycle state of a deployment, computed
// from the live state of its containers reported by the Docker Engine.
type DeploymentStatus string

const (
	StatusPending          DeploymentStatus = "pending"           // created, never deployed
	StatusRunning          DeploymentStatus = "running"           // all expected containers up
	StatusPartiallyRunning DeploymentStatus = "partially-running" // some up, some down
	StatusNotRunning       DeploymentStatus = "not-running"       // none up
	StatusFailed           DeploymentStatus = "failed"            // last operation errored
)

// Deployment is the concrete materialisation of a Project on a Server — the
// result of "docker-compose up". It carries everything environment-specific
// (overrides for variable env / ports) so the Project itself stays abstract.
type Deployment struct {
	ID        string           `json:"id"`
	ProjectID string           `json:"project_id"`
	ServerID  string           `json:"server_id"`
	Name      string           `json:"name"`
	Status    DeploymentStatus `json:"status"`

	Overrides  []*DeploymentOverride `json:"overrides,omitempty"`
	Containers []*Container          `json:"containers,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OverrideKind identifies what a DeploymentOverride targets.
type OverrideKind string

const (
	OverrideEnv  OverrideKind = "env"  // override a service env value (TargetRef = service ID, Key = env key)
	OverridePort OverrideKind = "port" // override a host port (TargetRef = service ID, Key = "containerPort/proto")
)

// DeploymentOverride supplies a concrete value for an element the project marked
// as variable (IsVariable). This is what makes one project reusable across many
// deployments (different domains, DB params, ports...).
type DeploymentOverride struct {
	ID           string       `json:"id"`
	DeploymentID string       `json:"deployment_id"`
	Kind         OverrideKind `json:"kind"`
	TargetRef    string       `json:"target_ref"` // service ID the override applies to
	Key          string       `json:"key"`
	Value        string       `json:"value"`
}

// Container tracks one running container instance bound to a service of a
// deployment. It is the runtime mirror used for supervision.
type Container struct {
	ID                string    `json:"id"`
	DeploymentID      string    `json:"deployment_id"`
	ServiceID         string    `json:"service_id"`
	DockerContainerID string    `json:"docker_container_id"`
	Name              string    `json:"name"`
	State             string    `json:"state"`  // "running", "exited", "created"...
	Health            string    `json:"health"` // "healthy", "unhealthy", "none"...
	CreatedAt         time.Time `json:"created_at"`
}

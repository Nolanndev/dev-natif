// Package domain holds the core entities and the ports (interfaces) of the
// application. It must NOT import any infrastructure package (gin, docker SDK,
// sqlite). Adapters in internal/store, internal/docker and internal/http depend
// on these contracts — never the other way around.
package domain

import "time"

// Project is the abstract, reusable description of a multi-container software
// infrastructure — the equivalent of a docker-compose file. A Project is
// environment-agnostic: everything specific to a runtime target lives in a
// Deployment.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Populated by the service layer when a detailed view is requested.
	Services []*Service `json:"services,omitempty"`
	Volumes  []*Volume  `json:"volumes,omitempty"`
}

// Service describes one logical service of a project (≈ a service entry in
// docker-compose). It produces one or more containers at deployment time.
type Service struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`

	// Image OR Build must be provided. Image wins if both are set.
	Image           string `json:"image,omitempty"`
	BuildContext    string `json:"build_context,omitempty"`
	BuildDockerfile string `json:"build_dockerfile,omitempty"`

	Command       []string `json:"command,omitempty"`
	RestartPolicy string   `json:"restart_policy,omitempty"` // "", "no", "always", "on-failure", "unless-stopped"

	// Replicas is the desired number of container instances. MVP keeps it at 1;
	// the field exists so Phase 2 scaling needs no schema change.
	Replicas int `json:"replicas"`

	Env          []*ServiceEnv    `json:"env,omitempty"`
	Ports        []*ServicePort   `json:"ports,omitempty"`
	Mounts       []*ServiceVolume `json:"mounts,omitempty"`
	DependsOn    []string         `json:"depends_on,omitempty"` // service IDs this service depends on

	CreatedAt time.Time `json:"created_at"`
}

// ServiceEnv is an environment variable attached to a service.
// When IsVariable is true the value is a placeholder that MUST be supplied by a
// Deployment override; the project only declares that the variable exists.
type ServiceEnv struct {
	ID         string `json:"id"`
	ServiceID  string `json:"service_id"`
	Key        string `json:"key"`
	Value      string `json:"value"`       // default value (may be empty for variables)
	IsVariable bool   `json:"is_variable"` // must be provided at deployment time
}

// ServicePort declares a port mapping. HostPort may be left variable so the
// concrete value is chosen per-deployment.
type ServicePort struct {
	ID            string `json:"id"`
	ServiceID     string `json:"service_id"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`           // 0 when IsVariable
	Protocol      string `json:"protocol"`            // "tcp" (default) | "udp"
	IsVariable    bool   `json:"is_variable"`
}

// ServiceVolume binds a project Volume into a service's container.
type ServiceVolume struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	VolumeID  string `json:"volume_id"`
	Target    string `json:"target"` // mount path inside the container
	ReadOnly  bool   `json:"read_only"`
}

// Volume is a named, project-scoped persistent volume.
type Volume struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Driver    string `json:"driver"` // defaults to "local"
}

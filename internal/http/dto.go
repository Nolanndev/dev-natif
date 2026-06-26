package httpapi

import "github.com/actigraph/dev-natif/internal/domain"

// ---- Project DTOs ----------------------------------------------------------

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ---- Service DTOs ----------------------------------------------------------

type envDTO struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	IsVariable bool   `json:"is_variable"`
}

type portDTO struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
	IsVariable    bool   `json:"is_variable"`
}

type mountDTO struct {
	VolumeID string `json:"volume_id"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

type serviceRequest struct {
	Name            string     `json:"name"`
	Image           string     `json:"image"`
	BuildContext    string     `json:"build_context"`
	BuildDockerfile string     `json:"build_dockerfile"`
	Command         []string   `json:"command"`
	RestartPolicy   string     `json:"restart_policy"`
	Replicas        int        `json:"replicas"`
	Env             []envDTO   `json:"env"`
	Ports           []portDTO  `json:"ports"`
	Mounts          []mountDTO `json:"mounts"`
	DependsOn       []string   `json:"depends_on"`
}

func (r serviceRequest) toDomain() *domain.Service {
	svc := &domain.Service{
		Name:            r.Name,
		Image:           r.Image,
		BuildContext:    r.BuildContext,
		BuildDockerfile: r.BuildDockerfile,
		Command:         r.Command,
		RestartPolicy:   r.RestartPolicy,
		Replicas:        r.Replicas,
		DependsOn:       r.DependsOn,
	}
	for _, e := range r.Env {
		svc.Env = append(svc.Env, &domain.ServiceEnv{Key: e.Key, Value: e.Value, IsVariable: e.IsVariable})
	}
	for _, p := range r.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		svc.Ports = append(svc.Ports, &domain.ServicePort{
			ContainerPort: p.ContainerPort, HostPort: p.HostPort, Protocol: proto, IsVariable: p.IsVariable,
		})
	}
	for _, m := range r.Mounts {
		svc.Mounts = append(svc.Mounts, &domain.ServiceVolume{VolumeID: m.VolumeID, Target: m.Target, ReadOnly: m.ReadOnly})
	}
	return svc
}

// ---- Volume DTO ------------------------------------------------------------

type volumeRequest struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// ---- Deployment DTOs -------------------------------------------------------

type overrideDTO struct {
	Kind      string `json:"kind"`       // "env" | "port"
	TargetRef string `json:"target_ref"` // service ID
	Key       string `json:"key"`
	Value     string `json:"value"`
}

type createDeploymentRequest struct {
	Name      string        `json:"name"`
	ServerID  string        `json:"server_id"`
	Overrides []overrideDTO `json:"overrides"`
}

func (r createDeploymentRequest) overridesToDomain() []*domain.DeploymentOverride {
	out := make([]*domain.DeploymentOverride, 0, len(r.Overrides))
	for _, o := range r.Overrides {
		out = append(out, &domain.DeploymentOverride{
			Kind:      domain.OverrideKind(o.Kind),
			TargetRef: o.TargetRef,
			Key:       o.Key,
			Value:     o.Value,
		})
	}
	return out
}

// ---- Image DTOs ------------------------------------------------------------

type pullImageRequest struct {
	Ref     string `json:"ref"`
	AuthB64 string `json:"auth"`
}

type buildImageRequest struct {
	ContextDir string `json:"context_dir"`
	Dockerfile string `json:"dockerfile"`
	Tag        string `json:"tag"`
}

// ---- Status response -------------------------------------------------------

type statusResponse struct {
	Status     domain.DeploymentStatus `json:"status"`
	Containers []*domain.Container     `json:"containers"`
}

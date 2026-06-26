package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Nolanndev/dev-natif/internal/domain"
)

// DeploymentService orchestrates the materialisation of a project on a Docker
// Engine: resolving deployment-specific overrides, ordering services by their
// dependencies, creating volumes/containers and supervising their state.
type DeploymentService struct {
	deployments domain.DeploymentRepository
	projects    domain.ProjectRepository
	servers     domain.ServerRepository
	engine      domain.DockerEngine
	events      domain.EventRepository
}

// NewDeploymentService wires the deployment use cases.
func NewDeploymentService(
	d domain.DeploymentRepository,
	p domain.ProjectRepository,
	srv domain.ServerRepository,
	eng domain.DockerEngine,
	events domain.EventRepository,
) *DeploymentService {
	return &DeploymentService{deployments: d, projects: p, servers: srv, engine: eng, events: events}
}

// record persists an event (best effort; never fails the caller).
func (s *DeploymentService) record(ctx context.Context, level, typ string, dep *domain.Deployment, message string) {
	if s.events == nil {
		return
	}
	e := &domain.Event{Level: level, Type: typ, Message: message}
	if dep != nil {
		e.ProjectID = dep.ProjectID
		e.DeploymentID = dep.ID
	}
	_ = s.events.RecordEvent(ctx, e)
}

// ListProjectDeployments returns a project's deployment history (newest first).
func (s *DeploymentService) ListProjectDeployments(ctx context.Context, projectID string) ([]*domain.Deployment, error) {
	return s.deployments.ListDeploymentsByProject(ctx, projectID)
}

// ListProjectEvents returns recent events for a project.
func (s *DeploymentService) ListProjectEvents(ctx context.Context, projectID string, limit int) ([]*domain.Event, error) {
	return s.events.ListEvents(ctx, domain.EventFilter{ProjectID: projectID, Limit: limit})
}

// ListDeploymentEvents returns recent events for one deployment.
func (s *DeploymentService) ListDeploymentEvents(ctx context.Context, deploymentID string, limit int) ([]*domain.Event, error) {
	return s.events.ListEvents(ctx, domain.EventFilter{DeploymentID: deploymentID, Limit: limit})
}

// ListRecentEvents returns the most recent events across the system.
func (s *DeploymentService) ListRecentEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	return s.events.ListEvents(ctx, domain.EventFilter{Limit: limit})
}

// ContainerLogs returns the last `tail` lines of a container's logs.
func (s *DeploymentService) ContainerLogs(ctx context.Context, dockerID string, tail int) (string, error) {
	return s.engine.ContainerLogs(ctx, dockerID, tail)
}

// CreateDeployment registers a new (not yet instantiated) deployment for a
// project, with its environment-specific overrides.
func (s *DeploymentService) CreateDeployment(ctx context.Context, projectID, name, serverID string, overrides []*domain.DeploymentOverride) (*domain.Deployment, error) {
	if _, err := s.projects.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, validation("deployment name is required")
	}
	if serverID == "" {
		def, err := s.servers.DefaultServer(ctx)
		if err != nil {
			return nil, err
		}
		serverID = def.ID
	} else if _, err := s.servers.GetServer(ctx, serverID); err != nil {
		return nil, err
	}

	d := &domain.Deployment{
		ProjectID: projectID,
		ServerID:  serverID,
		Name:      name,
		Status:    domain.StatusPending,
		Overrides: overrides,
	}
	if err := s.deployments.CreateDeployment(ctx, d); err != nil {
		return nil, err
	}
	s.record(ctx, domain.LevelInfo, domain.EvtDeploymentCreated, d, "deployment created: "+name)
	return s.deployments.GetDeployment(ctx, d.ID)
}

func (s *DeploymentService) GetDeployment(ctx context.Context, id string) (*domain.Deployment, error) {
	return s.deployments.GetDeployment(ctx, id)
}

func (s *DeploymentService) ListDeployments(ctx context.Context) ([]*domain.Deployment, error) {
	return s.deployments.ListDeployments(ctx)
}

// DeleteDeployment tears down the running resources (best effort) then removes
// the deployment record.
func (s *DeploymentService) DeleteDeployment(ctx context.Context, id string) error {
	dep, _ := s.deployments.GetDeployment(ctx, id)
	_ = s.Down(ctx, id) // best effort; ignore if nothing is running
	if err := s.deployments.DeleteDeployment(ctx, id); err != nil {
		return err
	}
	if dep != nil {
		s.record(ctx, domain.LevelInfo, domain.EvtDeploymentDeleted, dep, "deployment deleted: "+dep.Name)
	}
	return nil
}

// Up instantiates the project on the engine: creates volumes, then creates and
// starts one container per service replica in dependency order.
func (s *DeploymentService) Up(ctx context.Context, id string) (*domain.Deployment, error) {
	dep, err := s.deployments.GetDeployment(ctx, id)
	if err != nil {
		return nil, err
	}
	services, err := s.projects.ListServices(ctx, dep.ProjectID)
	if err != nil {
		return nil, err
	}
	volumes, err := s.projects.ListVolumes(ctx, dep.ProjectID)
	if err != nil {
		return nil, err
	}

	ordered, err := topoSort(services)
	if err != nil {
		return nil, err
	}

	// Create the deployment-scoped network so all services and replicas can
	// resolve each other by service name (docker-compose default-network style).
	netName := networkName(dep)
	if nerr := s.engine.EnsureNetwork(ctx, netName, s.labels(dep)); nerr != nil {
		s.markFailed(ctx, dep, nerr)
		return nil, fmt.Errorf("create network: %w", nerr)
	}

	// Create the deployment-scoped volumes; map domain volume ID -> engine name.
	volNames := make(map[string]string, len(volumes))
	for _, v := range volumes {
		engineName := sanitizeName("devnatif", dep.ID, v.Name)
		labels := s.labels(dep)
		if cerr := s.engine.CreateVolume(ctx, engineName, labels); cerr != nil {
			s.markFailed(ctx, dep, cerr)
			return nil, fmt.Errorf("create volume %q: %w", v.Name, cerr)
		}
		volNames[v.ID] = engineName
	}

	overrideEnv, overridePort := indexOverrides(dep.Overrides)

	var tracked []*domain.Container
	for _, svc := range ordered {
		replicas := svc.Replicas
		if replicas <= 0 {
			replicas = 1
		}

		image, ierr := s.ensureImage(ctx, dep, svc)
		if ierr != nil {
			s.markFailed(ctx, dep, ierr)
			return nil, ierr
		}

		for i := 0; i < replicas; i++ {
			spec := domain.ContainerSpec{
				Name:          sanitizeName("devnatif", dep.Name, svc.Name, fmt.Sprintf("%d", i+1)),
				Image:         image,
				Env:           resolveEnv(svc, overrideEnv[svc.ID]),
				Cmd:           svc.Command,
				Labels:        s.labels(dep, svc),
				Ports:         resolvePorts(svc, overridePort[svc.ID]),
				Mounts:        resolveMounts(svc, volNames),
				RestartPolicy: svc.RestartPolicy,
				Network:       netName,
				// All replicas of a service share the service-name alias, so the
				// name round-robins across instances exactly like docker-compose.
				Aliases: []string{sanitizeName(svc.Name), svc.Name},
			}
			cid, cerr := s.engine.CreateContainer(ctx, spec)
			if cerr != nil {
				s.markFailed(ctx, dep, cerr)
				return nil, fmt.Errorf("create container for service %q: %w", svc.Name, cerr)
			}
			if serr := s.engine.StartContainer(ctx, cid); serr != nil {
				s.markFailed(ctx, dep, serr)
				return nil, fmt.Errorf("start container for service %q: %w", svc.Name, serr)
			}
			tracked = append(tracked, &domain.Container{
				DeploymentID:      dep.ID,
				ServiceID:         svc.ID,
				DockerContainerID: cid,
				Name:              spec.Name,
				State:             "running",
			})
		}
	}

	if err := s.deployments.SaveContainers(ctx, dep.ID, tracked); err != nil {
		return nil, err
	}
	status, _, _ := s.computeStatus(ctx, dep.ID)
	dep.Status = status
	dep.UpdatedAt = time.Now().UTC()
	if err := s.deployments.UpdateDeployment(ctx, dep); err != nil {
		return nil, err
	}
	s.record(ctx, domain.LevelInfo, domain.EvtDeploymentUp, dep,
		fmt.Sprintf("deployment up: %d container(s), status %s", len(tracked), status))
	return s.deployments.GetDeployment(ctx, dep.ID)
}

// Down stops and removes every container belonging to the deployment (found by
// label, so it is robust to tracking drift). Volumes are kept (persistent).
func (s *DeploymentService) Down(ctx context.Context, id string) error {
	dep, err := s.deployments.GetDeployment(ctx, id)
	if err != nil {
		return err
	}
	infos, lerr := s.engine.ListContainersByLabel(ctx, map[string]string{domain.LabelDeployment: dep.ID})
	if lerr != nil {
		s.record(ctx, domain.LevelError, domain.EvtDockerError, dep, lerr.Error())
		return lerr
	}
	for _, info := range infos {
		_ = s.engine.StopContainer(ctx, info.ID)
		if rerr := s.engine.RemoveContainer(ctx, info.ID, true); rerr != nil {
			s.record(ctx, domain.LevelError, domain.EvtDockerError, dep, rerr.Error())
			return fmt.Errorf("remove container %s: %w", info.ID, rerr)
		}
	}
	// Remove the deployment network (containers must be gone first).
	_ = s.engine.RemoveNetwork(ctx, networkName(dep))
	if err := s.deployments.SaveContainers(ctx, dep.ID, nil); err != nil {
		return err
	}
	dep.Status = domain.StatusNotRunning
	dep.UpdatedAt = time.Now().UTC()
	s.record(ctx, domain.LevelInfo, domain.EvtDeploymentDown, dep, "deployment stopped")
	return s.deployments.UpdateDeployment(ctx, dep)
}

// DownProject stops and removes every container the API created for a project,
// across all of its deployments (found by the project label). Used before a
// project is deleted so no orphan containers are left on the engine. Volumes are
// kept, consistent with Down semantics. Best effort: errors are returned but the
// loop tries every container first.
func (s *DeploymentService) DownProject(ctx context.Context, projectID string) error {
	infos, err := s.engine.ListContainersByLabel(ctx, map[string]string{domain.LabelProject: projectID})
	if err != nil {
		return err
	}
	var firstErr error
	for _, info := range infos {
		_ = s.engine.StopContainer(ctx, info.ID)
		if rerr := s.engine.RemoveContainer(ctx, info.ID, true); rerr != nil && firstErr == nil {
			firstErr = rerr
		}
	}
	// Remove the project's networks (across all its deployments).
	if nets, lerr := s.engine.ListNetworksByLabel(ctx, map[string]string{domain.LabelProject: projectID}); lerr == nil {
		for _, id := range nets {
			_ = s.engine.RemoveNetwork(ctx, id)
		}
	}
	return firstErr
}

// ListImages returns the images available on the engine.
func (s *DeploymentService) ListImages(ctx context.Context) ([]domain.ImageInfo, error) {
	return s.engine.ListImages(ctx)
}

// networkName is the deterministic per-deployment network name.
func networkName(dep *domain.Deployment) string {
	return sanitizeName("devnatif", "net", dep.ID)
}

// Status returns the live aggregated state plus the refreshed container list.
func (s *DeploymentService) Status(ctx context.Context, id string) (domain.DeploymentStatus, []*domain.Container, error) {
	status, containers, err := s.computeStatus(ctx, id)
	if err != nil {
		return "", nil, err
	}
	// Persist the refreshed view so GET deployment reflects reality.
	if dep, derr := s.deployments.GetDeployment(ctx, id); derr == nil {
		if dep.Status != status {
			dep.Status = status
			dep.UpdatedAt = time.Now().UTC()
			_ = s.deployments.UpdateDeployment(ctx, dep)
		}
	}
	return status, containers, nil
}

// ---- Images ----------------------------------------------------------------

// PullImage pulls an image on the engine.
func (s *DeploymentService) PullImage(ctx context.Context, ref, authB64 string) error {
	if strings.TrimSpace(ref) == "" {
		return validation("image ref is required")
	}
	if err := s.engine.PullImage(ctx, domain.ImagePullSpec{Ref: ref, AuthB64: authB64}); err != nil {
		s.record(ctx, domain.LevelError, domain.EvtDockerError, nil, "image pull failed ("+ref+"): "+err.Error())
		return err
	}
	s.record(ctx, domain.LevelInfo, domain.EvtImagePull, nil, "image pulled: "+ref)
	return nil
}

// BuildImage builds an image from a context directory.
func (s *DeploymentService) BuildImage(ctx context.Context, contextDir, dockerfile, tag string) error {
	if strings.TrimSpace(contextDir) == "" || strings.TrimSpace(tag) == "" {
		return validation("build context and tag are required")
	}
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if err := s.engine.BuildImage(ctx, domain.ImageBuildSpec{ContextDir: contextDir, Dockerfile: dockerfile, Tag: tag}); err != nil {
		s.record(ctx, domain.LevelError, domain.EvtDockerError, nil, "image build failed ("+tag+"): "+err.Error())
		return err
	}
	s.record(ctx, domain.LevelInfo, domain.EvtImageBuild, nil, "image built: "+tag)
	return nil
}

// ---- internal helpers ------------------------------------------------------

// ensureImage makes sure an image is available for a service and returns the
// reference to run. If the service declares an image, it is pulled; otherwise it
// is built from its build context and a derived tag is returned.
func (s *DeploymentService) ensureImage(ctx context.Context, dep *domain.Deployment, svc *domain.Service) (string, error) {
	if strings.TrimSpace(svc.Image) != "" {
		// Best effort pull: if it fails but the image exists locally, container
		// creation will still succeed, so we only surface the error from create.
		_ = s.engine.PullImage(ctx, domain.ImagePullSpec{Ref: svc.Image})
		return svc.Image, nil
	}
	tag := sanitizeName("devnatif", dep.ProjectID, svc.Name) + ":latest"
	dockerfile := svc.BuildDockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if err := s.engine.BuildImage(ctx, domain.ImageBuildSpec{
		ContextDir: svc.BuildContext,
		Dockerfile: dockerfile,
		Tag:        tag,
	}); err != nil {
		return "", fmt.Errorf("build image for service %q: %w", svc.Name, err)
	}
	return tag, nil
}

func (s *DeploymentService) computeStatus(ctx context.Context, id string) (domain.DeploymentStatus, []*domain.Container, error) {
	dep, err := s.deployments.GetDeployment(ctx, id)
	if err != nil {
		return "", nil, err
	}
	infos, err := s.engine.ListContainersByLabel(ctx, map[string]string{domain.LabelDeployment: dep.ID})
	if err != nil {
		return "", nil, err
	}
	if len(infos) == 0 {
		if dep.Status == domain.StatusPending {
			return domain.StatusPending, nil, nil
		}
		return domain.StatusNotRunning, nil, nil
	}

	running := 0
	containers := make([]*domain.Container, 0, len(infos))
	for _, info := range infos {
		if info.State == "running" {
			running++
		}
		containers = append(containers, &domain.Container{
			DeploymentID:      dep.ID,
			ServiceID:         info.Labels[domain.LabelService],
			DockerContainerID: info.ID,
			Name:              info.Name,
			State:             info.State,
			Health:            info.Health,
		})
	}
	_ = s.deployments.SaveContainers(ctx, dep.ID, containers)

	switch {
	case running == 0:
		return domain.StatusNotRunning, containers, nil
	case running == len(infos):
		return domain.StatusRunning, containers, nil
	default:
		return domain.StatusPartiallyRunning, containers, nil
	}
}

func (s *DeploymentService) markFailed(ctx context.Context, dep *domain.Deployment, cause error) {
	dep.Status = domain.StatusFailed
	dep.UpdatedAt = time.Now().UTC()
	_ = s.deployments.UpdateDeployment(ctx, dep)
	msg := "deployment failed"
	if cause != nil {
		msg = cause.Error()
	}
	// Recorded as a persistent error event so the Docker daemon failure is
	// reviewable in the UI, not just a transient flash message.
	s.record(ctx, domain.LevelError, domain.EvtDeploymentFailed, dep, msg)
}

// labels builds the management labels for a deployment (and optionally service).
func (s *DeploymentService) labels(dep *domain.Deployment, svc ...*domain.Service) map[string]string {
	l := map[string]string{
		domain.LabelManaged:    "true",
		domain.LabelProject:    dep.ProjectID,
		domain.LabelDeployment: dep.ID,
	}
	if len(svc) > 0 && svc[0] != nil {
		l[domain.LabelService] = svc[0].ID
	}
	return l
}

// indexOverrides groups overrides by service ID and key for fast lookup.
func indexOverrides(overrides []*domain.DeploymentOverride) (env map[string]map[string]string, port map[string]map[string]string) {
	env = map[string]map[string]string{}
	port = map[string]map[string]string{}
	for _, o := range overrides {
		switch o.Kind {
		case domain.OverrideEnv:
			if env[o.TargetRef] == nil {
				env[o.TargetRef] = map[string]string{}
			}
			env[o.TargetRef][o.Key] = o.Value
		case domain.OverridePort:
			if port[o.TargetRef] == nil {
				port[o.TargetRef] = map[string]string{}
			}
			port[o.TargetRef][o.Key] = o.Value
		}
	}
	return env, port
}

// resolveEnv merges service env with deployment overrides into KEY=VALUE pairs.
func resolveEnv(svc *domain.Service, overrides map[string]string) []string {
	out := make([]string, 0, len(svc.Env))
	for _, e := range svc.Env {
		val := e.Value
		if ov, ok := overrides[e.Key]; ok {
			val = ov
		}
		out = append(out, e.Key+"="+val)
	}
	return out
}

// resolvePorts applies host-port overrides to the service's declared ports.
func resolvePorts(svc *domain.Service, overrides map[string]string) []domain.PortBinding {
	out := make([]domain.PortBinding, 0, len(svc.Ports))
	for _, p := range svc.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		host := p.HostPort
		if ov, ok := overrides[portKey(p.ContainerPort, proto)]; ok {
			if n, err := atoiSafe(ov); err == nil {
				host = n
			}
		}
		out = append(out, domain.PortBinding{ContainerPort: p.ContainerPort, HostPort: host, Protocol: proto})
	}
	return out
}

// resolveMounts maps a service's volume bindings to engine mounts.
func resolveMounts(svc *domain.Service, volNames map[string]string) []domain.Mount {
	out := make([]domain.Mount, 0, len(svc.Mounts))
	for _, m := range svc.Mounts {
		name, ok := volNames[m.VolumeID]
		if !ok {
			continue // volume not part of project; skip defensively
		}
		out = append(out, domain.Mount{VolumeName: name, Target: m.Target, ReadOnly: m.ReadOnly})
	}
	return out
}

func atoiSafe(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}
